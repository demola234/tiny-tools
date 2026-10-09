package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/schema"
)

type requestCheck struct {
	params  []paramCheck
	query   []paramCheck
	headers []paramCheck
	body    *jsonschema.Schema
	set     *schema.Set
}

type paramCheck struct {
	name     string
	required bool
	schema   *jsonschema.Schema
	types    []string
}

type requestProblem struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type invalidRequest struct {
	Error    string           `json:"error"`
	Route    string           `json:"route"`
	Problems []requestProblem `json:"problems"`
}

func compileRequest(set *schema.Set, schemas []*model.Schema, req *model.Request) (*requestCheck, error) {
	if req == nil {
		return nil, nil
	}
	c := &requestCheck{set: set}
	var err error
	if c.params, err = compileParams(set, schemas, req.Params); err != nil {
		return nil, err
	}
	if c.query, err = compileParams(set, schemas, req.Query); err != nil {
		return nil, err
	}
	if c.headers, err = compileParams(set, schemas, req.Headers); err != nil {
		return nil, err
	}
	if req.Body != nil {
		if c.body, _, err = set.Ref(*req.Body); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func compileParams(set *schema.Set, schemas []*model.Schema, params []model.Param) ([]paramCheck, error) {
	out := make([]paramCheck, 0, len(params))
	for _, p := range params {
		sc, _, err := set.Ref(p.Schema)
		if err != nil {
			return nil, err
		}
		out = append(out, paramCheck{name: p.Name, required: p.Required, schema: sc, types: declaredTypes(schemas, p.Schema)})
	}
	return out, nil
}

func declaredTypes(schemas []*model.Schema, ref model.SchemaRef) []string {
	data := []byte(nil)
	if ref.Inline != nil {
		data = ref.Inline.JSON
	} else if i := slices.IndexFunc(schemas, func(s *model.Schema) bool { return s.Name == ref.Name }); i >= 0 {
		data = schemas[i].JSON
	}
	var doc struct {
		Type any `json:"type"`
	}
	_ = json.Unmarshal(data, &doc)
	switch t := doc.Type.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			s, _ := v.(string)
			out = append(out, s)
		}
		return out
	}
	return nil
}

func (c *requestCheck) check(req *http.Request, body func() []byte) []requestProblem {
	var out []requestProblem
	query := req.URL.Query()
	for _, p := range c.params {
		out = append(out, p.check("params", textValues(req.PathValue(p.name)))...)
	}
	for _, p := range c.query {
		if p.name != stateQuery {
			out = append(out, p.check("query", query[p.name])...)
		}
	}
	for _, p := range c.headers {
		out = append(out, p.check("headers", req.Header.Values(p.name))...)
	}
	if c.body != nil {
		data := body()
		if len(strings.TrimSpace(string(data))) == 0 {
			return append(out, requestProblem{At: "body", Message: "is required"})
		}
		for _, v := range c.set.Validate(c.body, data) {
			out = append(out, split("body", v))
		}
	}
	return out
}

func textValues(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

func (p paramCheck) check(kind string, values []string) []requestProblem {
	at := kind + "." + p.name
	if len(values) == 0 {
		if p.required {
			return []requestProblem{{At: at, Message: "is required"}}
		}
		return nil
	}
	instance := coerce(values[0], p.types)
	if slices.Contains(p.types, "array") {
		items := make([]any, len(values))
		for i, v := range values {
			items[i] = v
		}
		instance = items
	}
	var out []requestProblem
	for _, v := range schema.Check(p.schema, instance) {
		out = append(out, split(at, v))
	}
	return out
}

func coerce(text string, types []string) any {
	if slices.Contains(types, "integer") || slices.Contains(types, "number") {
		if _, err := strconv.ParseFloat(text, 64); err == nil {
			return json.Number(text)
		}
	}
	if slices.Contains(types, "boolean") && (text == "true" || text == "false") {
		return text == "true"
	}
	return text
}

func split(at, violation string) requestProblem {
	where, msg, _ := strings.Cut(violation, ": ")
	switch {
	case where == "body":
	case strings.HasPrefix(where, "["):
		at += where
	default:
		at += "." + where
	}
	return requestProblem{At: at, Message: msg}
}
