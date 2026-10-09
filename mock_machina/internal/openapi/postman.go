package openapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	postmanSchema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	routeMarker   = "mockmachina route "
	mockBaseURL   = "http://localhost:4001"
)

var (
	hostPrefix   = regexp.MustCompile(`^(\{\{[^}]+\}\}|https?://[^/]+)`)
	templateVar  = regexp.MustCompile(`^\{\{(.+)\}\}$`)
	jsonTextType = "application/json"
)

func isPostman(doc *omap) bool {
	return strings.Contains(doc.obj("info").str("schema"), "getpostman.com/json/collection")
}

type postmanRoute struct {
	route    *model.Route
	examples map[string][]any
	order    []string
}

type postman struct {
	res    *Result
	routes []*postmanRoute
	byKey  map[string]*postmanRoute
	ids    map[string]bool
}

func importPostman(doc *omap) *Result {
	p := &postman{
		res:   &Result{Title: doc.obj("info").str("name"), Version: "Postman Collection v2.1"},
		byKey: map[string]*postmanRoute{}, ids: map[string]bool{},
	}
	p.items(doc.get("item"))
	for _, pr := range p.routes {
		for _, code := range pr.order {
			schema := model.SchemaRef{Inline: &model.Schema{JSON: []byte("{}"), Src: pr.route.Src}}
			if values := pr.examples[code]; len(values) > 0 {
				inferred, _ := infer(values).(*omap)
				if inferred == nil {
					inferred = &omap{}
				}
				inferred.set("x-mockmachina-inferred", true)
				schema.Inline.JSON = encode(inferred)
			}
			pr.route.Responses = append(pr.route.Responses, model.Response{Status: code, Schema: schema})
		}
		pr.route.Active = firstSuccess(pr.route.States)
		p.res.Routes = append(p.res.Routes, pr.route)
	}
	return p.res
}

func (p *postman) items(v any) {
	for _, raw := range asList(v) {
		item, _ := raw.(*omap)
		if item.get("item") != nil {
			p.items(item.get("item"))
			continue
		}
		if req := item.obj("request"); req != nil || item.str("request") != "" {
			p.request(item)
		}
	}
}

func (p *postman) request(item *omap) {
	req := item.obj("request")
	method := strings.ToUpper(orDefault(req.str("method"), "GET"))
	path, query := postmanURL(req.get("url"))
	key := method + " " + path
	pr, ok := p.byKey[key]
	if !ok {
		pr = p.newRoute(item, method, path, query)
		p.byKey[key] = pr
		p.routes = append(p.routes, pr)
	}
	for _, raw := range asList(item.get("response")) {
		resp, _ := raw.(*omap)
		example(pr, resp)
	}
}

func (p *postman) newRoute(item *omap, method, path string, query []string) *postmanRoute {
	summary := item.str("name")
	resource, name := model.RouteNames(method, path)
	if id, rest, ok := strings.Cut(strings.TrimPrefix(item.str("description"), routeMarker), "\n"); strings.HasPrefix(item.str("description"), routeMarker) {
		if m := routeID.FindStringSubmatch(strings.TrimSpace(id)); m != nil {
			resource, name = m[1], m[2]
		}
		summary = ""
		if ok {
			summary = strings.TrimSpace(rest)
		}
	}
	id := resource + "." + name
	for n := 2; p.ids[id]; n++ {
		id = resource + "." + name + "-" + strconv.Itoa(n)
	}
	p.ids[id] = true
	r := &model.Route{
		ID: id, Group: resource, Method: model.Method(method), Path: path, Summary: summary,
		Src: model.Source{File: "routes/" + resource + ".yaml"},
	}
	if len(query) > 0 {
		r.Request = &model.Request{}
		for _, q := range query {
			r.Request.Query = append(r.Request.Query, model.Param{Name: q, Schema: model.SchemaRef{Inline: &model.Schema{JSON: []byte(`{"type":"string"}`), Src: r.Src}}})
		}
	}
	return &postmanRoute{route: r, examples: map[string][]any{}}
}

