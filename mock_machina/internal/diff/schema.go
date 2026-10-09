package diff

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type field struct {
	types    []string
	required bool
}

type fields map[string]field

type schemaIndex map[string]any

func index(schemas []*model.Schema) schemaIndex {
	idx := schemaIndex{}
	for _, s := range schemas {
		var v any
		if json.Unmarshal(s.JSON, &v) == nil {
			idx[s.Name] = v
		}
	}
	return idx
}

func (idx schemaIndex) flatten(ref model.SchemaRef) fields {
	var root any
	if ref.Inline != nil {
		_ = json.Unmarshal(ref.Inline.JSON, &root)
	} else {
		root = idx[ref.Name]
	}
	out := fields{}
	idx.walk(out, root, "", true, nil)
	return out
}

func (idx schemaIndex) walk(out fields, v any, path string, required bool, stack []string) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if ref, ok := m["$ref"].(string); ok {
		if slices.Contains(stack, ref) {
			return
		}
		idx.walk(out, idx[ref], path, required, append(stack, ref))
		return
	}
	if all, ok := m["allOf"].([]any); ok {
		for _, part := range all {
			idx.walk(out, part, path, required, stack)
		}
	}
	record(out, path, types(m), required)
	need := requiredSet(m)
	if props, ok := m["properties"].(map[string]any); ok {
		for name, child := range props {
			idx.walk(out, child, join(path, name), need[name], stack)
		}
	}
	if items, ok := m["items"]; ok {
		idx.walk(out, items, path+"[]", true, stack)
	}
}

func requiredSet(m map[string]any) map[string]bool {
	need := map[string]bool{}
	list, _ := m["required"].([]any)
	for _, n := range list {
		if s, ok := n.(string); ok {
			need[s] = true
		}
	}
	return need
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func types(m map[string]any) []string {
	switch t := m["type"].(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if _, ok := m["properties"]; ok {
		return []string{"object"}
	}
	return nil
}

func record(out fields, path string, ts []string, required bool) {
	f := out[path]
	for _, t := range ts {
		if !slices.Contains(f.types, t) {
			f.types = append(f.types, t)
		}
	}
	f.required = f.required || required
	out[path] = f
}

func missing(from, in []string) []string {
	var out []string
	for _, t := range from {
		if !slices.Contains(in, t) {
			out = append(out, t)
		}
	}
	return out
}

func (c *collector) schemas(b, h *model.Route) {
	for _, hr := range h.Responses {
		i := slices.IndexFunc(b.Responses, func(br model.Response) bool { return br.Status == hr.Status })
		if i >= 0 {
			c.response(h, "response "+hr.Status+": ", c.base.flatten(b.Responses[i].Schema), c.head.flatten(hr.Schema))
		}
	}
	br, hr := requestOf(b), requestOf(h)
	if br.Body != nil && hr.Body != nil {
		c.request(h, "request body: ", c.base.flatten(*br.Body), c.head.flatten(*hr.Body))
	}
	for _, part := range []struct {
		name string
		b, h []model.Param
	}{{"params", br.Params, hr.Params}, {"query", br.Query, hr.Query}, {"headers", br.Headers, hr.Headers}} {
		c.request(h, "request "+part.name+": ", c.base.params(part.b), c.head.params(part.h))
	}
}

func requestOf(r *model.Route) model.Request {
	if r.Request == nil {
		return model.Request{}
	}
	return *r.Request
}

func (idx schemaIndex) params(list []model.Param) fields {
	out := fields{}
	for _, p := range list {
		f := idx.flatten(p.Schema)[""]
		f.required = p.Required
		out[p.Name] = f
	}
	return out
}

func (c *collector) response(r *model.Route, prefix string, was, now fields) {
	for path, b := range was {
		if path == "" {
			continue
		}
		h, ok := now[path]
		switch {
		case !ok:
			c.add(Breaking, r, "", "%s%s removed", prefix, path)
			continue
		case len(b.types) > 0 && slices.Contains(missing(h.types, b.types), "null"):
			c.add(Breaking, r, "", "%s%s can now be null", prefix, path)
		case len(b.types) > 0 && len(missing(h.types, b.types)) > 0:
			c.add(Breaking, r, "", "%s%s is now %s, was %s", prefix, path, strings.Join(h.types, " or "), strings.Join(b.types, " or "))
		}
		if b.required && !h.required {
			c.add(Warning, r, "", "%s%s is no longer required, so apps may not get it", prefix, path)
		}
	}
	for path := range now {
		if _, ok := was[path]; !ok && path != "" {
			c.add(Safe, r, "", "%s%s added", prefix, path)
		}
	}
}

func (c *collector) request(r *model.Route, prefix string, was, now fields) {
	for path, h := range now {
		if path == "" {
			continue
		}
		b, ok := was[path]
		switch {
		case h.required && (!ok || !b.required):
			c.add(Breaking, r, "", "%s%s is now required", prefix, path)
		case !ok:
			c.add(Safe, r, "", "%s%s added", prefix, path)
		}
		if ok && len(h.types) > 0 {
			c.narrowed(r, prefix, path, b, h)
		}
	}
	for path := range was {
		if _, ok := now[path]; !ok && path != "" {
			c.add(Info, r, "", "%s%s removed", prefix, path)
		}
	}
}

func (c *collector) narrowed(r *model.Route, prefix, path string, b, h field) {
	dropped := missing(b.types, h.types)
	switch {
	case slices.Contains(dropped, "null"):
		c.add(Breaking, r, "", "%s%s no longer accepts null", prefix, path)
	case len(dropped) > 0:
		c.add(Breaking, r, "", "%s%s is now %s, was %s", prefix, path, strings.Join(h.types, " or "), strings.Join(b.types, " or "))
	}
}
