package server

import (
	"cmp"
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
)

type Problem string

const (
	ProblemNoRoute        Problem = "no route"
	ProblemUnknownState   Problem = "unknown state"
	ProblemClientLeft     Problem = "client left before the response"
	ProblemBackend        Problem = "backend unreachable"
	ProblemInvalidRequest Problem = "invalid request"
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
}

const clientClosedRequest = 499

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
	var px *proxy
	if opts.Proxy != nil {
		px = newProxy(opts.Proxy, opts)
	}
	set, _ := schema.New(p.Schemas)
	mux := http.NewServeMux()
	mux.Handle("/", noRoute(opts.Report, p.Routes, px))

	owners := make(map[string]string, len(p.Routes))
	for _, rt := range p.Routes {
		compiled, templatesReadBody, err := compileStates(rt)
		if err != nil {
			return nil, err
		}
		checks, err := compileRequest(set, p.Schemas, rt.Request)
		if err != nil {
			return nil, fmt.Errorf("route %s: %w", rt.ID, err)
		}
		h := serveRoute(rt, opts, px, compiled, templatesReadBody, checks)
		for _, pattern := range patterns(rt) {
			if err := handle(mux, pattern, h); err != nil {
				return nil, conflictError(rt, pattern, err, owners)
			}
			owners[pattern] = rt.ID
		}
	}
	if opts.DisableCORS {
		return mux, nil
	}
	return withCORS(mux), nil
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

func serveRoute(rt *model.Route, opts Options, px *proxy, compiled map[string]*templates, templatesReadBody bool, checks *requestCheck) http.HandlerFunc {
	report := opts.Report
	withBody := readsBody(rt) || templatesReadBody
	return func(w http.ResponseWriter, req *http.Request) {
		if px != nil && rt.Serve == model.ServeProxy && !asksForState(req) {
			px.serve(w, req, rt.ID)
			return
		}
		x := &exchange{rt: rt, opts: opts, req: req, call: opts.Memory.call(rt.ID), withBody: withBody}
		name, reason, rule := pickState(rt, req, opts.Seed, x.call, x.facts)
		rec := Request{Method: req.Method, Path: req.URL.Path, Route: rt.ID, State: name, Reason: reason, Rule: rule}

		st, ok := rt.States.Get(name)
		if !ok {
			rec.Status = http.StatusBadRequest
			rec.Problem = ProblemUnknownState
			names := rt.States.Names()
			writeJSON(w, rec.Status, unknownState{Error: "unknown_state", Route: rt.ID, State: name, Valid: names, Hint: hint(name, names)})
			report(rec)
			return
		}
		if checks != nil && !opts.NoRequestValidation && !st.SkipRequestValidation {
			if problems := checks.check(req, x.body); len(problems) > 0 {
				rec.Status, rec.Problem = http.StatusBadRequest, ProblemInvalidRequest
				rec.Detail = problems[0].At + ": " + problems[0].Message
				writeJSON(w, rec.Status, invalidRequest{Error: "invalid_request", Route: rt.ID, Problems: problems})
				report(rec)
				return
			}
		}
		if opts.Clock.Sleep(req.Context(), delay(st, opts.Seed, rt.ID, x.call)) != nil {
			rec.Status, rec.Problem = clientClosedRequest, ProblemClientLeft
			report(rec)
			return
		}
		x.respond(w, st, compiled[name], &rec)
		report(rec)
	}
}

type exchange struct {
	rt       *model.Route
	opts     Options
	req      *http.Request
	call     int
	withBody bool
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

func (x *exchange) respond(w http.ResponseWriter, st *model.State, t *templates, rec *Request) {
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

func pickState(rt *model.Route, req *http.Request, src seed.Source, call int, facts func() match.Facts) (string, Reason, int) {
	if name := req.Header.Get(stateHeader); name != "" {
		return name, ReasonHeader, 0
	}
	if name := req.URL.Query().Get(stateQuery); name != "" {
		return name, ReasonQuery, 0
	}
	switch rt.Mode {
	case model.ModeRules:
		if i, ok := match.First(rt.Rules, facts()); ok {
			return rt.Rules[i].State, ReasonRule, i + 1
		}
	case model.ModeSequential:
		return rt.States[min(call, len(rt.States))-1].Name, ReasonSequential, 0
	case model.ModeRandom:
		pick := src.Stream(rt.ID, strconv.Itoa(call)).IntN(len(rt.States))
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

func noRoute(report func(Request), routes []*model.Route, px *proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if px != nil {
			px.serve(w, req, "")
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
	if rt.Method != model.MethodCRUD {
		return []string{string(rt.Method) + " " + rt.Path}
	}
	one := rt.Path + "/{" + crudParam + "}"
	return []string{"GET " + rt.Path, "POST " + rt.Path, "GET " + one, "PUT " + one, "PATCH " + one, "DELETE " + one}
}
