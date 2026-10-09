package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const contractSchemas = "schemas/api.yaml"

type outFile struct {
	name string
	data []byte
}

func WriteContract(dir string, routes []*model.Route, schemas []*model.Schema) ([]string, error) {
	files, err := renderContract(routes, schemas)
	if err != nil {
		return nil, err
	}
	var exists []string
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.name))); err == nil {
			exists = append(exists, f.name)
		}
	}
	switch len(exists) {
	case 0:
	case 1:
		return nil, fmt.Errorf("%s already exists", exists[0])
	default:
		return nil, fmt.Errorf("%s already exist", strings.Join(exists, ", "))
	}
	var written []string
	for _, f := range files {
		full := filepath.Join(dir, filepath.FromSlash(f.name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return written, err
		}
		if err := WriteFileAtomic(full, f.data, fs.FileMode(0o644)); err != nil {
			return written, err
		}
		if !strings.HasSuffix(f.name, ".json") {
			written = append(written, f.name)
		}
	}
	return written, nil
}

func RenderContract(routes []*model.Route, schemas []*model.Schema) (map[string][]byte, error) {
	files, err := renderContract(routes, schemas)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(files))
	for _, f := range files {
		out[f.name] = f.data
	}
	return out, nil
}

func ContractFiles(routes []*model.Route, schemas []*model.Schema) ([]string, error) {
	files, err := renderContract(routes, schemas)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range files {
		if !strings.HasSuffix(f.name, ".json") {
			names = append(names, f.name)
		}
	}
	return names, nil
}

func renderContract(routes []*model.Route, schemas []*model.Schema) ([]outFile, error) {
	var order []string
	byGroup := map[string][]*model.Route{}
	for _, r := range routes {
		if _, ok := byGroup[r.Group]; !ok {
			order = append(order, r.Group)
		}
		byGroup[r.Group] = append(byGroup[r.Group], r)
	}
	var files []outFile
	for _, group := range order {
		groupFiles, err := renderGroup(group, byGroup[group])
		if err != nil {
			return nil, err
		}
		files = append(files, groupFiles...)
	}
	if len(schemas) == 0 {
		return files, nil
	}
	f, err := renderSchemas(schemas)
	if err != nil {
		return nil, err
	}
	return append(files, f), nil
}

func renderGroup(group string, routes []*model.Route) ([]outFile, error) {
	var b strings.Builder
	var files []outFile
	b.WriteString(schemaLine("route"))
	for i, r := range routes {
		if i > 0 {
			b.WriteString("\n")
		}
		text, bodies, err := renderRoute(r)
		if err != nil {
			return nil, fmt.Errorf("route %s: %w", r.ID, err)
		}
		b.WriteString(text)
		files = append(files, bodies...)
	}
	return append(files, outFile{name: path.Join(routesDir, group+resourceExt), data: []byte(b.String())}), nil
}

func renderSchemas(schemas []*model.Schema) (outFile, error) {
	doc := mapping()
	for _, s := range schemas {
		n, err := jsonToNode(s.JSON)
		if err != nil {
			return outFile{}, fmt.Errorf("schema %s: %w", s.Name, err)
		}
		doc.Content = append(doc.Content, str(s.Name), tidy(n))
	}
	text, err := encode(doc, defaultIndent)
	return outFile{name: contractSchemas, data: []byte(text)}, err
}

func renderRoute(r *model.Route) (string, []outFile, error) {
	name := strings.TrimPrefix(r.ID, r.Group+".")
	m := mapping()
	add := func(key string, v *yaml.Node) { m.Content = append(m.Content, str(key), v) }
	add("route", str(string(r.Method)+" "+r.Path))
	if r.Summary != "" {
		add("summary", str(r.Summary))
	}
	for _, e := range r.Extensions {
		n, err := extensionNode(e.Value)
		if err != nil {
			return "", nil, err
		}
		add(e.Key, n)
	}
	if r.Request != nil {
		n, err := requestNode(r.Request)
		if err != nil {
			return "", nil, err
		}
		add("request", n)
	}
	if len(r.Responses) > 0 {
		n, err := responsesNode(r.Responses)
		if err != nil {
			return "", nil, err
		}
		add("responses", n)
	}
	if len(r.States) > 0 && r.Active != "" && r.Active != r.States[0].Name {
		add("active", str(r.Active))
	}
	states, bodies, err := statesNode(r, name)
	if err != nil {
		return "", nil, err
	}
	add("states", states)
	doc := mapping()
	doc.Content = append(doc.Content, str(name), m)
	text, err := encode(doc, defaultIndent)
	return text, bodies, err
}

