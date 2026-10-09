package openapi

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const schemasFile = "schemas/api.yaml"

type Result struct {
	Title    string
	Version  string
	BasePath string
	Routes   []*model.Route
	Schemas  []*model.Schema
	Notes    []string
}

var (
	methods       = []string{"get", "put", "post", "delete", "options", "head", "patch"}
	ignoredHeader = []string{"accept", "content-type", "authorization"}
	notName       = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
	notStateChars = regexp.MustCompile(`[^a-z0-9]+`)
	notRouteChars = regexp.MustCompile(`[^a-z0-9]+`)
	camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	stateBases    = map[string]string{
		"200": "success", "201": "created", "202": "accepted", "204": "no_content", "301": "moved", "302": "found",
		"304": "not_modified", "400": "bad_request", "401": "unauthorized", "403": "forbidden", "404": "not_found",
		"405": "not_allowed", "409": "conflict", "410": "gone", "422": "invalid", "429": "too_many_requests",
		"500": "server_error", "502": "bad_gateway", "503": "unavailable", "504": "timeout", "default": "server_error",
	}
)

type importer struct {
	doc    *omap
	v30    bool
	res    *Result
	rename map[string]string
	ids    map[string]bool
}

func Import(data []byte) (*Result, error) {
	root, err := parse(data)
	if err != nil {
		return nil, err
	}
	doc, ok := root.(*omap)
	if !ok {
		return nil, errors.New("isn't an OpenAPI or Swagger document (no openapi or swagger field)")
	}
	if isPostman(doc) {
		return importPostman(doc), nil
	}
	version := doc.str("openapi")
	swagger := doc.str("swagger")
	switch {
	case version == "" && swagger == "":
		return nil, errors.New("isn't an OpenAPI or Swagger document (no openapi or swagger field)")
	case version == "" && swagger != "2.0":
		return nil, fmt.Errorf("swagger %s isn't supported (2.0 is)", swagger)
	case version == "":
		doc, version = convert2(doc), "3.0.0"
	case !strings.HasPrefix(version, "3.0") && !strings.HasPrefix(version, "3.1"):
		return nil, fmt.Errorf("OpenAPI %s isn't supported (3.0 and 3.1 are)", version)
	}
	im := &importer{
		doc: doc, v30: strings.HasPrefix(version, "3.0"), rename: map[string]string{}, ids: map[string]bool{},
		res: &Result{Title: doc.obj("info").str("title"), Version: version},
	}
	if swagger != "" {
		im.res.Version = "Swagger " + swagger
		im.note("converted from Swagger %s", swagger)
	}
	im.schemas()
	im.basePath()
	im.paths()
	return im.res, nil
}

func (im *importer) note(format string, args ...any) {
	im.res.Notes = append(im.res.Notes, fmt.Sprintf(format, args...))
}

func (im *importer) schemas() {
	comps := im.doc.obj("components").obj("schemas")
	if comps == nil {
		return
	}
	for _, name := range comps.keys {
		im.rename[name] = notName.ReplaceAllString(name, "_")
	}
	for i, name := range comps.keys {
		im.res.Schemas = append(im.res.Schemas, &model.Schema{
			Name: im.rename[name],
			JSON: encode(im.transform(clone(comps.vals[i]))),
			Src:  model.Source{File: schemasFile},
		})
	}
	slices.SortFunc(im.res.Schemas, func(a, b *model.Schema) int { return strings.Compare(a.Name, b.Name) })
}

func (im *importer) transform(v any) any {
	switch x := v.(type) {
	case *omap:
		if ref, ok := x.get("$ref").(string); ok {
			if name, ok := strings.CutPrefix(ref, "#/components/schemas/"); ok {
				x.set("$ref", im.rename[name])
			}
		}
		if im.v30 {
			convert30(x)
		}
		for i, c := range x.vals {
			x.vals[i] = im.transform(c)
		}
	case []any:
		for i, c := range x {
			x[i] = im.transform(c)
		}
	}
	return v
}

func convert30(x *omap) {
	if nullable, ok := x.get("nullable").(bool); ok {
		x.del("nullable")
		if t, ok := x.get("type").(string); ok && nullable {
			x.set("type", []any{t, "null"})
		}
	}
	for _, pair := range [][2]string{{"exclusiveMinimum", "minimum"}, {"exclusiveMaximum", "maximum"}} {
		flag, ok := x.get(pair[0]).(bool)
		if !ok {
			continue
		}
		bound := x.get(pair[1])
		if flag && bound != nil {
			x.set(pair[0], bound)
			x.del(pair[1])
			continue
		}
		x.del(pair[0])
	}
}

