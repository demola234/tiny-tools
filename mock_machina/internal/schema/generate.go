package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const maxDepth = 6

type object struct {
	keys   []string
	values []any
}

func (o *object) get(key string) (any, bool) {
	i := slices.Index(o.keys, key)
	if i < 0 {
		return nil, false
	}
	return o.values[i], true
}

func (o *object) set(key string, v any) {
	if i := slices.Index(o.keys, key); i >= 0 {
		o.values[i] = v
		return
	}
	o.keys = append(o.keys, key)
	o.values = append(o.values, v)
}

type generator struct {
	named map[string]any
}

func Generate(schemas []*model.Schema, ref model.SchemaRef) ([]byte, error) {
	g := generator{named: map[string]any{}}
	for _, s := range schemas {
		doc, err := decodeOrdered(s.JSON)
		if err != nil {
			return nil, err
		}
		g.named[s.Name] = doc
	}
	var root any
	if ref.Inline != nil {
		doc, err := decodeOrdered(ref.Inline.JSON)
		if err != nil {
			return nil, err
		}
		root = doc
	} else {
		doc, ok := g.named[ref.Name]
		if !ok {
			return nil, fmt.Errorf("%q isn't a schema", ref.Name)
		}
		root = doc
	}
	var b bytes.Buffer
	encodeOrdered(&b, g.value(root, root, 0))
	return b.Bytes(), nil
}

func (g generator) value(s, doc any, depth int) any {
	o, ok := s.(*object)
	if !ok {
		return nil
	}
	if v, ok := fixed(o); ok {
		return v
	}
	if ref, ok := o.get("$ref"); ok {
		target, nextDoc := g.resolve(fmt.Sprint(ref), doc)
		return g.value(target, nextDoc, depth)
	}
	if all, ok := o.get("allOf"); ok {
		return g.merge(all, doc, depth)
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		if v, ok := first(o, key); ok {
			return g.value(v, doc, depth)
		}
	}
	return g.typed(o, doc, depth)
}

func fixed(o *object) (any, bool) {
	if v, ok := o.get("example"); ok {
		return v, true
	}
	if v, ok := first(o, "examples"); ok {
		return v, true
	}
	if v, ok := o.get("const"); ok {
		return v, true
	}
	return first(o, "enum")
}

func first(o *object, key string) (any, bool) {
	v, ok := o.get(key)
	if !ok {
		return nil, false
	}
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, false
	}
	return list[0], true
}

func (g generator) resolve(ref string, doc any) (any, any) {
	if !strings.HasPrefix(ref, "#") {
		target := g.named[ref]
		return target, target
	}
	cur := doc
	for part := range strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/") {
		o, ok := cur.(*object)
		if !ok {
			return nil, doc
		}
		cur, _ = o.get(strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~"))
	}
	return cur, doc
}

func (g generator) merge(all, doc any, depth int) any {
	out := &object{}
	list, _ := all.([]any)
	for _, s := range list {
		if part, ok := g.value(s, doc, depth).(*object); ok {
			for i, k := range part.keys {
				out.set(k, part.values[i])
			}
		}
	}
	return out
}

func (g generator) typed(o *object, doc any, depth int) any {
	switch firstType(o) {
	case "object":
		out := &object{}
		if depth >= maxDepth {
			return out
		}
		props, _ := o.get("properties")
		propObj, _ := props.(*object)
		required, _ := o.get("required")
		names, _ := required.([]any)
		for _, n := range names {
			name := fmt.Sprint(n)
			var prop any
			if propObj != nil {
				prop, _ = propObj.get(name)
			}
			out.set(name, g.value(prop, doc, depth+1))
		}
		return out
	case "array":
		n := max(1, intKeyword(o, "minItems", 1))
		if depth >= maxDepth-1 {
			n = intKeyword(o, "minItems", 0)
		}
		items, _ := o.get("items")
		out := make([]any, n)
		for i := range out {
			out[i] = g.value(items, doc, depth+1)
		}
		return out
	case "integer", "number":
		return number(o)
	case "boolean":
		return true
	case "null":
		return nil
	default:
		return text(o)
	}
}

func firstType(o *object) string {
	t, ok := o.get("type")
	if !ok {
		if _, ok := o.get("properties"); ok {
			return "object"
		}
		return "string"
	}
	if list, ok := t.([]any); ok {
		for _, v := range list {
			if s := fmt.Sprint(v); s != "null" {
				return s
			}
		}
		return "null"
	}
	return fmt.Sprint(t)
}

var formats = map[string]string{
	"email":     "user@example.com",
	"date-time": "2026-01-01T00:00:00Z",
	"date":      "2026-01-01",
	"time":      "00:00:00Z",
	"uuid":      "00000000-0000-4000-8000-000000000000",
	"uri":       "https://example.com",
	"url":       "https://example.com",
	"hostname":  "example.com",
	"ipv4":      "192.0.2.1",
	"ipv6":      "2001:db8::1",
}

func text(o *object) string {
	if f, ok := o.get("format"); ok {
		if v, ok := formats[fmt.Sprint(f)]; ok {
			return v
		}
	}
	if p, ok := o.get("pattern"); ok {
		return literalPrefix(fmt.Sprint(p)) + "1"
	}
	s := "string"
	if n := intKeyword(o, "minLength", 0); n > 0 {
		s = strings.Repeat("x", n)
	}
	if n := intKeyword(o, "maxLength", -1); n >= 0 && len(s) > n {
		s = s[:n]
	}
	return s
}

func literalPrefix(pattern string) string {
	p := strings.TrimPrefix(pattern, "^")
	end := strings.IndexAny(p, `.*+?()[]{}|\$`)
	if end < 0 {
		return p
	}
	return p[:end]
}

func number(o *object) json.Number {
	n := 1.0
	if v, ok := floatKeyword(o, "minimum"); ok {
		n = v
	}
	if v, ok := floatKeyword(o, "exclusiveMinimum"); ok {
		n = v + 1
	}
	if v, ok := floatKeyword(o, "maximum"); ok && n > v {
		n = v
	}
	return json.Number(strconv.FormatFloat(n, 'f', -1, 64))
}

func floatKeyword(o *object, key string) (float64, bool) {
	v, ok := o.get(key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
	return f, err == nil
}

func intKeyword(o *object, key string, fallback int) int {
	if f, ok := floatKeyword(o, key); ok {
		return int(f)
	}
	return fallback
}

func decodeOrdered(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('['):
		return decodeArray(dec)
	case json.Delim('{'):
		return decodeObject(dec)
	default:
		return tok, nil
	}
}

func decodeArray(dec *json.Decoder) (any, error) {
	var list []any
	for dec.More() {
		v, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	_, err := dec.Token()
	return list, err
}

func decodeObject(dec *json.Decoder) (any, error) {
	o := &object{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		v, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		o.set(fmt.Sprint(key), v)
	}
	_, err := dec.Token()
	return o, err
}

func encodeOrdered(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case *object:
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			key, _ := json.Marshal(k)
			b.Write(key)
			b.WriteByte(':')
			encodeOrdered(b, x.values[i])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			encodeOrdered(b, item)
		}
		b.WriteByte(']')
	default:
		data, _ := json.Marshal(x)
		b.Write(data)
	}
}
