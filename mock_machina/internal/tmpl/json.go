package tmpl

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

type JSON struct {
	root node
}

type node struct {
	kind     kind
	keys     []string
	children []node
	raw      string
	text     Text
}

type kind int

const (
	literal kind = iota
	str
	templated
	object
	array
)

func ParseJSON(data []byte) (JSON, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	root, err := parseNode(dec)
	if err != nil {
		return JSON{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return JSON{}, errors.New("body has more than one JSON value")
	}
	return JSON{root: root}, nil
}

func parseNode(dec *json.Decoder) (node, error) {
	tok, err := dec.Token()
	if err != nil {
		return node{}, err
	}
	switch v := tok.(type) {
	case json.Delim:
		return parseContainer(dec, v)
	case string:
		if !Has([]byte(v)) {
			return node{kind: str, raw: v}, nil
		}
		t, err := Parse(v)
		return node{kind: templated, text: t}, err
	case json.Number:
		return node{kind: literal, raw: v.String()}, nil
	case bool:
		return node{kind: literal, raw: strconv.FormatBool(v)}, nil
	default:
		return node{kind: literal, raw: "null"}, nil
	}
}

func parseContainer(dec *json.Decoder, open json.Delim) (node, error) {
	n := node{kind: array}
	if open == '{' {
		n.kind = object
	}
	for dec.More() {
		if n.kind == object {
			key, err := dec.Token()
			if err != nil {
				return node{}, err
			}
			n.keys = append(n.keys, key.(string))
		}
		child, err := parseNode(dec)
		if err != nil {
			return node{}, err
		}
		n.children = append(n.children, child)
	}
	_, err := dec.Token()
	return n, err
}

func (j JSON) Render(env Env) []byte {
	var b bytes.Buffer
	j.root.write(&b, env)
	return b.Bytes()
}

func (n node) write(b *bytes.Buffer, env Env) {
	switch n.kind {
	case literal:
		b.WriteString(n.raw)
	case str:
		b.Write(marshal(n.raw))
	case templated:
		b.Write(marshal(n.text.Eval(env)))
	case object, array:
		opening, closing := byte('['), byte(']')
		if n.kind == object {
			opening, closing = '{', '}'
		}
		b.WriteByte(opening)
		for i, child := range n.children {
			if i > 0 {
				b.WriteByte(',')
			}
			if n.kind == object {
				b.Write(marshal(n.keys[i]))
				b.WriteByte(':')
			}
			child.write(b, env)
		}
		b.WriteByte(closing)
	}
}

func (j JSON) UsesBody() bool { return j.root.any(Text.UsesBody) }

func (j JSON) UsesRandom() bool { return j.root.any(Text.UsesRandom) }

func (n node) any(check func(Text) bool) bool {
	if n.kind == templated {
		return check(n.text)
	}
	for _, c := range n.children {
		if c.any(check) {
			return true
		}
	}
	return false
}
