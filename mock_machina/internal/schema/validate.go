package schema

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const maxViolations = 3

func (s *Set) Inline(key string, data []byte) (*jsonschema.Schema, error) {
	if sc, ok := s.inline[key]; ok {
		return sc, nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	loc := base + "inline/" + url.PathEscape(key)
	if err := s.compiler.AddResource(loc, resolveNames(doc)); err != nil {
		return nil, err
	}
	sc, err := s.compiler.Compile(loc)
	if err != nil {
		_, msg := compileProblem(err)
		return nil, errors.New(msg)
	}
	s.inline[key] = sc
	return sc, nil
}

func (*Set) Validate(sc *jsonschema.Schema, body []byte) []string {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return []string{"body: isn't valid JSON"}
	}
	return Check(sc, inst)
}

func Check(sc *jsonschema.Schema, instance any) []string {
	var ve *jsonschema.ValidationError
	if !errors.As(sc.Validate(instance), &ve) {
		return nil
	}
	var out []string
	for _, leaf := range leaves(ve) {
		out = append(out, Where(leaf.InstanceLocation)+": "+Message(leaf.ErrorKind))
	}
	slices.Sort(out)
	out = slices.Compact(out)
	return out[:min(len(out), maxViolations)]
}

func leaves(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}
	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

func Where(loc []string) string {
	if len(loc) == 0 {
		return "body"
	}
	var b strings.Builder
	for _, part := range loc {
		if _, err := strconv.Atoi(part); err == nil {
			b.WriteString("[" + part + "]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(part)
	}
	return b.String()
}

func Message(k jsonschema.ErrorKind) string {
	switch k := k.(type) {
	case *kind.Type:
		want := make([]string, len(k.Want))
		for i, w := range k.Want {
			want[i] = article(w)
		}
		return "should be " + strings.Join(want, " or ") + ", but is " + article(k.Got)
	case *kind.Required:
		return "missing required " + noun(len(k.Missing), "field", "fields") + " " + quoteAll(k.Missing)
	case *kind.AdditionalProperties:
		return "has " + noun(len(k.Properties), "a field", "fields") + " the schema doesn't allow: " + quoteAll(k.Properties)
	case *kind.Format:
		return value(k.Got) + " isn't a valid " + k.Want
	case *kind.Pattern:
		return value(k.Got) + " doesn't match the pattern " + k.Want
	case *kind.Enum:
		want := make([]string, len(k.Want))
		for i, w := range k.Want {
			want[i] = value(w)
		}
		return value(k.Got) + " isn't one of: " + strings.Join(want, ", ")
	case *kind.Const:
		return "should be " + value(k.Want)
	default:
		return bounds(k)
	}
}

func bounds(k jsonschema.ErrorKind) string {
	switch k := k.(type) {
	case *kind.MinLength:
		return fmt.Sprintf("has %s; the schema wants at least %d", count(k.Got, "character"), k.Want)
	case *kind.MaxLength:
		return fmt.Sprintf("has %s; the schema wants at most %d", count(k.Got, "character"), k.Want)
	case *kind.Minimum:
		return rat(k.Got) + " is less than the minimum, " + rat(k.Want)
	case *kind.Maximum:
		return rat(k.Got) + " is more than the maximum, " + rat(k.Want)
	case *kind.ExclusiveMinimum:
		return rat(k.Got) + " must be more than " + rat(k.Want)
	case *kind.ExclusiveMaximum:
		return rat(k.Got) + " must be less than " + rat(k.Want)
	case *kind.MinItems:
		return fmt.Sprintf("has %d items; the schema wants at least %d", k.Got, k.Want)
	case *kind.MaxItems:
		return fmt.Sprintf("has %d items; the schema wants at most %d", k.Got, k.Want)
	case *kind.FalseSchema:
		return "isn't allowed here"
	}
	return k.LocalizedString(english)
}

func article(t string) string {
	switch t {
	case "integer", "object", "array":
		return "an " + t
	case "null":
		return "null"
	}
	return "a " + t
}

func noun(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func count(n int, word string) string {
	return strconv.Itoa(n) + " " + noun(n, word, word+"s")
}

func quoteAll(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = strconv.Quote(n)
	}
	return strings.Join(q, ", ")
}

func value(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

func rat(r *big.Rat) string {
	if r.IsInt() {
		return r.RatString()
	}
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func (s *Set) Ref(ref model.SchemaRef) (*jsonschema.Schema, string, error) {
	if ref.Inline == nil {
		sc, ok := s.Schema(ref.Name)
		if !ok {
			return nil, ref.Name, fmt.Errorf("%q isn't a schema", ref.Name)
		}
		return sc, ref.Name, nil
	}
	sc, err := s.Inline(fmt.Sprintf("%s:%d:%s:%x", ref.Inline.Src.File, ref.Inline.Src.Line, ref.Inline.Name, sha256.Sum256(ref.Inline.JSON)), ref.Inline.JSON)
	return sc, "its " + ref.Inline.Name + " schema", err
}
