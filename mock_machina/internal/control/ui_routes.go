package control

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const maxBodyShown = 4 << 10

var protocolTabs = []struct{ key, label string }{
	{"HTTP", "HTTP"}, {"CRUD", "CRUD"}, {"WS", "WebSocket"}, {"SSE", "SSE"}, {"GRAPHQL", "GraphQL"}, {"GRPC", "gRPC"},
}

type routeFilter struct {
	Q        string
	Protocol string
	Group    string
}

type protocolTab struct {
	Key     string
	Label   string
	Current bool
}

type routeRow struct {
	ID      string
	Method  string
	Path    string
	Serving string
	Note    string
	Current bool
}

type rowGroup struct {
	Name string
	Rows []routeRow
}

type stateChip struct {
	Name    string
	Code    string
	Current bool
}

type stateDetail struct {
	Name        string
	Status      string
	Latency     string
	Fault       string
	Headers     [][2]string
	ContentType string
	File        string
	Body        string
	BodyNote    string
}

type routePane struct {
	ID        string
	Method    string
	Path      string
	Operation string
	Summary   string
	Lifecycle string
	Mode      string
	Active    string
	Pinned    string
	Chips     []stateChip
	Shown     stateDetail
	Rules     []string
	Try       string
}

type routesPage struct {
	page
	Filter routeFilter
	Tabs   []protocolTab
	List   []rowGroup
	Pane   *routePane
	Error  string
}

func protocol(m model.Method) string {
	switch m {
	case model.MethodCRUD, model.MethodWS, model.MethodSSE, model.MethodGraphQL, model.MethodGRPC:
		return string(m)
	default:
		return "HTTP"
	}
}

func (f routeFilter) matches(r *model.Route) bool {
	if f.Group != "" && r.Group != f.Group {
		return false
	}
	if f.Protocol != "" && protocol(r.Method) != f.Protocol {
		return false
	}
	q := strings.ToLower(f.Q)
	return q == "" || strings.Contains(strings.ToLower(r.ID+" "+string(r.Method)+" "+r.Path+" "+r.Operation), q)
}

func (a *API) serving(r *model.Route) (state, note string) {
	if pinned, ok := a.opts.Memory.Pinned(r.ID); ok {
		return pinned, "pinned"
	}
	if r.Mode != "" && r.Mode != model.ModeActive {
		return string(r.Mode), "mode"
	}
	return r.Active, ""
}

func (a *API) routesPage(req *http.Request, current string) routesPage {
	p := a.project.Load()
	f := routeFilter{Q: req.FormValue("q"), Protocol: req.FormValue("protocol"), Group: req.FormValue("group")}
	rp := routesPage{page: a.page(req, "Routes"), Filter: f}
	present := map[string]bool{}
	for _, r := range p.Routes {
		present[protocol(r.Method)] = true
	}
	rp.Tabs = append(rp.Tabs, protocolTab{Label: "All", Current: f.Protocol == ""})
	for _, t := range protocolTabs {
		if present[t.key] {
			rp.Tabs = append(rp.Tabs, protocolTab{Key: t.key, Label: t.label, Current: f.Protocol == t.key})
		}
	}
	for _, r := range p.Routes {
		if !f.matches(r) {
			continue
		}
		state, note := a.serving(r)
		row := routeRow{ID: r.ID, Method: string(r.Method), Path: r.Path, Serving: state, Note: note, Current: r.ID == current}
		if r.Operation != "" {
			row.Path = r.Operation
		}
		if n := len(rp.List); n > 0 && rp.List[n-1].Name == r.Group {
			rp.List[n-1].Rows = append(rp.List[n-1].Rows, row)
			continue
		}
		rp.List = append(rp.List, rowGroup{Name: r.Group, Rows: []routeRow{row}})
	}
	return rp
}

func chipCode(r *model.Route, st *model.State) string {
	switch {
	case r.Method == model.MethodGRPC && st.GRPC.Code != "":
		return st.GRPC.Code
	case r.Method == model.MethodGRPC:
		return "OK"
	case st.Stream != nil:
		return "stream"
	case r.CRUD != nil && st.Status == 0 && st.Body.Data == nil:
		return "CRUD"
	case st.Status != 0:
		return strconv.Itoa(st.Status)
	default:
		return "200"
	}
}

