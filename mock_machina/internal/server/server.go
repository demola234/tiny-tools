package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/match"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/schema"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

const (
	stateHeader = "X-Mock-State"
	routeHeader = "X-Mock-Route"
	stateQuery  = "__state"
)

type Reason string

const (
	ReasonHeader     Reason = "header"
	ReasonQuery      Reason = "query"
	ReasonActive     Reason = "active"
	ReasonRule       Reason = "rule"
	ReasonSequential Reason = "sequential"
	ReasonRandom     Reason = "random"
	ReasonControl    Reason = "control"
	ReasonScenario   Reason = "scenario"
)

type Problem string

const (
	ProblemNoRoute         Problem = "no route"
	ProblemUnknownState    Problem = "unknown state"
	ProblemClientLeft      Problem = "client left before the response"
	ProblemBackend         Problem = "backend unreachable"
	ProblemInvalidRequest  Problem = "invalid request"
	ProblemUnknownScenario Problem = "unknown scenario"
)

type Request struct {
	Method     string
	Path       string
	Status     int
	Route      string
	State      string
	Reason     Reason
	Problem    Problem
	Suggestion string
	Proxied    bool
	Rule       int
	Fault      model.FaultType
	Detail     string
	Webhook    bool
	GRPCCode   string
	Duration   time.Duration
}

type Options struct {
	Report        func(Request)
	Clock         clock.Clock
	DisableCORS   bool
	NoMockHeaders bool
	Proxy         *url.URL
	Memory        *Memory
	Seed          seed.Source
	Locale        string

	NoRequestValidation bool
	WebhookRetryWait    time.Duration
	Dashboard           string

	scenarios scenarios
}

const clientClosedRequest = 499

func (o Options) reporter() func(Request) {
	start := o.Clock.Now()
	return func(r Request) {
		r.Duration = o.Clock.Now().Sub(start)
		if r.Route != "" && r.State != "" && !r.Webhook && r.Problem == "" {
			o.Memory.serve(r.Route, r.State)
		}
		o.Report(r)
	}
}

func New(p *model.Project, opts Options) (http.Handler, error) {
	if opts.Report == nil {
		opts.Report = func(Request) {}
	}
	if opts.Clock == nil {
		opts.Clock = clock.Real{}
	}
	if opts.Memory == nil {
		opts.Memory = NewMemory()
	}
	opts.scenarios = indexScenarios(p.Scenarios)
	var px *proxy
	if opts.Proxy != nil {
		px = newProxy(opts.Proxy, opts)
	}
	set, _ := schema.New(p.Schemas)
	mux := http.NewServeMux()
	mux.Handle("/", noRoute(opts.reporter, p.Routes, px, opts.Dashboard))

	if err := route(mux, p, opts, px, set); err != nil {
		return nil, err
	}
	h := checkScenario(mux, opts)
	if opts.DisableCORS {
		return h, nil
	}
	return withCORS(h), nil
}

func handle(mux *http.ServeMux, pattern string, h http.Handler) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("%v", v)
		}
	}()
	mux.Handle(pattern, h)
	return nil
}

func conflictError(rt *model.Route, pattern string, cause error, owners map[string]string) error {
	for other, id := range owners {
		if strings.Contains(cause.Error(), strconv.Quote(other)) {
			return fmt.Errorf("routes %s (%s) and %s (%s) overlap: a request could match both", id, other, rt.ID, pattern)
		}
	}
	return fmt.Errorf("route %s (%s): %w", rt.ID, pattern, cause)
}

