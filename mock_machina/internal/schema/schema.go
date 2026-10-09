package schema

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const base = "https://schemas.mockmachina.invalid/"

var english = message.NewPrinter(language.English)

type Problem struct {
	Schema  string
	Pointer string
	Msg     string
}

type Set struct {
	compiler *jsonschema.Compiler
	compiled map[string]*jsonschema.Schema
	inline   map[string]*jsonschema.Schema
}

func URL(name string) string { return base + name }

func New(schemas []*model.Schema) (*Set, []Problem) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	s := &Set{compiler: c, compiled: map[string]*jsonschema.Schema{}, inline: map[string]*jsonschema.Schema{}}
	var probs []Problem
	for _, sc := range schemas {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(sc.JSON))
		if err != nil {
			probs = append(probs, Problem{Schema: sc.Name, Msg: err.Error()})
			continue
		}
		if err := c.AddResource(URL(sc.Name), resolveNames(doc)); err != nil {
			probs = append(probs, Problem{Schema: sc.Name, Msg: err.Error()})
		}
	}
	for _, sc := range schemas {
		compiled, err := c.Compile(URL(sc.Name))
		if err != nil {
			pointer, msg := compileProblem(err)
			probs = append(probs, Problem{Schema: sc.Name, Pointer: pointer, Msg: msg})
			continue
		}
		s.compiled[sc.Name] = compiled
	}
	return s, probs
}

func (s *Set) Schema(name string) (*jsonschema.Schema, bool) {
	sc, ok := s.compiled[name]
	return sc, ok
}

func resolveNames(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if ref, ok := child.(string); ok && k == "$ref" && !strings.ContainsAny(ref, "#:/") {
				x[k] = URL(ref)
				continue
			}
			x[k] = resolveNames(child)
		}
	case []any:
		for i, child := range x {
			x[i] = resolveNames(child)
		}
	}
	return v
}

func compileProblem(err error) (string, string) {
	var sve *jsonschema.SchemaValidationError
	if errors.As(err, &sve) {
		var ve *jsonschema.ValidationError
		if errors.As(sve.Err, &ve) {
			leaf := deepest(ve)
			return pointer(leaf.InstanceLocation), Message(leaf.ErrorKind)
		}
	}
	return "", fmt.Sprint(err)
}

func deepest(e *jsonschema.ValidationError) *jsonschema.ValidationError {
	for len(e.Causes) > 0 {
		e = e.Causes[0]
	}
	return e
}

func pointer(loc []string) string {
	if len(loc) == 0 {
		return ""
	}
	escaped := make([]string, len(loc))
	for i, p := range loc {
		escaped[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(escaped, "/")
}
