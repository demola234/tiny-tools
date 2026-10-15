package openapi

import (
	"regexp"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var schemaRef = regexp.MustCompile(`"\$ref":"([^"\\]+)"`)

type renamer map[string]string

func (r *Result) RenameSchemas(to map[string]string) {
	if len(to) == 0 {
		return
	}
	rn := renamer(to)
	for _, s := range r.Schemas {
		s.Name = rn.name(s.Name)
		s.JSON = rn.json(s.JSON)
	}
	slices.SortFunc(r.Schemas, func(a, b *model.Schema) int { return strings.Compare(a.Name, b.Name) })
	for _, rt := range r.Routes {
		rn.route(rt)
	}
}

func (rn renamer) name(n string) string {
	if renamed, ok := rn[n]; ok {
		return renamed
	}
	return n
}

func (rn renamer) json(data []byte) []byte {
	return schemaRef.ReplaceAllFunc(data, func(m []byte) []byte {
		return []byte(`"$ref":"` + rn.name(string(schemaRef.FindSubmatch(m)[1])) + `"`)
	})
}

func (rn renamer) ref(ref *model.SchemaRef) {
	ref.Name = rn.name(ref.Name)
	if ref.Inline != nil {
		ref.Inline.JSON = rn.json(ref.Inline.JSON)
	}
}

func (rn renamer) route(rt *model.Route) {
	for i := range rt.Responses {
		rn.ref(&rt.Responses[i].Schema)
	}
	if rt.Request == nil {
		return
	}
	if rt.Request.Body != nil {
		rn.ref(rt.Request.Body)
	}
	for _, params := range [][]model.Param{rt.Request.Params, rt.Request.Query, rt.Request.Headers} {
		for i := range params {
			rn.ref(&params[i].Schema)
		}
	}
}
