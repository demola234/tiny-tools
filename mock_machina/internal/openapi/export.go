package openapi

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const extension = "x-mockmachina"

type Info struct {
	Title   string
	Version string
}

func Export(p *model.Project, info Info) ([]byte, error) {
	doc := &omap{}
	doc.set("openapi", "3.1.0")
	meta := &omap{}
	meta.set("title", orDefault(info.Title, "MockMachina contract"))
	meta.set("version", orDefault(info.Version, "0.0.0"))
	doc.set("info", meta)
	paths := &omap{}
	for _, r := range p.Routes {
		for _, op := range operations(r) {
			item, _ := paths.get(op.path).(*omap)
			if item == nil {
				item = &omap{}
				paths.set(op.path, item)
			}
			item.set(op.method, op.body)
		}
	}
	sortPaths(paths)
	doc.set("paths", paths)
	if len(p.Schemas) > 0 {
		schemas := &omap{}
		for _, s := range p.Schemas {
			v, err := parse(s.JSON)
			if err != nil {
				return nil, err
			}
			schemas.set(s.Name, toComponents(v))
		}
		comps := &omap{}
		comps.set("schemas", schemas)
		doc.set("components", comps)
	}
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(yamlNode(doc)); err != nil {
		return nil, err
	}
	return []byte(b.String()), enc.Close()
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

type exportedOp struct {
	path, method string
	body         *omap
}

func operations(r *model.Route) []exportedOp {
	if r.Method != model.MethodCRUD {
		return []exportedOp{{path: r.Path, method: strings.ToLower(string(r.Method)), body: operation(r)}}
	}
	one := r.Path + "/{id}"
	main := operation(r)
	part := func(id string) *omap {
		op := &omap{}
		op.set("operationId", r.ID+"-"+id)
		ext := &omap{}
		ext.set("partOf", r.ID)
		op.set(extension, ext)
		op.set("responses", &omap{keys: []string{"200"}, vals: []any{&omap{keys: []string{"description"}, vals: []any{"OK"}}}})
		return op
	}
	return []exportedOp{
		{r.Path, "get", main},
		{r.Path, "post", part("create")},
		{one, "get", part("get")},
		{one, "put", part("replace")},
		{one, "patch", part("update")},
		{one, "delete", part("delete")},
	}
}

func sortPaths(paths *omap) {
	order := slices.Clone(paths.keys)
	slices.Sort(order)
	vals := make([]any, len(order))
	for i, k := range order {
		vals[i] = paths.get(k)
	}
	paths.keys, paths.vals = order, vals
	for _, v := range paths.vals {
		item, _ := v.(*omap)
		sortMethods(item)
	}
}

func sortMethods(item *omap) {
	var keys []string
	var vals []any
	for _, m := range methods {
		if v := item.get(m); v != nil {
			keys, vals = append(keys, m), append(vals, v)
		}
	}
	item.keys, item.vals = keys, vals
}

func operation(r *model.Route) *omap {
	op := &omap{}
	op.set("operationId", r.ID)
	if r.Summary != "" {
		op.set("summary", r.Summary)
	}
	if r.Request != nil {
		if params := parametersOut(r.Request); len(params) > 0 {
			op.set("parameters", params)
		}
		if r.Request.Body != nil {
			media := &omap{}
			media.set("schema", schemaOut(*r.Request.Body))
			content := &omap{}
			content.set("application/json", media)
			body := &omap{}
			body.set("required", true)
			body.set("content", content)
			op.set("requestBody", body)
		}
	}
	op.set("responses", responsesOut(r))
	for _, e := range r.Extensions {
		op.set(e.Key, extensionValue(e.Value))
	}
	if ext := routeExtension(r); len(ext.keys) > 0 {
		op.set(extension, ext)
	}
	return op
}

func parametersOut(req *model.Request) []any {
	var out []any
	for _, part := range []struct {
		in     string
		params []model.Param
	}{{"path", req.Params}, {"query", req.Query}, {"header", req.Headers}} {
		for _, p := range part.params {
			o := &omap{}
			o.set("name", p.Name)
			o.set("in", part.in)
			if p.Required || part.in == "path" {
				o.set("required", true)
			}
			o.set("schema", schemaOut(p.Schema))
			out = append(out, o)
		}
	}
	return out
}

func schemaOut(ref model.SchemaRef) any {
	if ref.Inline == nil {
		return &omap{keys: []string{"$ref"}, vals: []any{"#/components/schemas/" + ref.Name}}
	}
	v, err := parse(ref.Inline.JSON)
	if err != nil {
		return &omap{}
	}
	return toComponents(v)
}

func toComponents(v any) any {
	switch x := v.(type) {
	case *omap:
		if ref, ok := x.get("$ref").(string); ok && !strings.ContainsAny(ref, "#:/") {
			x.set("$ref", "#/components/schemas/"+ref)
		}
		for _, c := range x.vals {
			toComponents(c)
		}
	case []any:
		for _, c := range x {
			toComponents(c)
		}
	}
	return v
}

func responsesOut(r *model.Route) *omap {
	out := &omap{}
	documented := map[string]bool{}
	for _, resp := range r.Responses {
		documented[resp.Status] = true
	}
	for _, resp := range r.Responses {
		out.set(resp.Status, responseOut(r, resp.Status, &resp.Schema, func(status int) bool {
			code := strconv.Itoa(status)
			return code == resp.Status || (resp.Status == "default" && !documented[code])
		}))
	}
	for _, ns := range r.States {
		code := strconv.Itoa(ns.State.EffectiveStatus())
		if !documented[code] && !documented["default"] && out.get(code) == nil {
			out.set(code, responseOut(r, code, nil, func(status int) bool { return strconv.Itoa(status) == code }))
		}
	}
	return out
}

func responseOut(r *model.Route, code string, schema *model.SchemaRef, owns func(int) bool) *omap {
	resp := &omap{}
	resp.set("description", description(code))
	media := &omap{}
	if schema != nil && (schema.Inline == nil || strings.TrimSpace(string(schema.Inline.JSON)) != "{}") {
		media.set("schema", schemaOut(*schema))
	}
	examples := &omap{}
	for _, ns := range r.States {
		if owns(ns.State.EffectiveStatus()) {
			examples.set(ns.Name, exampleOut(ns.State, code))
		}
	}
	if len(examples.keys) > 0 {
		media.set("examples", examples)
	}
	if len(media.keys) > 0 {
		content := &omap{}
		content.set("application/json", media)
		resp.set("content", content)
	}
	return resp
}

func description(code string) string {
	n, err := strconv.Atoi(code)
	if text := http.StatusText(n); err == nil && text != "" {
		return text
	}
	return "Default response"
}

func exampleOut(st *model.State, code string) *omap {
	ex := &omap{}
	ext := &omap{}
	switch {
	case st.Body.Generate:
		ext.set("generate", true)
	case len(st.Body.Data) > 0:
		if v, err := parse(st.Body.Data); err == nil {
			ex.set("value", v)
		}
	}
	if strconv.Itoa(st.EffectiveStatus()) != code {
		ext.set("status", json.Number(strconv.Itoa(st.EffectiveStatus())))
	}
	stateExtras(st, ext)
	ex.set(extension, ext)
	return ex
}

func stateExtras(st *model.State, ext *omap) {
	if len(st.Headers) > 0 {
		h := &omap{}
		for _, k := range slices.Sorted(maps.Keys(st.Headers)) {
			h.set(k, st.Headers[k])
		}
		ext.set("headers", h)
	}
	if st.Latency > 0 || st.Jitter > 0 {
		lat := &omap{}
		lat.set("base", st.Latency.String())
		if st.Jitter > 0 {
			lat.set("jitter", st.Jitter.String())
		}
		ext.set("latency", lat)
	}
	if f := st.Fault; f != nil {
		fault := &omap{}
		fault.set("type", string(f.Type))
		fault.set("rate", json.Number(strconv.FormatFloat(f.Rate, 'f', -1, 64)))
		if f.After > 0 {
			fault.set("after", f.After.String())
		}
		ext.set("fault", fault)
	}
	if len(st.Set) > 0 {
		set := &omap{}
		for _, a := range st.Set {
			set.set(a.Name, plain(a.Value))
		}
		ext.set("set", set)
	}
	if st.Verbatim {
		ext.set("template", false)
	}
	if st.SkipRequestValidation {
		ext.set("validateRequest", false)
	}
}

func plain(v any) any {
	if f, ok := v.(float64); ok {
		return json.Number(strconv.FormatFloat(f, 'f', -1, 64))
	}
	return v
}

func routeExtension(r *model.Route) *omap {
	ext := &omap{}
	if r.Status != "" {
		ext.set("status", string(r.Status))
	}
	if !r.Owners.IsZero() {
		owners := &omap{}
		if len(r.Owners.Backend) > 0 {
			owners.set("backend", anyList(r.Owners.Backend))
		}
		if len(r.Owners.Frontend) > 0 {
			owners.set("frontend", anyList(r.Owners.Frontend))
		}
		ext.set("owners", owners)
	}
	if len(r.States) > 0 && r.Active != "" && r.Active != r.States[0].Name {
		ext.set("active", r.Active)
	}
	if r.Mode != "" && r.Mode != model.ModeActive && (r.Mode != model.ModeRules || len(r.Rules) == 0) {
		ext.set("mode", string(r.Mode))
	}
	if len(r.Rules) > 0 {
		ext.set("rules", rulesOut(r.Rules))
	}
	if r.Serve == model.ServeProxy {
		ext.set("serve", string(r.Serve))
	}
	if r.CRUD != nil {
		crud := &omap{}
		crud.set("collection", r.CRUD.Collection)
		crud.set("idField", r.CRUD.IDField)
		ext.set("crud", crud)
		ext.set("route", "CRUD "+r.Path)
	}
	return ext
}

func anyList(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func rulesOut(rules []model.Rule) []any {
	out := make([]any, len(rules))
	for i, rule := range rules {
		when := &omap{}
		for _, c := range rule.When {
			selector := string(c.Scope)
			if c.Key != "" {
				selector += "." + c.Key
			}
			when.set(selector, matcherOut(c))
		}
		r := &omap{}
		r.set("when", when)
		r.set("state", rule.State)
		out[i] = r
	}
	return out
}

func matcherOut(c model.Condition) any {
	switch c.Op {
	case model.OpEq:
		return plain(c.Value)
	case model.OpIn:
		values := make([]any, len(c.Values))
		for i, v := range c.Values {
			values[i] = plain(v)
		}
		return &omap{keys: []string{"in"}, vals: []any{values}}
	default:
		return &omap{keys: []string{string(c.Op)}, vals: []any{plain(c.Value)}}
	}
}

func extensionValue(v any) any {
	data, ok := v.([]byte)
	if !ok {
		var err error
		if data, err = json.Marshal(v); err != nil {
			return nil
		}
	}
	parsed, err := parse(data)
	if err != nil {
		return nil
	}
	return parsed
}

func yamlNode(v any) *yaml.Node {
	switch x := v.(type) {
	case *omap:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for i, k := range x.keys {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, yamlNode(x.vals[i]))
		}
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, c := range x {
			n.Content = append(n.Content, yamlNode(c))
		}
		if len(x) == 0 {
			n.Style = yaml.FlowStyle
		}
		return n
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x}
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(x.String(), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: x.String()}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(x)}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
}
