package config

import (
	"errors"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/schema"
)

const schemasDir = "schemas"

var (
	schemaName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

	schemaKeywords = []string{
		"$schema", "$id", "$ref", "$anchor", "$dynamicRef", "$dynamicAnchor", "$vocabulary", "$comment", "$defs",
		"type", "enum", "const", "multipleOf", "maximum", "exclusiveMaximum", "minimum", "exclusiveMinimum",
		"maxLength", "minLength", "pattern", "maxItems", "minItems", "uniqueItems", "maxContains", "minContains",
		"maxProperties", "minProperties", "required", "dependentRequired",
		"allOf", "anyOf", "oneOf", "not", "if", "then", "else", "dependentSchemas", "prefixItems", "items",
		"contains", "properties", "patternProperties", "additionalProperties", "propertyNames",
		"unevaluatedItems", "unevaluatedProperties", "format", "contentEncoding", "contentMediaType", "contentSchema",
		"title", "description", "default", "deprecated", "readOnly", "writeOnly", "examples",
		"example", "discriminator", "xml", "externalDocs",
	}
	namedSchemas  = []string{"properties", "patternProperties", "$defs", "dependentSchemas"}
	schemaValues  = []string{"items", "additionalProperties", "not", "if", "then", "else", "contains", "propertyNames", "unevaluatedItems", "unevaluatedProperties", "contentSchema"}
	schemaLists   = []string{"allOf", "anyOf", "oneOf", "prefixItems"}
	jsonTypes     = []string{"string", "number", "integer", "boolean", "object", "array", "null"}
	pointerEscape = strings.NewReplacer("~", "~0", "/", "~1")
)

type schemaRef struct {
	file string
	line int
	name string
}

type schemaWalk struct {
	l     *loader
	name  string
	file  string
	lines map[string]int
	refs  *[]schemaRef
}

func (l *loader) schemas() []*model.Schema {
	entries, err := fs.ReadDir(l.fsys, schemasDir)
	if errors.Is(err, fs.ErrNotExist) {
		l.schemasOK = true
		return nil
	}
	if err != nil {
		l.add(schemasDir, 0, "can't read this folder: %v", err)
		return nil
	}
	var out []*model.Schema
	var refs []schemaRef
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || path.Ext(e.Name()) != resourceExt {
			continue
		}
		out = append(out, l.schemaFile(path.Join(schemasDir, e.Name()), out, &refs)...)
	}
	slices.SortFunc(out, func(a, b *model.Schema) int { return strings.Compare(a.Name, b.Name) })
	names := make([]string, len(out))
	for i, s := range out {
		names[i] = s.Name
	}
	for _, r := range refs {
		if !slices.Contains(names, r.name) {
			l.add(r.file, r.line, "$ref %q isn't a schema (%s)", r.name, closeMatchOr(r.name, names, "schemas: "+strings.Join(names, ", ")))
		}
	}
	l.schemaNames = names
	if !l.probs.HasErrors() {
		l.schemasOK = l.compileSchemas(out)
	}
	return out
}

func (l *loader) schemaFile(file string, known []*model.Schema, refs *[]schemaRef) []*model.Schema {
	root := l.parse(file)
	if root == nil {
		return nil
	}
	if root.Kind != yaml.MappingNode {
		l.add(file, root.Line, "a schemas file maps names to schemas, like User: { type: object }")
		return nil
	}
	var out []*model.Schema
	for key, val := range pairs(root) {
		name := key.Value
		switch {
		case !schemaName.MatchString(name):
			l.add(file, key.Line, "schema name %q must use letters, digits, dots, dashes and underscores", name)
			continue
		case val.Kind != yaml.MappingNode:
			l.add(file, key.Line, "schema %q must be a mapping, like { type: object }", name)
			continue
		}
		if i := slices.IndexFunc(slices.Concat(known, out), func(s *model.Schema) bool { return s.Name == name }); i >= 0 {
			first := slices.Concat(known, out)[i]
			l.add(file, key.Line, "schema %q is already defined in %s:%d", name, first.Src.File, first.Src.Line)
			continue
		}
		w := schemaWalk{l: l, name: name, file: file, lines: map[string]int{}, refs: refs}
		w.node(val, "")
		data, err := toJSON(val)
		if err != nil {
			l.add(file, key.Line, "schema %q can't be written as JSON: %v", name, err)
			continue
		}
		out = append(out, &model.Schema{Name: name, JSON: data, Lines: w.lines, Src: model.Source{File: file, Line: key.Line}})
	}
	return out
}

func (w schemaWalk) node(n *yaml.Node, ptr string) {
	w.lines[ptr] = n.Line
	if n.Kind != yaml.MappingNode {
		return
	}
	for key, val := range pairs(n) {
		k := key.Value
		p := ptr + "/" + pointerEscape.Replace(k)
		w.lines[p] = key.Line
		switch {
		case strings.HasPrefix(k, "x-"):
		case !slices.Contains(schemaKeywords, k):
			w.l.add(w.file, key.Line, "unknown schema keyword %q (%s)", k, closeMatchOr(k, schemaKeywords, "see https://json-schema.org/understanding-json-schema"))
		case k == "$ref":
			if !strings.ContainsAny(val.Value, "#:/") {
				*w.refs = append(*w.refs, schemaRef{file: w.file, line: key.Line, name: val.Value})
			}
		case k == "type":
			w.types(val, p)
		case slices.Contains(namedSchemas, k) && val.Kind == yaml.MappingNode:
			for name, child := range pairs(val) {
				w.lines[p+"/"+pointerEscape.Replace(name.Value)] = name.Line
				w.node(child, p+"/"+pointerEscape.Replace(name.Value))
			}
		case slices.Contains(schemaValues, k):
			w.node(val, p)
		case slices.Contains(schemaLists, k) && val.Kind == yaml.SequenceNode:
			for i, child := range val.Content {
				w.node(child, p+"/"+strconv.Itoa(i))
			}
		}
	}
}

func (w schemaWalk) types(v *yaml.Node, ptr string) {
	values := []*yaml.Node{v}
	if v.Kind == yaml.SequenceNode {
		values = v.Content
	}
	for _, t := range values {
		if !slices.Contains(jsonTypes, t.Value) {
			w.l.add(w.file, t.Line, "%s isn't a valid schema at %s: %q isn't a type (%s)", w.name, ptr, t.Value,
				closeMatchOr(t.Value, jsonTypes, "types: "+strings.Join(jsonTypes, ", ")))
		}
	}
}

func (l *loader) compileSchemas(schemas []*model.Schema) bool {
	_, probs := schema.New(schemas)
	for _, p := range probs {
		i := slices.IndexFunc(schemas, func(s *model.Schema) bool { return s.Name == p.Schema })
		s := schemas[i]
		line, ok := s.Lines[p.Pointer]
		if !ok {
			line = s.Src.Line
		}
		at := ""
		if p.Pointer != "" {
			at = " at " + p.Pointer
		}
		l.add(s.Src.File, line, "%s isn't a valid schema%s: %s", s.Name, at, p.Msg)
	}
	return len(probs) == 0
}