func compileRoute(rt *model.Route, set *schema.Set, schemas []*model.Schema) (routeTemplates, *requestCheck, error) {
	states, readsBody, err := compileStates(rt)
	if err != nil {
		return routeTemplates{}, nil, err
	}
	checks, err := compileRequest(set, schemas, rt.Request)
	if err != nil {
		return routeTemplates{}, nil, fmt.Errorf("route %s: %w", rt.ID, err)
	}
	streams, err := compileStreams(rt)
	if err != nil {
		return routeTemplates{}, nil, err
	}
	hooks, err := compileHooks(rt)
	if err != nil {
		return routeTemplates{}, nil, err
	}
	return routeTemplates{states: states, readsBody: readsBody, streams: streams, hooks: hooks}, checks, nil
}

type routeTemplates struct {
	states    map[string]*templates
	readsBody bool
	streams   map[string]*stream
	hooks     map[string][]hook
}

func serveRoute(rt *model.Route, opts Options, px *proxy, compiled routeTemplates, checks *requestCheck) http.HandlerFunc {
	withBody := readsBody(rt) || compiled.readsBody
	return func(w http.ResponseWriter, req *http.Request) {
		report := opts.reporter()
		_, inScenario := opts.scenarios.state(req, rt.ID)
		if px != nil && rt.Serve == model.ServeProxy && !asksForState(req) && !inScenario {
			px.serve(w, req, rt.ID)
			return
		}
		x := &exchange{rt: rt, opts: opts, req: req, call: opts.Memory.call(rt.ID), withBody: withBody, report: report}
		name, reason, rule := pickState(rt, req, opts, x.call, x.facts)
		rec := Request{Method: req.Method, Path: req.URL.Path, Route: rt.ID, State: name, Reason: reason, Rule: rule}
		if rt.Method == model.MethodGraphQL {
			rec.Method, rec.Path = string(model.MethodGraphQL), rt.Operation
		}

		st, ok := rt.States.Get(name)
		if !ok {
			rec.Status = http.StatusBadRequest
			rec.Problem = ProblemUnknownState
			names := rt.States.Names()
			writeJSON(w, rec.Status, unknownState{Error: "unknown_state", Route: rt.ID, State: name, Valid: names, Hint: hint(name, names)})
			report(rec)
			return
		}
		if x.rejected(w, checks, st, &rec) {
			report(rec)
			return
		}
		if opts.Clock.Sleep(req.Context(), delay(st, opts.Seed, rt.ID, x.call)) != nil {
			rec.Status, rec.Problem = clientClosedRequest, ProblemClientLeft
			report(rec)
			return
		}
		if opensStream(rt, st) {
			x.open(req.Context(), w, st, compiled.states[name], compiled.streams[name], &rec)
			return
		}
		x.state, x.hooks = name, compiled.hooks[name]
		x.respond(req.Context(), w, st, compiled.states[name], &rec)
		report(rec)
	}
}

type exchange struct {
	rt       *model.Route
	opts     Options
	req      *http.Request
	report   func(Request)
	call     int
	withBody bool
	state    string
	hooks    []hook
	cached   *match.Facts
	raw      []byte
	read     bool
}

func (x *exchange) body() []byte {
	if !x.read {
		x.raw, _ = io.ReadAll(io.LimitReader(x.req.Body, maxBody))
		x.read = true
	}
	return x.raw
}

func (x *exchange) facts() match.Facts {
	if x.cached == nil {
		var body []byte
		if x.withBody {
			body = x.body()
		}
		f := requestFacts(x.rt, x.req, x.call, body)
		f.Vars = x.opts.Memory.variables()
		x.cached = &f
	}
	return *x.cached
}

func (x *exchange) env() tmpl.Env {
	return tmpl.Env{
		Facts: x.facts(), Rand: x.opts.Seed.Stream(x.rt.ID, "template", strconv.Itoa(x.call)),
		Now: x.opts.Clock.Now(), Locale: x.opts.Locale,
	}
}