func postmanURL(v any) (string, []string) {
	raw, _ := v.(string)
	var query []string
	if u, ok := v.(*omap); ok {
		raw = u.str("raw")
		for _, q := range asList(u.get("query")) {
			if qo, _ := q.(*omap); qo != nil && qo.get("disabled") != true {
				query = append(query, qo.str("key"))
			}
		}
	}
	raw = hostPrefix.ReplaceAllString(raw, "")
	pathPart, rawQuery, _ := strings.Cut(raw, "?")
	if query == nil && rawQuery != "" {
		for pair := range strings.SplitSeq(rawQuery, "&") {
			k, _, _ := strings.Cut(pair, "=")
			query = append(query, k)
		}
	}
	segments := strings.Split(strings.Trim(pathPart, "/"), "/")
	for i, s := range segments {
		switch {
		case strings.HasPrefix(s, ":"):
			segments[i] = "{" + s[1:] + "}"
		case templateVar.MatchString(s):
			segments[i] = "{" + templateVar.FindStringSubmatch(s)[1] + "}"
		}
	}
	return "/" + strings.Join(segments, "/"), query
}

func example(pr *postmanRoute, resp *omap) {
	code := 200
	if n, ok := number(resp.get("code")); ok {
		code = int(n)
	}
	status := strconv.Itoa(code)
	if _, seen := pr.examples[status]; !seen {
		pr.order = append(pr.order, status)
		pr.examples[status] = nil
	}
	base := stateBases[status]
	if base == "" {
		base = "status_" + status
	}
	name := stateName(resp.str("name"), base)
	for n := 2; containsState(pr.route, name); n++ {
		name = stateName(resp.str("name"), base) + "_" + strconv.Itoa(n)
	}
	st := &model.State{Status: code}
	if text := resp.str("body"); text != "" {
		if v, err := parse([]byte(text)); err == nil && isContainer(v) {
			st.Body = model.Body{Data: encode(v), ContentType: jsonTextType}
			pr.examples[status] = append(pr.examples[status], v)
		} else {
			st.Body = model.Body{Data: []byte(text), ContentType: "text/plain; charset=utf-8"}
		}
	}
	pr.route.States = append(pr.route.States, model.NamedState{Name: name, State: st})
}

func containsState(r *model.Route, name string) bool {
	_, ok := r.States.Get(name)
	return ok
}

func isContainer(v any) bool {
	switch v.(type) {
	case *omap, []any:
		return true
	}
	return false
}

func kindOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case json.Number:
		if strings.ContainsAny(x.String(), ".eE") {
			return "number"
		}
		return "integer"
	case string:
		return "string"
	case []any:
		return "array"
	case *omap:
		return "object"
	}
	return "string"
}

func infer(values []any) any {
	kinds, groups, nullable := group(values)
	if len(kinds) == 0 {
		return &omap{}
	}
	if len(kinds) > 1 {
		var anyOf []any
		for _, k := range kinds {
			anyOf = append(anyOf, inferKind(k, groups[k]))
		}
		if nullable {
			anyOf = append(anyOf, &omap{keys: []string{"type"}, vals: []any{"null"}})
		}
		return &omap{keys: []string{"anyOf"}, vals: []any{anyOf}}
	}
	s := inferKind(kinds[0], groups[kinds[0]])
	if nullable {
		s.set("type", []any{kinds[0], "null"})
	}
	return s
}

func group(values []any) ([]string, map[string][]any, bool) {
	var kinds []string
	nullable := false
	groups := map[string][]any{}
	for _, v := range values {
		k := kindOf(v)
		if k == "null" {
			nullable = true
			continue
		}
		if k == "integer" && groups["number"] != nil {
			k = "number"
		}
		if k == "number" && groups["integer"] != nil {
			groups["number"] = append(groups["number"], groups["integer"]...)
			delete(groups, "integer")
			kinds = replace(kinds, "integer", "number")
		}
		if _, ok := groups[k]; !ok {
			kinds = append(kinds, k)
		}
		groups[k] = append(groups[k], v)
	}
	return kinds, groups, nullable
}

func replace(list []string, from, to string) []string {
	for i, s := range list {
		if s == from {
			list[i] = to
		}
	}
	return list
}

func inferKind(kind string, values []any) *omap {
	s := &omap{}
	s.set("type", kind)
	switch kind {
	case "object":
		inferObject(s, values)
	case "array":
		items := make([]any, 0, len(values))
		for _, v := range values {
			items = append(items, asList(v)...)
		}
		if len(items) > 0 {
			s.set("items", infer(items))
		}
	}
	return s
}

