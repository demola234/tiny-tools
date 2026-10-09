package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

const maxBody = 32 << 10

type listRoutesIn struct {
	Resource string `json:"resource,omitempty" jsonschema:"only routes of this resource, such as users for routes/users.yaml"`
}

type listRoutesOut struct {
	Routes []routeSummary `json:"routes"`
	Errors int            `json:"errors,omitempty" jsonschema:"lint errors in the project; routes in broken files may be missing, so call lint"`
}

type routeSummary struct {
	ID        string         `json:"id"`
	Method    string         `json:"method"`
	Path      string         `json:"path"`
	Summary   string         `json:"summary,omitempty"`
	Status    string         `json:"status" jsonschema:"lifecycle: draft, agreed, implemented or deprecated"`
	Active    string         `json:"active" jsonschema:"the state served when a request doesn't ask for one"`
	Generated bool           `json:"generated,omitempty" jsonschema:"written by an AI assistant and not yet reviewed by a person"`
	States    []stateSummary `json:"states"`
	File      string         `json:"file"`
	Line      int            `json:"line"`
}

type stateSummary struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type getRouteIn struct {
	ID string `json:"id" jsonschema:"route id, such as users.get"`
}

type routeDetail struct {
	ID        string            `json:"id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Summary   string            `json:"summary,omitempty"`
	Status    string            `json:"status" jsonschema:"lifecycle: draft, agreed, implemented or deprecated"`
	Owners    *owners           `json:"owners,omitempty"`
	Examples  map[string]string `json:"examples,omitempty" jsonschema:"path parameter values used to call a live API"`
	Active    string            `json:"active" jsonschema:"the state served when a request doesn't ask for one"`
	Generated bool              `json:"generated,omitempty" jsonschema:"written by an AI assistant and not yet reviewed by a person"`
	Mode      string            `json:"mode" jsonschema:"how the state is picked: active, rules (first matching rule), sequential or random"`
	Rules     []ruleOut         `json:"rules,omitempty" jsonschema:"checked in order; the first whose conditions all match picks the state"`
	CRUD      *crudOut          `json:"crud,omitempty" jsonschema:"for CRUD routes: the in-memory collection behind list, get, create, replace, update and delete"`
	Request   *requestOut       `json:"request,omitempty" jsonschema:"what the app must send; requests that don't match get a 400"`
	Responses []responseOut     `json:"responses,omitempty" jsonschema:"the schema of each response; lint checks every state's body against it"`
	States    []stateDetail     `json:"states"`
	File      string            `json:"file"`
	Line      int               `json:"line"`
}

type owners struct {
	Backend  []string `json:"backend,omitempty"`
	Frontend []string `json:"frontend,omitempty"`
}

type stateDetail struct {
	Name        string            `json:"name"`
	Status      int               `json:"status"`
	Headers     map[string]string `json:"headers,omitempty"`
	Latency     string            `json:"latency,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	BodyFile    string            `json:"bodyFile,omitempty" jsonschema:"file the body is read from, relative to the project folder"`
	Body        any               `json:"body,omitempty"`
	BodyOmitted string            `json:"bodyOmitted,omitempty" jsonschema:"why the body isn't included"`
	Jitter      string            `json:"jitter,omitempty" jsonschema:"latency varies by up to this either way"`
	Fault       *faultOut         `json:"fault,omitempty" jsonschema:"how this state fails like a network does"`
	Set         map[string]any    `json:"set,omitempty" jsonschema:"variables stored when this state is served"`
	Generated   bool              `json:"generated,omitempty" jsonschema:"written by an AI assistant and not yet reviewed by a person"`
	Line        int               `json:"line"`
}

type ruleOut struct {
	State string         `json:"state"`
	When  []conditionOut `json:"when"`
}

type conditionOut struct {
	Selector string `json:"selector"`
	Op       string `json:"op" jsonschema:"eq, ne, in, matches, exists, gt, gte, lt or lte"`
	Value    any    `json:"value"`
	Values   []any  `json:"values,omitempty"`
}

type requestOut struct {
	Params  []paramOut `json:"params,omitempty"`
	Query   []paramOut `json:"query,omitempty"`
	Headers []paramOut `json:"headers,omitempty"`
	Body    any        `json:"body,omitempty" jsonschema:"a schema name from schemas/, or the schema itself"`
}