func detail(r *model.Route, name string) stateDetail {
	st, ok := r.States.Get(name)
	if !ok {
		return stateDetail{}
	}
	d := stateDetail{Name: name, Status: chipCode(r, st), ContentType: st.Body.ContentType, File: st.Body.File}
	if st.Latency > 0 {
		d.Latency = st.Latency.String()
		if st.Jitter > 0 {
			d.Latency += " ± " + st.Jitter.String()
		}
	}
	if st.Fault != nil {
		d.Fault = string(st.Fault.Type)
	}
	for _, k := range slices.Sorted(maps.Keys(st.Headers)) {
		d.Headers = append(d.Headers, [2]string{k, st.Headers[k]})
	}
	switch body := st.Body.Data; {
	case st.Body.Generate:
		d.BodyNote = "generated from the response schema"
	case len(body) == 0:
	case !utf8.Valid(body):
		d.BodyNote = fmt.Sprintf("binary, %d bytes", len(body))
	case len(body) > maxBodyShown:
		d.Body = string(body[:maxBodyShown])
		d.BodyNote = fmt.Sprintf("first %d of %d bytes", maxBodyShown, len(body))
	default:
		d.Body = string(body)
	}
	return d
}

func describeRule(r model.Rule) string {
	parts := make([]string, len(r.When))
	for i, c := range r.When {
		value := fmt.Sprint(c.Value)
		if c.Values != nil {
			value = fmt.Sprint(c.Values)
		}
		subject := string(c.Scope)
		if c.Key != "" {
			subject += "." + c.Key
		}
		parts[i] = fmt.Sprintf("%s %s %s", subject, c.Op, value)
	}
	return "when " + strings.Join(parts, " and ") + " → " + r.State
}

func tryIt(req *http.Request, r *model.Route) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	url := scheme + "://" + req.Host + r.Path
	switch r.Method {
	case model.MethodGRPC:
		return ""
	case model.MethodWS:
		return "websocat ws" + strings.TrimPrefix(url, "http")
	case model.MethodSSE:
		return "curl -N " + url
	case model.MethodGraphQL:
		return fmt.Sprintf(`curl -X POST %s -H 'Content-Type: application/json' -d '{"query":"query %s { __typename }"}'`, url, r.Operation)
	case model.MethodGet, model.MethodCRUD:
		return "curl " + url
	default:
		return "curl -X " + string(r.Method) + " " + url
	}
}

func (a *API) pane(req *http.Request, r *model.Route) *routePane {
	pinned, _ := a.opts.Memory.Pinned(r.ID)
	served, _ := a.serving(r)
	p := &routePane{
		ID: r.ID, Method: string(r.Method), Path: r.Path, Operation: r.Operation, Summary: r.Summary,
		Lifecycle: string(r.Status), Mode: string(r.Mode), Active: r.Active, Pinned: pinned, Try: tryIt(req, r),
	}
	for _, s := range r.States {
		p.Chips = append(p.Chips, stateChip{Name: s.Name, Code: chipCode(r, s.State), Current: s.Name == served})
	}
	shown := pinned
	if shown == "" {
		shown = r.Active
	}
	p.Shown = detail(r, shown)
	for _, rule := range r.Rules {
		p.Rules = append(p.Rules, describeRule(rule))
	}
	return p
}

func failureText(f *failure) string {
	b := f.body
	msg := b.Detail
	switch b.Error {
	case "unknown_route":
		msg = fmt.Sprintf("no route %q", b.Route)
	case "unknown_state":
		msg = fmt.Sprintf("%s has no state %q", b.Route, b.State)
	case "unknown_collection":
		msg = fmt.Sprintf("no collection %q", b.Collection)
	case "unknown_item":
		msg = fmt.Sprintf("%s has no item %q", b.Collection, b.ID)
	case "unknown_scenario":
		msg = fmt.Sprintf("no scenario %q", b.Scenario)
	case "not_saved":
		msg = "couldn't save the routes file: " + b.Detail
	}
	if b.Hint != "" {
		msg += " (" + b.Hint + ")"
	}
	return msg
}

func (a *API) showRoute(w http.ResponseWriter, req *http.Request, f *failure) {
	id := req.PathValue("id")
	rp := a.routesPage(req, id)
	if f != nil {
		rp.Error = failureText(f)
	}
	r, missing := a.lookupRoute(id)
	if missing != nil {
		rp.Error = failureText(missing)
		render(w, "routes", http.StatusNotFound, rp)
		return
	}
	rp.Pane = a.pane(req, r)
	render(w, "routes", http.StatusOK, rp)
}

func (a *API) routesView(w http.ResponseWriter, req *http.Request) {
	render(w, "routes", http.StatusOK, a.routesPage(req, ""))
}

func (a *API) routeView(w http.ResponseWriter, req *http.Request) {
	a.showRoute(w, req, nil)
}

func (a *API) pinFromUI(w http.ResponseWriter, req *http.Request) {
	_, f := a.setState(req.PathValue("id"), req.FormValue("state"), false)
	a.showRoute(w, req, f)
}

func (a *API) defaultFromUI(w http.ResponseWriter, req *http.Request) {
	_, f := a.setState(req.PathValue("id"), req.FormValue("state"), true)
	a.showRoute(w, req, f)
}

func (a *API) unpinFromUI(w http.ResponseWriter, req *http.Request) {
	_, f := a.unpinRoute(req.PathValue("id"))
	a.showRoute(w, req, f)
}