func inferObject(s *omap, values []any) {
	var order []string
	seen := map[string][]any{}
	for _, v := range values {
		o, _ := v.(*omap)
		for i, k := range o.keys {
			if _, ok := seen[k]; !ok {
				order = append(order, k)
			}
			seen[k] = append(seen[k], o.vals[i])
		}
	}
	var required []any
	props := &omap{}
	for _, k := range order {
		if len(seen[k]) == len(values) {
			required = append(required, k)
		}
		props.set(k, infer(seen[k]))
	}
	if len(required) > 0 {
		s.set("required", required)
	}
	s.set("properties", props)
}

type postmanCollection struct {
	Info     postmanInfo       `json:"info"`
	Item     []postmanFolder   `json:"item"`
	Variable []postmanVariable `json:"variable"`
}

type postmanInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type postmanFolder struct {
	Name string        `json:"name"`
	Item []postmanItem `json:"item"`
}

type postmanItem struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Request     postmanRequest    `json:"request"`
	Response    []postmanResponse `json:"response"`
}

type postmanRequest struct {
	Method string            `json:"method"`
	Header []postmanVariable `json:"header"`
	URL    postmanURLOut     `json:"url"`
}

type postmanURLOut struct {
	Raw      string            `json:"raw"`
	Host     []string          `json:"host"`
	Path     []string          `json:"path"`
	Variable []postmanVariable `json:"variable,omitempty"`
}

type postmanResponse struct {
	Name   string            `json:"name"`
	Status string            `json:"status"`
	Code   int               `json:"code"`
	Header []postmanVariable `json:"header,omitempty"`
	Body   string            `json:"body"`
}

type postmanVariable struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled,omitempty"`
}

type postmanEnvironment struct {
	Name   string            `json:"name"`
	Values []postmanVariable `json:"values"`
	Scope  string            `json:"_postman_variable_scope"`
}

func ExportPostman(p *model.Project, info Info) ([]byte, []byte, error) {
	title := orDefault(info.Title, "MockMachina contract")
	c := postmanCollection{Info: postmanInfo{Name: title, Schema: postmanSchema}, Variable: []postmanVariable{{Key: "baseUrl", Value: mockBaseURL}}}
	index := map[string]int{}
	for _, r := range p.Routes {
		i, ok := index[r.Group]
		if !ok {
			i = len(c.Item)
			index[r.Group] = i
			c.Item = append(c.Item, postmanFolder{Name: r.Group})
		}
		for _, ns := range r.States {
			c.Item[i].Item = append(c.Item[i].Item, postmanStateItem(r, ns))
		}
	}
	collection, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	env := postmanEnvironment{Name: title + " mock", Values: []postmanVariable{{Key: "baseUrl", Value: mockBaseURL, Enabled: true}}, Scope: "environment"}
	environment, err := json.MarshalIndent(env, "", "  ")
	return collection, environment, err
}

func postmanStateItem(r *model.Route, ns model.NamedState) postmanItem {
	method := string(r.Method)
	if r.Method == model.MethodCRUD {
		method = http.MethodGet
	}
	segments := strings.Split(strings.Trim(r.Path, "/"), "/")
	var vars []postmanVariable
	for i, s := range segments {
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			name := s[1 : len(s)-1]
			segments[i] = ":" + name
			vars = append(vars, postmanVariable{Key: name, Value: r.Examples[name]})
		}
	}
	st := ns.State
	resp := postmanResponse{Name: ns.Name, Status: http.StatusText(st.EffectiveStatus()), Code: st.EffectiveStatus(), Body: string(st.Body.Data)}
	if st.Body.ContentType != "" {
		resp.Header = []postmanVariable{{Key: "Content-Type", Value: st.Body.ContentType}}
	}
	description := routeMarker + r.ID
	if r.Summary != "" {
		description += "\n" + r.Summary
	}
	return postmanItem{
		Name:        r.ID + " · " + ns.Name,
		Description: description,
		Request: postmanRequest{
			Method: method,
			Header: []postmanVariable{{Key: "X-Mock-State", Value: ns.Name}},
			URL:    postmanURLOut{Raw: "{{baseUrl}}/" + strings.Join(segments, "/"), Host: []string{"{{baseUrl}}"}, Path: segments, Variable: vars},
		},
		Response: []postmanResponse{resp},
	}
}
