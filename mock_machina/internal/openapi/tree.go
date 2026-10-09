package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

type omap struct {
	keys []string
	vals []any
}

func (m *omap) get(key string) any {
	if m == nil {
		return nil
	}
	for i, k := range m.keys {
		if k == key {
			return m.vals[i]
		}
	}
	return nil
}

func (m *omap) obj(key string) *omap {
	o, _ := m.get(key).(*omap)
	return o
}

func (m *omap) str(key string) string {
	switch v := m.get(key).(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return ""
}

func (m *omap) set(key string, v any) {
	for i, k := range m.keys {
		if k == key {
			m.vals[i] = v
			return
		}
	}
	m.keys = append(m.keys, key)
	m.vals = append(m.vals, v)
}

func (m *omap) del(key string) {
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			m.vals = append(m.vals[:i], m.vals[i+1:]...)
			return
		}
	}
}

var errNotYAML = errors.New("isn't valid YAML or JSON")

func parse(data []byte) (any, error) {
	var n yaml.Node
	if err := yaml.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("%w: %w", errNotYAML, err)
	}
	if len(n.Content) == 0 {
		return nil, errNotYAML
	}
	return value(n.Content[0]), nil
}

func value(n *yaml.Node) any {
	switch n.Kind {
	case yaml.DocumentNode:
		return value(n.Content[0])
	case yaml.AliasNode:
		return value(n.Alias)
	case yaml.MappingNode:
		m := &omap{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			m.set(n.Content[i].Value, value(n.Content[i+1]))
		}
		return m
	case yaml.SequenceNode:
		list := make([]any, len(n.Content))
		for i, c := range n.Content {
			list[i] = value(c)
		}
		return list
	case yaml.ScalarNode:
	}
	switch n.Tag {
	case "!!int", "!!float":
		text := strings.ReplaceAll(n.Value, "_", "")
		if _, err := strconv.ParseFloat(text, 64); err == nil {
			return json.Number(text)
		}
	case "!!bool":
		b, err := strconv.ParseBool(strings.ToLower(n.Value))
		if err == nil {
			return b
		}
	case "!!null":
		return nil
	}
	return n.Value
}

func clone(v any) any {
	switch x := v.(type) {
	case *omap:
		m := &omap{keys: append([]string(nil), x.keys...), vals: make([]any, len(x.vals))}
		for i, c := range x.vals {
			m.vals[i] = clone(c)
		}
		return m
	case []any:
		list := make([]any, len(x))
		for i, c := range x {
			list[i] = clone(c)
		}
		return list
	}
	return v
}

func encode(v any) []byte {
	var b bytes.Buffer
	write(&b, v)
	return b.Bytes()
}

func write(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case *omap:
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			write(b, x.vals[i])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, c := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			write(b, c)
		}
		b.WriteByte(']')
	case string:
		writeString(b, x)
	case json.Number:
		b.WriteString(x.String())
	case bool:
		b.WriteString(strconv.FormatBool(x))
	default:
		b.WriteString("null")
	}
}

func writeString(b *bytes.Buffer, s string) {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	b.Truncate(b.Len() - 1)
}