func (x *exchange) respond(ctx context.Context, w http.ResponseWriter, st *model.State, t *templates, rec *Request) {
	rec.Status = st.EffectiveStatus()
	h := w.Header()
	setStateHeaders(h, st)
	if !x.opts.NoMockHeaders {
		h.Set(stateHeader, rec.State)
		h.Set(routeHeader, x.rt.ID)
	}
	if len(st.Set) > 0 {
		x.assign(st.Set, t)
	}
	x.sendWebhooks(ctx, x.hooks)
	if x.crud(st) {
		r := x.serveCRUD()
		rec.Status = r.status
		if r.body != nil {
			h.Set("Content-Type", "application/json")
		}
		if r.location != "" {
			h.Set("Location", r.location)
		}
		w.WriteHeader(r.status)
		_, _ = w.Write(r.body)
		return
	}
	body := st.Body.Data
	if t != nil {
		body = t.apply(h, body, x.env())
	}
	if fires(st.Fault, x.opts.Seed, x.rt.ID, x.call) {
		rec.Fault = st.Fault.Type
		injectFault(w, x.req, st.Fault, rec.Status, body, x.opts.Clock)
		return
	}
	w.WriteHeader(rec.Status)
	_, _ = w.Write(body)
}

func (x *exchange) assign(set []model.Assignment, t *templates) {
	env := x.env()
	for _, a := range set {
		value := a.Value
		if tx, ok := t.setTemplate(a.Name); ok {
			value = tx.Eval(env)
		}
		x.opts.Memory.assign(a.Name, value)
	}
	if x.cached != nil {
		x.cached.Vars = x.opts.Memory.variables()
	}
}

func pickState(rt *model.Route, req *http.Request, opts Options, call int, facts func() match.Facts) (string, Reason, int) {
	if name := req.Header.Get(stateHeader); name != "" {
		return name, ReasonHeader, 0
	}
	if name := req.URL.Query().Get(stateQuery); name != "" {
		return name, ReasonQuery, 0
	}
	if name, ok := opts.scenarios.state(req, rt.ID); ok {
		return name, ReasonScenario, 0
	}
	if name, ok := opts.Memory.Pinned(rt.ID); ok {
		return name, ReasonControl, 0
	}
	switch rt.Mode {
	case model.ModeRules:
		if i, ok := match.First(rt.Rules, facts()); ok {
			return rt.Rules[i].State, ReasonRule, i + 1
		}
	case model.ModeSequential:
		return rt.States[min(call, len(rt.States))-1].Name, ReasonSequential, 0
	case model.ModeRandom:
		pick := opts.Seed.Stream(rt.ID, strconv.Itoa(call)).IntN(len(rt.States))
		return rt.States[pick].Name, ReasonRandom, 0
	case model.ModeActive:
	}
	return rt.Active, ReasonActive, 0
}

func setStateHeaders(h http.Header, st *model.State) {
	if st.Body.ContentType != "" {
		h.Set("Content-Type", st.Body.ContentType)
	}
	for k, v := range st.Headers {
		h.Set(k, v)
	}
}

func noRoute(reporter func() func(Request), routes []*model.Route, px *proxy, dashboard string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		report := reporter()
		if px != nil {
			px.serve(w, req, "")
			return
		}
		if dashboard != "" && opensPage(req) && req.URL.Path == "/" {
			http.Redirect(w, req, dashboard, http.StatusFound)
			return
		}
		closest := closestRoutes(req.Method, req.URL.Path, routes)
		writeJSON(w, http.StatusNotFound, missingRoute{Error: "no_route", Method: req.Method, Path: req.URL.Path, Closest: closest})
		rec := Request{Method: req.Method, Path: req.URL.Path, Status: http.StatusNotFound, Problem: ProblemNoRoute}
		if len(closest) > 0 {
			rec.Suggestion = closest[0]
		}
		report(rec)
	}
}

func opensPage(req *http.Request) bool {
	return req.Method == http.MethodGet && req.Header.Get("Sec-Fetch-Mode") == "navigate" && req.Header.Get("Sec-Fetch-Dest") == "document"
}

const maxClosest = 3

