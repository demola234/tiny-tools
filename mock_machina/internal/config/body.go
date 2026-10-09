package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

const jsonType = "application/json"

var contentTypes = map[string]string{
	".json": jsonType,
	".txt":  "text/plain; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".xml":  "application/xml",
}

func contentType(file string) string {
	if t, ok := contentTypes[strings.ToLower(path.Ext(file))]; ok {
		return t
	}
	return "application/octet-stream"
}

func toJSON(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	err := writeJSON(&buf, node)
	return buf.Bytes(), err
}

func writeJSON(buf *bytes.Buffer, node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		return writeObject(buf, node)
	case yaml.SequenceNode:
		return writeArray(buf, node)
	case yaml.AliasNode:
		return writeJSON(buf, node.Alias)
	default:
		return writeScalar(buf, node)
	}
}

func writeObject(buf *bytes.Buffer, node *yaml.Node) error {
	buf.WriteByte('{')
	for i := 0; i+1 < len(node.Content); i += 2 {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(node.Content[i].Value)
		buf.Write(key)
		buf.WriteByte(':')
		if err := writeJSON(buf, node.Content[i+1]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func writeArray(buf *bytes.Buffer, node *yaml.Node) error {
	buf.WriteByte('[')
	for i, item := range node.Content {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeJSON(buf, item); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

func writeScalar(buf *bytes.Buffer, node *yaml.Node) error {
	var v any
	if err := node.Decode(&v); err != nil {
		return err
	}
	if f, ok := v.(float64); ok && (math.IsInf(f, 0) || math.IsNaN(f)) {
		return errNotJSONNumber
	}
	b, _ := json.Marshal(v)
	buf.Write(b)
	return nil
}

var errNotJSONNumber = errors.New("infinity and NaN aren't JSON numbers")