func responsesNode(responses []model.Response) (*yaml.Node, error) {
	resp := mapping()
	for _, res := range responses {
		n, err := refNode(res.Schema, false)
		if err != nil {
			return nil, err
		}
		key := str(res.Status)
		if _, err := strconv.Atoi(res.Status); err == nil {
			key = scalar("!!int", res.Status)
		}
		resp.Content = append(resp.Content, key, n)
	}
	return tidy(resp), nil
}

func statesNode(r *model.Route, routeName string) (*yaml.Node, []outFile, error) {
	states := mapping()
	var bodies []outFile
	for _, ns := range r.States {
		n, file, err := stateNode(r, routeName, ns)
		if err != nil {
			return nil, nil, err
		}
		if file != nil {
			bodies = append(bodies, *file)
		}
		states.Content = append(states.Content, str(ns.Name), n)
	}
	return states, bodies, nil
}

func requestNode(req *model.Request) (*yaml.Node, error) {
	n := mapping()
	for _, part := range []struct {
		key    string
		params []model.Param
	}{{"params", req.Params}, {"query", req.Query}, {"headers", req.Headers}} {
		if len(part.params) == 0 {
			continue
		}
		m := mapping()
		for _, p := range part.params {
			v, err := refNode(p.Schema, p.Required && part.key != "params")
			if err != nil {
				return nil, err
			}
			m.Content = append(m.Content, str(p.Name), v)
		}
		n.Content = append(n.Content, str(part.key), tidy(m))
	}
	if req.Body != nil {
		v, err := refNode(*req.Body, false)
		if err != nil {
			return nil, err
		}
		n.Content = append(n.Content, str("body"), v)
	}
	return n, nil
}

func refNode(ref model.SchemaRef, required bool) (*yaml.Node, error) {
	var n *yaml.Node
	if ref.Inline == nil {
		if !required {
			return str(ref.Name), nil
		}
		n = mapping()
		n.Content = append(n.Content, str("$ref"), str(ref.Name))
	} else {
		var err error
		if n, err = jsonToNode(ref.Inline.JSON); err != nil {
			return nil, err
		}
	}
	if required && n.Kind == yaml.MappingNode {
		n.Content = append(n.Content, str("required"), scalar("!!bool", "true"))
	}
	return tidy(n), nil
}

func stateNode(r *model.Route, routeName string, ns model.NamedState) (*yaml.Node, *outFile, error) {
	st := ns.State
	n := mapping()
	if st.Status != 0 && st.Status != 200 {
		n.Content = append(n.Content, str("status"), scalar("!!int", strconv.Itoa(st.Status)))
	}
	var file *outFile
	switch {
	case st.Body.Generate:
		n.Content = append(n.Content, str("body"), str("generate"))
	case len(st.Body.Data) > 0 && !strings.HasPrefix(st.Body.ContentType, jsonType):
		name := routeName + "." + ns.Name + extensionFor(st.Body.ContentType)
		file = &outFile{name: path.Join(routesDir, r.Group, name), data: st.Body.Data}
		n.Content = append(n.Content, str("body"), str(name))
	case len(st.Body.Data) > maxInlineBody:
		name := routeName + "." + ns.Name + ".json"
		file = &outFile{name: path.Join(routesDir, r.Group, name), data: indentJSON(st.Body.Data)}
		n.Content = append(n.Content, str("body"), str(name))
	case len(st.Body.Data) > 0:
		body, err := jsonToNode(st.Body.Data)
		if err != nil {
			return nil, nil, err
		}
		if body.Kind == yaml.ScalarNode {
			return nil, nil, errors.New("a body must be a JSON object or list")
		}
		n.Content = append(n.Content, str("body"), tidy(body))
	}
	if len(n.Content) == 0 {
		n.Style = yaml.FlowStyle
		return n, nil, nil
	}
	return tidy(n), file, nil
}

func extensionNode(v any) (*yaml.Node, error) {
	if data, ok := v.([]byte); ok {
		return jsonToNode(data)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jsonToNode(data)
}

func jsonToNode(data []byte) (*yaml.Node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return jsonNode(dec)
}

func mapping() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }

func str(s string) *yaml.Node { return scalar("!!str", s) }

func tidy(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.ScalarNode {
		return n
	}
	setFlow(n, true)
	if text, err := encode(n, defaultIndent); err == nil && len(strings.TrimSuffix(text, "\n")) <= maxFlowBody && !strings.Contains(strings.TrimSuffix(text, "\n"), "\n") {
		return n
	}
	n.Style = 0
	if len(n.Content) == 0 {
		n.Style = yaml.FlowStyle
	}
	for _, c := range n.Content {
		tidy(c)
	}
	return n
}

func extensionFor(contentType string) string {
	for ext, t := range contentTypes {
		if t == contentType {
			return ext
		}
	}
	return ".txt"
}
