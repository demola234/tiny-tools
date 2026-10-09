package openapi

import (
	"slices"
	"strings"
)

var schemaFields = []string{
	"type", "format", "items", "enum", "default", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
	"minLength", "maxLength", "pattern", "minItems", "maxItems", "uniqueItems", "multipleOf",
}

type swagger struct {
	doc      *omap
	consumes []any
	produces []any
}

func convert2(doc *omap) *omap {
	s := swagger{doc: doc, consumes: list(doc, "consumes"), produces: list(doc, "produces")}
	out := &omap{}
	out.set("openapi", "3.0.0")
	out.set("info", doc.get("info"))
	if server := s.server(); server != "" {
		out.set("servers", []any{&omap{keys: []string{"url"}, vals: []any{server}}})
	}
	comps := &omap{}
	if defs := doc.obj("definitions"); defs != nil {
		comps.set("schemas", defs)
	}
	if responses := doc.obj("responses"); responses != nil {
		converted := &omap{}
		for i, k := range responses.keys {
			r, _ := responses.vals[i].(*omap)
			converted.set(k, response(r, s.produces))
		}
		comps.set("responses", converted)
	}
	paths := &omap{}
	for i, p := range doc.obj("paths").keysOrNil() {
		item, _ := doc.obj("paths").vals[i].(*omap)
		paths.set(p, s.item(item))
	}
	out.set("paths", paths)
	out.set("components", comps)
	rewriteRefs(out)
	return out
}

func (m *omap) keysOrNil() []string {
	if m == nil {
		return nil
	}
	return m.keys
}

func list(m *omap, key string) []any {
	l, _ := m.get(key).([]any)
	return l
}

func (s swagger) server() string {
	host, base := s.doc.str("host"), s.doc.str("basePath")
	if host == "" {
		return base
	}
	scheme := "https"
	if schemes := list(s.doc, "schemes"); len(schemes) > 0 {
		scheme, _ = schemes[0].(string)
	}
	return scheme + "://" + host + base
}

func (s swagger) item(item *omap) *omap {
	out := &omap{}
	for i, k := range item.keysOrNil() {
		switch {
		case k == "parameters":
			out.set(k, s.plainParams(item.vals[i]))
		case slices.Contains(methods, k):
			op, _ := item.vals[i].(*omap)
			out.set(k, s.operation(op))
		default:
			out.set(k, item.vals[i])
		}
	}
	return out
}

func (s swagger) plainParams(v any) []any {
	var out []any
	for _, raw := range asList(v) {
		p := s.resolveParam(raw)
		if in := p.str("in"); in != "body" && in != "formData" {
			out = append(out, param(p))
		}
	}
	return out
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func (s swagger) resolveParam(raw any) *omap {
	p, _ := raw.(*omap)
	if ref, ok := p.get("$ref").(string); ok {
		if name, ok := strings.CutPrefix(ref, "#/parameters/"); ok {
			resolved, _ := s.doc.obj("parameters").get(name).(*omap)
			return resolved
		}
	}
	return p
}

func (s swagger) operation(op *omap) *omap {
	consumes, produces := list(op, "consumes"), list(op, "produces")
	if len(consumes) == 0 {
		consumes = s.consumes
	}
	if len(produces) == 0 {
		produces = s.produces
	}
	out := &omap{}
	var params []any
	var body *omap
	form := &omap{keys: []string{"type"}, vals: []any{"object"}}
	formProps := &omap{}
	for _, raw := range asList(op.get("parameters")) {
		p := s.resolveParam(raw)
		switch p.str("in") {
		case "body":
			body = &omap{}
			body.set("required", p.get("required") == true)
			body.set("content", media(consumes, p.get("schema"), nil))
		case "formData":
			formProps.set(p.str("name"), schemaOf(p))
		default:
			params = append(params, param(p))
		}
	}
	if len(formProps.keys) > 0 {
		form.set("properties", formProps)
		body = &omap{}
		body.set("content", media(consumes, form, nil))
	}
	for i, k := range op.keys {
		switch k {
		case "parameters", "consumes", "produces":
		case "responses":
			out.set(k, responses(op.vals[i], produces))
		default:
			out.set(k, op.vals[i])
		}
	}
	if len(params) > 0 {
		out.set("parameters", params)
	}
	if body != nil {
		out.set("requestBody", body)
	}
	return out
}

func responses(v any, produces []any) *omap {
	in, _ := v.(*omap)
	out := &omap{}
	for i, code := range in.keysOrNil() {
		r, _ := in.vals[i].(*omap)
		out.set(code, response(r, produces))
	}
	return out
}

func response(r *omap, produces []any) *omap {
	if ref, ok := r.get("$ref").(string); ok {
		return &omap{keys: []string{"$ref"}, vals: []any{ref}}
	}
	out := &omap{}
	out.set("description", r.str("description"))
	if schema := r.get("schema"); schema != nil {
		var example any
		if examples := r.obj("examples"); examples != nil && len(examples.keys) > 0 {
			example = examples.vals[0]
			for i, k := range examples.keys {
				if strings.Contains(k, "json") {
					example = examples.vals[i]
				}
			}
		}
		out.set("content", media(produces, schema, example))
	}
	return out
}

func media(types []any, schema, example any) *omap {
	mime := "application/json"
	for _, t := range types {
		if s, ok := t.(string); ok {
			mime = s
			if strings.Contains(s, "json") {
				break
			}
		}
	}
	m := &omap{}
	m.set("schema", schema)
	if example != nil {
		m.set("example", example)
	}
	content := &omap{}
	content.set(mime, m)
	return content
}

func param(p *omap) *omap {
	out := &omap{}
	for _, k := range []string{"name", "in", "description", "required"} {
		if v := p.get(k); v != nil {
			out.set(k, v)
		}
	}
	out.set("schema", schemaOf(p))
	return out
}

func schemaOf(p *omap) *omap {
	schema := &omap{}
	for _, k := range p.keys {
		if slices.Contains(schemaFields, k) {
			schema.set(k, p.get(k))
		}
	}
	return schema
}

func rewriteRefs(v any) {
	switch x := v.(type) {
	case *omap:
		if ref, ok := x.get("$ref").(string); ok {
			ref = strings.Replace(ref, "#/definitions/", "#/components/schemas/", 1)
			ref = strings.Replace(ref, "#/responses/", "#/components/responses/", 1)
			x.set("$ref", ref)
		}
		for _, c := range x.vals {
			rewriteRefs(c)
		}
	case []any:
		for _, c := range x {
			rewriteRefs(c)
		}
	}
}