func (im *importer) basePath() {
	servers, _ := im.doc.get("servers").([]any)
	if len(servers) == 0 {
		return
	}
	first, _ := servers[0].(*omap)
	raw := first.str("url")
	u, err := url.Parse(raw)
	if err != nil || strings.Contains(raw, "{") {
		return
	}
	if p := strings.TrimSuffix(u.Path, "/"); p != "" {
		im.res.BasePath = p
		im.note("all routes start with %s, the path of the first server", p)
	}
}

func (im *importer) resolve(v any) *omap {
	m, _ := v.(*omap)
	for range 10 {
		ref, ok := m.get("$ref").(string)
		if !ok {
			return m
		}
		cur := any(im.doc)
		for part := range strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/") {
			o, _ := cur.(*omap)
			cur = o.get(strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~"))
		}
		m, _ = cur.(*omap)
	}
	return m
}

func (im *importer) paths() {
	paths := im.doc.obj("paths")
	if paths == nil {
		return
	}
	for i, p := range paths.keys {
		item := im.resolve(paths.vals[i])
		for _, method := range item.keys {
			if slices.Contains(methods, method) {
				im.operation(im.res.BasePath+p, strings.ToUpper(method), item, item.obj(method))
			}
		}
	}
	if callbacks := im.doc.get("webhooks"); callbacks != nil {
		im.note("webhooks aren't imported; they come with Phase 6")
	}
}

func (im *importer) operation(path, method string, item, op *omap) {
	ext := op.obj(extension)
	if ext.str("partOf") != "" {
		return
	}
	if crud, ok := strings.CutPrefix(ext.str("route"), "CRUD "); ok {
		method, path = string(model.MethodCRUD), crud
	}
	resource, name := model.RouteNames(method, path)
	if m := routeID.FindStringSubmatch(op.str("operationId")); m != nil {
		resource, name = m[1], m[2]
	}
	id := resource + "." + name
	if im.ids[id] {
		if slug := routeSlug(op.str("operationId")); slug != "" {
			name = slug
		}
		id = resource + "." + name
		for n := 2; im.ids[id]; n++ {
			id = resource + "." + name + "-" + strconv.Itoa(n)
		}
	}
	im.ids[id] = true
	r := &model.Route{
		ID: id, Group: resource, Method: model.Method(method), Path: path,
		Summary: summary(op), Src: model.Source{File: "routes/" + resource + ".yaml"},
	}
	for i, k := range op.keys {
		if strings.HasPrefix(k, "x-") && k != extension {
			r.Extensions = append(r.Extensions, model.Extension{Key: k, Value: encode(op.vals[i])})
		}
	}
	im.request(r, item, op)
	im.responses(r, op)
	applyRoute(r, ext)
	if op.get("callbacks") != nil {
		im.note("%s: callbacks aren't imported; they come with Phase 6", id)
	}
	im.res.Routes = append(im.res.Routes, r)
}

func summary(op *omap) string {
	if s := strings.TrimSpace(op.str("summary")); s != "" {
		return firstLine(s)
	}
	return firstLine(strings.TrimSpace(op.str("description")))
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}

