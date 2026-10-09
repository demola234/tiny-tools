package config

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/schema"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

var requestFields = []string{"params", "query", "headers", "body"}

func (l *loader) contract(f map[string]*yaml.Node, r *model.Route) {
	if v := f["request"]; v != nil {
		r.Request = l.requestSpec(r, v)
	}
	if v := f["responses"]; v != nil {
		r.Responses = l.responses(r, v)
	}
}

func (l *loader) requestSpec(r *model.Route, v *yaml.Node) *model.Request {
	file := r.Src.File
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "request must be a mapping of params, query, headers and body")
		return nil
	}
	f := l.fields(file, v, requestFields, nil)
	req := &model.Request{}
	if n := f["params"]; n != nil {
		req.Params = l.params(r, "params", n)
	}
	if n := f["query"]; n != nil {
		req.Query = l.params(r, "query", n)
	}
	if n := f["headers"]; n != nil {
		req.Headers = l.params(r, "headers", n)
	}
	if n := f["body"]; n != nil {
		if ref, ok := l.schemaRef(r, "request.body", n); ok {
			req.Body = &ref
		}
	}
	return req
}

func (l *loader) params(r *model.Route, kind string, v *yaml.Node) []model.Param {
	file := r.Src.File
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "request.%s must map names to schemas, like { page: { type: integer } }", kind)
		return nil
	}
	var pathParams []string
	for _, m := range pathParamName.FindAllStringSubmatch(r.Path, -1) {
		pathParams = append(pathParams, m[1])
	}
	var out []model.Param
	for key, val := range pairs(v) {
		name := key.Value
		if kind == "params" && !slices.Contains(pathParams, name) {
			l.add(file, key.Line, "request.params.%s isn't a parameter of %s %s (%s)", name, r.Method, r.Path,
				closeMatchOr(name, pathParams, "parameters: "+strings.Join(pathParams, ", ")))
			continue
		}
		required := kind == "params"
		if val.Kind == yaml.MappingNode {
			val, required = withoutRequired(val, required)
		}
		if ref, ok := l.schemaRef(r, "request."+kind+"."+name, val); ok {
			out = append(out, model.Param{Name: name, Required: required, Schema: ref})
		}
	}
	return out
}

func withoutRequired(n *yaml.Node, required bool) (*yaml.Node, bool) {
	out := *n
	out.Content = nil
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Value == "required" && v.Tag == "!!bool" {
			required = required || v.Value == "true"
			continue
		}
		out.Content = append(out.Content, k, v)
	}
	return &out, required
}

func (l *loader) responses(r *model.Route, v *yaml.Node) []model.Response {
	file := r.Src.File
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "responses must map statuses to schemas, like { 200: UserList, 404: Error }")
		return nil
	}
	var out []model.Response
	for key, val := range pairs(v) {
		status := key.Value
		if n, err := strconv.Atoi(status); status != "default" && (err != nil || n < minStatus || n > maxStatus) {
			l.add(file, key.Line, "responses.%s isn't an HTTP status (%d-%d) or default", status, minStatus, maxStatus)
			continue
		}
		if ref, ok := l.schemaRef(r, "responses."+status, val); ok {
			out = append(out, model.Response{Status: status, Schema: ref})
		}
	}
	return out
}