func closestRoutes(method, path string, routes []*model.Route) []string {
	type candidate struct {
		methodRank int
		distance   int
		key        string
	}
	candidates := make([]candidate, len(routes))
	for i, r := range routes {
		c := candidate{distance: suggest.Distance(path, r.Path), key: string(r.Method) + " " + r.Path}
		if string(r.Method) != method {
			c.methodRank = 1
		}
		candidates[i] = c
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Or(cmp.Compare(a.methodRank, b.methodRank), cmp.Compare(a.distance, b.distance), strings.Compare(a.key, b.key))
	})
	closest := make([]string, 0, maxClosest)
	for _, c := range candidates[:min(maxClosest, len(candidates))] {
		closest = append(closest, c.key)
	}
	return closest
}

func hint(name string, names []string) string {
	if s, ok := suggest.Closest(name, names); ok {
		return `did you mean "` + s + `"?`
	}
	return ""
}

type unknownState struct {
	Error string   `json:"error"`
	Route string   `json:"route"`
	State string   `json:"state"`
	Valid []string `json:"valid"`
	Hint  string   `json:"hint,omitempty"`
}

type missingRoute struct {
	Error   string   `json:"error"`
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	Closest []string `json:"closest,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func delay(st *model.State, src seed.Source, route string, call int) time.Duration {
	if st.Jitter <= 0 {
		return st.Latency
	}
	j := int64(st.Jitter)
	offset := src.Stream(route, "latency", strconv.Itoa(call)).Int64N(2*j+1) - j
	return max(0, st.Latency+time.Duration(offset))
}

func patterns(rt *model.Route) []string {
	if rt.Method.Stream() {
		return []string{"GET " + rt.Path}
	}
	if rt.Method != model.MethodCRUD {
		return []string{string(rt.Method) + " " + rt.Path}
	}
	one := rt.Path + "/{" + crudParam + "}"
	return []string{"GET " + rt.Path, "POST " + rt.Path, "GET " + one, "PUT " + one, "PATCH " + one, "DELETE " + one}
}

func (x *exchange) rejected(w http.ResponseWriter, checks *requestCheck, st *model.State, rec *Request) bool {
	if checks == nil || x.opts.NoRequestValidation || st.SkipRequestValidation {
		return false
	}
	problems := checks.check(x.req, x.body)
	if len(problems) == 0 {
		return false
	}
	rec.Status, rec.Problem = http.StatusBadRequest, ProblemInvalidRequest
	rec.Detail = problems[0].At + ": " + problems[0].Message
	writeJSON(w, rec.Status, invalidRequest{Error: "invalid_request", Route: x.rt.ID, Problems: problems})
	return true
}

func route(mux *http.ServeMux, p *model.Project, opts Options, px *proxy, set *schema.Set) error {
	owners := make(map[string]string, len(p.Routes))
	graphqlHandlers := map[*model.Route]http.Handler{}
	for _, rt := range p.Routes {
		compiled, checks, err := compileRoute(rt, set, p.Schemas)
		if err != nil {
			return err
		}
		h := serveRoute(rt, opts, px, compiled, checks)
		if rt.Method == model.MethodGraphQL {
			graphqlHandlers[rt] = h
			continue
		}
		if err := register(mux, rt, patterns(rt), h, owners); err != nil {
			return err
		}
	}
	for _, e := range graphqlEndpoints(p.Routes, graphqlHandlers, opts.reporter) {
		if err := register(mux, &model.Route{ID: "GRAPHQL " + e.path}, []string{"GET " + e.path, "POST " + e.path}, e, owners); err != nil {
			return err
		}
	}
	return nil
}

func register(mux *http.ServeMux, rt *model.Route, patterns []string, h http.Handler, owners map[string]string) error {
	for _, pattern := range patterns {
		if err := handle(mux, pattern, h); err != nil {
			return conflictError(rt, pattern, err, owners)
		}
		owners[pattern] = rt.ID
	}
	return nil
}