type paramOut struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Schema   any    `json:"schema" jsonschema:"a schema name from schemas/, or the schema itself"`
}

type responseOut struct {
	Status string `json:"status" jsonschema:"an HTTP status, or default"`
	Schema any    `json:"schema" jsonschema:"a schema name from schemas/, or the schema itself"`
}

type crudOut struct {
	Collection string `json:"collection"`
	IDField    string `json:"idField"`
	Items      int    `json:"items" jsonschema:"items in data/<collection>.json at start"`
}

type faultOut struct {
	Type  string  `json:"type" jsonschema:"timeout, reset or truncated"`
	Rate  float64 `json:"rate"`
	After string  `json:"after,omitempty"`
}

type lintOut struct {
	Errors   int       `json:"errors"`
	Warnings int       `json:"warnings"`
	Problems []problem `json:"problems"`
}

type problem struct {
	Severity string `json:"severity" jsonschema:"error or warning"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

func (t tools) listRoutes(_ context.Context, _ *sdk.CallToolRequest, in listRoutesIn) (*sdk.CallToolResult, listRoutesOut, error) {
	p, probs, err := t.load()
	if err != nil {
		return nil, listRoutesOut{}, err
	}
	routes := p.Routes
	if in.Resource != "" {
		if routes, err = ofResource(routes, in.Resource); err != nil {
			return nil, listRoutesOut{}, err
		}
	}
	out := listRoutesOut{Routes: make([]routeSummary, len(routes)), Errors: countErrors(probs)}
	for i, r := range routes {
		out.Routes[i] = summarize(r)
	}
	return nil, out, nil
}

func ofResource(routes []*model.Route, name string) ([]*model.Route, error) {
	var names []string
	var matched []*model.Route
	for _, r := range routes {
		res := resourceOf(r)
		if res == name {
			matched = append(matched, r)
		}
		if !slices.Contains(names, res) {
			names = append(names, res)
		}
	}
	if len(matched) > 0 {
		return matched, nil
	}
	slices.Sort(names)
	if s, ok := suggest.Closest(name, names); ok {
		return nil, fmt.Errorf("no resource %q (did you mean %q?)", name, s)
	}
	return nil, fmt.Errorf("no resource %q (resources: %s)", name, strings.Join(names, ", "))
}

func resourceOf(r *model.Route) string {
	res, _, _ := strings.Cut(r.ID, ".")
	return res
}

func countErrors(probs config.Problems) int {
	n := 0
	for _, p := range probs {
		if p.Severity == config.Error {
			n++
		}
	}
	return n
}

func summarize(r *model.Route) routeSummary {
	s := routeSummary{
		ID: r.ID, Method: string(r.Method), Path: r.Path, Summary: r.Summary,
		Status: string(r.Status.Effective()), Active: active(r), Generated: r.Generated,
		States: make([]stateSummary, len(r.States)),
		File:   r.Src.File, Line: r.Src.Line,
	}
	for i, ns := range r.States {
		s.States[i] = stateSummary{Name: ns.Name, Status: ns.State.EffectiveStatus()}
	}
	return s
}

func active(r *model.Route) string {
	if r.Active != "" || len(r.States) == 0 {
		return r.Active
	}
	return r.States[0].Name
}

func (t tools) getRoute(_ context.Context, _ *sdk.CallToolRequest, in getRouteIn) (*sdk.CallToolResult, routeDetail, error) {
	p, _, err := t.load()
	if err != nil {
		return nil, routeDetail{}, err
	}
	r, err := config.FindRoute(p, in.ID)
	if err != nil {
		return nil, routeDetail{}, err
	}
	d := routeDetail{
		ID: r.ID, Method: string(r.Method), Path: r.Path, Summary: r.Summary,
		Status: string(r.Status.Effective()), Examples: r.Examples, Active: active(r), Generated: r.Generated,
		States: make([]stateDetail, len(r.States)),
		File:   r.Src.File, Line: r.Src.Line,
	}
	if !r.Owners.IsZero() {
		d.Owners = &owners{Backend: r.Owners.Backend, Frontend: r.Owners.Frontend}
	}
	d.Mode = string(r.Mode)
	if d.Mode == "" {
		d.Mode = string(model.ModeActive)
	}
	d.Rules = rulesOut(r.Rules)
	d.Request = requestDetail(r.Request)
	for _, resp := range r.Responses {
		d.Responses = append(d.Responses, responseOut{Status: resp.Status, Schema: schemaValue(resp.Schema)})
	}
	if c := r.CRUD; c != nil {
		var items []json.RawMessage
		_ = json.Unmarshal(c.Data, &items)
		d.CRUD = &crudOut{Collection: c.Collection, IDField: c.IDField, Items: len(items)}
	}
	for i, ns := range r.States {
		d.States[i] = detail(r, ns)
	}
	return nil, d, nil
}

func detail(r *model.Route, ns model.NamedState) stateDetail {
	st := ns.State
	d := stateDetail{
		Name: ns.Name, Status: st.EffectiveStatus(), Headers: st.Headers,
		ContentType: st.Body.ContentType, Line: st.Src.Line, Generated: st.Generated,
	}
	if st.Latency > 0 {
		d.Latency = st.Latency.String()
	}
	if st.Jitter > 0 {
		d.Jitter = st.Jitter.String()
	}
	if f := st.Fault; f != nil {
		d.Fault = &faultOut{Type: string(f.Type), Rate: f.Rate}
		if f.After > 0 {
			d.Fault.After = f.After.String()
		}
	}
	for _, a := range st.Set {
		if d.Set == nil {
			d.Set = map[string]any{}
		}
		d.Set[a.Name] = a.Value
	}
	if st.Body.File != "" {
		d.BodyFile = path.Join(r.Dir, st.Body.File)
	}
	d.Body, d.BodyOmitted = body(st.Body.Data, st.Body.ContentType, d.BodyFile)
	return d
}

func body(data []byte, contentType, file string) (any, string) {
	switch {
	case len(data) == 0:
		return nil, ""
	case len(data) > maxBody:
		return nil, fmt.Sprintf("the body is %d KB; read %s for it", len(data)>>10, file)
	case strings.HasPrefix(contentType, "application/json") && json.Valid(data):
		return json.RawMessage(data), ""
	case utf8.Valid(data):
		return string(data), ""
	default:
		return nil, fmt.Sprintf("the body is binary; read %s for it", file)
	}
}

func (t tools) lint(_ context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, lintOut, error) {
	p, probs, err := t.load()
	if err != nil {
		return nil, lintOut{}, err
	}
	all := slices.Concat(probs, config.Warnings(p)).Sorted()
	out := lintOut{Problems: make([]problem, len(all))}
	for i, pr := range all {
		sev := "error"
		if pr.Severity == config.Warning {
			sev = "warning"
			out.Warnings++
		} else {
			out.Errors++
		}
		out.Problems[i] = problem{Severity: sev, File: pr.File, Line: pr.Line, Message: pr.Msg}
	}
	return nil, out, nil
}

func rulesOut(rules []model.Rule) []ruleOut {
	out := make([]ruleOut, len(rules))
	for i, r := range rules {
		out[i] = ruleOut{State: r.State}
		for _, c := range r.When {
			sel := string(c.Scope)
			if c.Key != "" {
				sel += "." + c.Key
			}
			out[i].When = append(out[i].When, conditionOut{Selector: sel, Op: string(c.Op), Value: c.Value, Values: c.Values})
		}
	}
	return out
}

func requestDetail(req *model.Request) *requestOut {
	if req == nil {
		return nil
	}
	params := func(list []model.Param) []paramOut {
		out := make([]paramOut, 0, len(list))
		for _, p := range list {
			out = append(out, paramOut{Name: p.Name, Required: p.Required, Schema: schemaValue(p.Schema)})
		}
		return out
	}
	out := &requestOut{Params: params(req.Params), Query: params(req.Query), Headers: params(req.Headers)}
	if req.Body != nil {
		out.Body = schemaValue(*req.Body)
	}
	return out
}

func schemaValue(ref model.SchemaRef) any {
	if ref.Inline == nil {
		return ref.Name
	}
	return json.RawMessage(ref.Inline.JSON)
}