func (l *loader) schemaRef(r *model.Route, label string, v *yaml.Node) (model.SchemaRef, bool) {
	file := r.Src.File
	switch v.Kind {
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
	case yaml.ScalarNode:
		if !slices.Contains(l.schemaNames, v.Value) {
			l.add(file, v.Line, "%s: %q isn't a schema (%s)", label, v.Value, closeMatchOr(v.Value, l.schemaNames, "schemas: "+strings.Join(l.schemaNames, ", ")))
			return model.SchemaRef{}, false
		}
		return model.SchemaRef{Name: v.Value}, true
	case yaml.MappingNode:
		var refs []schemaRef
		w := schemaWalk{l: l, name: r.ID + " " + label, file: file, lines: map[string]int{}, refs: &refs}
		w.node(v, "")
		for _, ref := range refs {
			if !slices.Contains(l.schemaNames, ref.name) {
				l.add(file, ref.line, "$ref %q isn't a schema (%s)", ref.name, closeMatchOr(ref.name, l.schemaNames, "schemas: "+strings.Join(l.schemaNames, ", ")))
			}
		}
		data, err := toJSON(v)
		if err != nil {
			l.add(file, v.Line, "%s can't be written as JSON: %v", label, err)
			return model.SchemaRef{}, false
		}
		return model.SchemaRef{Inline: &model.Schema{Name: label, JSON: data, Lines: w.lines, Src: model.Source{File: file, Line: v.Line}}}, true
	}
	l.add(file, v.Line, "%s must be a schema name or a schema, like UserList or { type: object }", label)
	return model.SchemaRef{}, false
}

func (l *loader) checkBodies(p *model.Project) {
	set, probs := schema.New(p.Schemas)
	if len(probs) > 0 {
		return
	}
	for _, r := range p.Routes {
		for _, ns := range r.States {
			l.checkState(set, p.Schemas, r, ns)
		}
	}
}

func (l *loader) checkState(set *schema.Set, schemas []*model.Schema, r *model.Route, ns model.NamedState) {
	st := ns.State
	status := strconv.Itoa(st.EffectiveStatus())
	i := slices.IndexFunc(r.Responses, func(resp model.Response) bool { return resp.Status == status })
	if i < 0 {
		i = slices.IndexFunc(r.Responses, func(resp model.Response) bool { return resp.Status == "default" })
	}
	switch {
	case i < 0 && st.Body.Generate:
		l.add(st.Src.File, st.Src.Line, "state %q has body: generate, but %s has no response schema for %s", ns.Name, r.ID, status)
		return
	case i < 0 && len(r.Responses) > 0:
		l.add(st.Src.File, st.Src.Line, "state %q returns %s, which %s doesn't document (add it to responses, or a default)", ns.Name, status, r.ID)
		return
	case i < 0:
		return
	}
	if st.Body.Generate {
		data, err := schema.Generate(schemas, r.Responses[i].Schema)
		if err != nil {
			l.add(st.Src.File, st.Src.Line, "state %q: couldn't generate a body: %v", ns.Name, err)
			return
		}
		st.Body.Data = data
	}
	if len(st.Body.Data) == 0 || !strings.HasPrefix(st.Body.ContentType, jsonType) {
		return
	}
	ref := r.Responses[i].Schema
	sc, label, err := set.Ref(ref)
	if err != nil {
		l.add(r.Src.File, r.Src.Line, "%s: %v", label, err)
		return
	}
	skip := templatedPaths(st.Body.Data)
	for _, v := range set.Validate(sc, st.Body.Data) {
		switch {
		case skipped(v, skip):
		case st.Body.Generate:
			l.add(st.Src.File, st.Src.Line, "state %q: the body generated from %s doesn't match it: %s (add an example to the schema)", ns.Name, label, v)
		default:
			l.add(st.Src.File, st.Src.Line, "state %q doesn't match %s: %s", ns.Name, label, v)
		}
	}
}

func templatedPaths(data []byte) []string {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return nil
	}
	var out []string
	var walk func(v any, loc []string)
	walk = func(v any, loc []string) {
		switch x := v.(type) {
		case string:
			if tmpl.Has([]byte(x)) {
				out = append(out, schema.Where(loc))
			}
		case map[string]any:
			for k, c := range x {
				walk(c, append(slices.Clone(loc), k))
			}
		case []any:
			for i, c := range x {
				walk(c, append(slices.Clone(loc), strconv.Itoa(i)))
			}
		}
	}
	walk(v, nil)
	return out
}

func skipped(violation string, paths []string) bool {
	at, _, _ := strings.Cut(violation, ": ")
	for _, p := range paths {
		if at == p || strings.HasPrefix(at, p+".") || strings.HasPrefix(at, p+"[") {
			return true
		}
	}
	return false
}