func routeSlug(operationID string) string {
	s := camelBoundary.ReplaceAllString(operationID, "$1-$2")
	return strings.Trim(notRouteChars.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func (im *importer) request(r *model.Route, item, op *omap) {
	req := &model.Request{}
	for _, p := range im.parameters(item, op) {
		im.parameter(r, req, p)
	}
	if body := im.resolve(op.get("requestBody")); body != nil {
		if media, kind := jsonMedia(body.obj("content")); media != nil && media.get("schema") != nil {
			ref := im.schemaRef(r, media.get("schema"))
			req.Body = &ref
		} else if kind != "" {
			im.note("%s: request body as %s isn't checked", r.ID, kind)
		}
	}
	if len(req.Params)+len(req.Query)+len(req.Headers) > 0 || req.Body != nil {
		r.Request = req
	}
}

func (im *importer) parameter(r *model.Route, req *model.Request, p *omap) {
	name, in := p.str("name"), p.str("in")
	switch {
	case in == "header" && slices.Contains(ignoredHeader, strings.ToLower(name)):
		return
	case in == "cookie":
		im.note("%s: cookie parameter %q isn't checked", r.ID, name)
		return
	case p.get("schema") == nil:
		im.note("%s: parameter %q has no schema, so it isn't checked", r.ID, name)
		return
	}
	param := model.Param{Name: name, Required: p.get("required") == true || in == "path", Schema: im.schemaRef(r, p.get("schema"))}
	switch in {
	case "path":
		req.Params = append(req.Params, param)
	case "query":
		req.Query = append(req.Query, param)
	case "header":
		req.Headers = append(req.Headers, param)
	}
}

func (im *importer) parameters(item, op *omap) []*omap {
	var out []*omap
	add := func(list any) {
		items, _ := list.([]any)
		for _, raw := range items {
			p := im.resolve(raw)
			if p == nil {
				continue
			}
			key := p.str("in") + " " + p.str("name")
			if i := slices.IndexFunc(out, func(o *omap) bool { return o.str("in")+" "+o.str("name") == key }); i >= 0 {
				out[i] = p
				continue
			}
			out = append(out, p)
		}
	}
	add(item.get("parameters"))
	add(op.get("parameters"))
	return out
}

func jsonMedia(content *omap) (*omap, string) {
	if content == nil || len(content.keys) == 0 {
		return nil, ""
	}
	if m := content.obj("application/json"); m != nil {
		return m, "application/json"
	}
	for i, k := range content.keys {
		if strings.Contains(k, "json") {
			m, _ := content.vals[i].(*omap)
			return m, k
		}
	}
	return nil, content.keys[0]
}

func (im *importer) schemaRef(r *model.Route, v any) model.SchemaRef {
	if m, ok := v.(*omap); ok && len(m.keys) == 1 {
		if ref, ok := m.get("$ref").(string); ok {
			if name, ok := strings.CutPrefix(ref, "#/components/schemas/"); ok {
				return model.SchemaRef{Name: im.rename[name]}
			}
		}
	}
	return model.SchemaRef{Inline: &model.Schema{JSON: encode(im.transform(clone(v))), Src: model.Source{File: r.Src.File}}}
}

func (im *importer) responses(r *model.Route, op *omap) {
	responses := op.obj("responses")
	if responses == nil {
		return
	}
	for i, code := range responses.keys {
		status := 500
		if code != "default" {
			n, err := strconv.Atoi(code)
			if err != nil {
				im.note("%s: response %s isn't imported; give each status its own response", r.ID, code)
				continue
			}
			status = n
		}
		resp := im.resolve(responses.vals[i])
		media, _ := jsonMedia(resp.obj("content"))
		schema := model.SchemaRef{Inline: &model.Schema{JSON: []byte("{}"), Src: model.Source{File: r.Src.File}}}
		if media != nil && media.get("schema") != nil {
			schema = im.schemaRef(r, media.get("schema"))
		}
		r.Responses = append(r.Responses, model.Response{Status: code, Schema: schema})
		im.states(r, code, status, media)
	}
	r.Active = firstSuccess(r.States)
}

func (im *importer) states(r *model.Route, code string, status int, media *omap) {
	base := stateBases[code]
	if base == "" {
		base = "status_" + code
	}
	add := func(name string, body model.Body) *model.State {
		for n := 2; slices.Contains(r.States.Names(), name); n++ {
			name = strings.TrimSuffix(name, "_"+strconv.Itoa(n-1)) + "_" + strconv.Itoa(n)
		}
		st := &model.State{Status: status, Body: body}
		r.States = append(r.States, model.NamedState{Name: name, State: st})
		return st
	}
	examples := media.obj("examples")
	switch {
	case examples != nil && len(examples.keys) > 0:
		im.exampleStates(examples, base, add)
	case media != nil && media.get("example") != nil:
		add(base, jsonBody(media.get("example")))
	case media != nil && media.get("schema") != nil:
		add(base, model.Body{Generate: true, ContentType: "application/json"})
	default:
		add(base, model.Body{})
	}
}

func (im *importer) exampleStates(examples *omap, base string, add func(string, model.Body) *model.State) {
	for i, key := range examples.keys {
		example := im.resolve(examples.vals[i])
		ext := example.obj(extension)
		name := base
		if len(examples.keys) > 1 || ext != nil {
			name = stateName(key, base)
		}
		body := model.Body{}
		if example.get("value") != nil || ext == nil {
			body = jsonBody(example.get("value"))
		}
		st := add(name, body)
		if ext != nil {
			applyState(st, ext)
		}
	}
}

func jsonBody(v any) model.Body {
	return model.Body{Data: encode(v), ContentType: "application/json"}
}

func stateName(key, base string) string {
	s := strings.Trim(notStateChars.ReplaceAllString(strings.ToLower(key), "_"), "_")
	switch {
	case s == "":
		return base
	case s[0] >= '0' && s[0] <= '9':
		s = "e_" + s
	}
	if model.IsYAMLKeyword(s) {
		s += "_example"
	}
	return s
}

func firstSuccess(states model.States) string {
	for _, ns := range states {
		if ns.State.Status >= 200 && ns.State.Status < 300 {
			return ns.Name
		}
	}
	if len(states) > 0 {
		return states[0].Name
	}
	return ""
}
