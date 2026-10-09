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
	"slices"
	"strconv"
	"strings"
	"testing/fstest"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	maxInlineBody = 4 << 10
	maxFlowBody   = 80
	defaultIndent = 2
)

type NewState struct {
	Name      string
	Status    int
	Headers   map[string]string
	Latency   time.Duration
	Generated bool
	Body      json.RawMessage
}

type AddedState struct {
	File     string
	Line     int
	BodyFile string
}

func AddState(dir, routeID string, st NewState) (AddedState, error) {
	if _, err := newStateBody(st); err != nil {
		return AddedState{}, err
	}
	proj, probs, err := Load(dir)
	if err != nil {
		return AddedState{}, fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	r, err := FindRoute(proj, routeID)
	if err != nil {
		return AddedState{}, err
	}
	file := r.Src.File
	if own := problemsIn(probs, file); len(own) > 0 {
		return AddedState{}, fmt.Errorf("%s has problems; fix them first:\n  %s", file, indentLines(own))
	}
	if _, ok := r.States.Get(st.Name); ok {
		return AddedState{}, fmt.Errorf("route %s already has a state %q", routeID, st.Name)
	}

	full := filepath.Join(dir, filepath.FromSlash(file))
	data, err := os.ReadFile(full)
	if err != nil {
		return AddedState{}, err
	}
	routeName := strings.TrimPrefix(r.ID, r.Group+".")
	ins, err := newInsertion(st, routeName)
	if err != nil {
		return AddedState{}, err
	}
	updated, line, err := ins.into(data, routeName)
	if err != nil {
		return AddedState{}, fmt.Errorf("route %s %w", routeID, err)
	}

	added := AddedState{File: file, Line: line}
	overlay := fstest.MapFS{file: {Data: updated}}
	if ins.bodyFile != "" {
		added.BodyFile = path.Join(r.Dir, ins.bodyFile)
		overlay[added.BodyFile] = &fstest.MapFile{Data: ins.bodyData}
	}
	if err := checkNew("state", file, overlayFS{FS: os.DirFS(dir), files: overlay}); err != nil {
		return AddedState{}, err
	}
	bodies := map[string][]byte{}
	if ins.bodyFile != "" {
		bodies[ins.bodyFile] = ins.bodyData
	}
	return added, writeWithBodies(full, updated, filepath.Join(dir, filepath.FromSlash(r.Dir)), bodies)
}

func newInsertion(st NewState, routeName string) (insertion, error) {
	body, err := newStateBody(st)
	if err != nil {
		return insertion{}, err
	}
	ins := insertion{state: st, body: body}
	if body != nil && len(st.Body) > maxInlineBody {
		ins.bodyFile = routeName + "." + st.Name + ".json"
		ins.bodyData = indentJSON(st.Body)
	}
	return ins, nil
}

func newStateBody(st NewState) (*yaml.Node, error) {
	switch {
	case model.IsYAMLKeyword(st.Name):
		return nil, fmt.Errorf("state %q is a YAML keyword; pick another name", st.Name)
	case !model.ValidStateName(st.Name):
		return nil, fmt.Errorf("state %q must use lowercase letters, digits and underscores, starting with a letter", st.Name)
	case len(st.Body) == 0:
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(st.Body, &v); err != nil {
		return nil, fmt.Errorf("body isn't valid JSON: %w", err)
	}
	switch v.(type) {
	case nil:
		return nil, nil
	case map[string]any, []any:
		dec := json.NewDecoder(bytes.NewReader(st.Body))
		dec.UseNumber()
		return jsonNode(dec)
	default:
		return nil, errors.New("body must be a JSON object or array")
	}
}

func jsonNode(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if v == '[' {
			n.Kind, n.Tag = yaml.SequenceNode, "!!seq"
		}
		for dec.More() {
			if n.Kind == yaml.MappingNode {
				key, _ := dec.Token()
				n.Content = append(n.Content, scalar("!!str", key.(string)))
			}
			child, err := jsonNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		_, err := dec.Token()
		return n, err
	case json.Number:
		if strings.ContainsAny(v.String(), ".eE") {
			return scalar("!!float", v.String()), nil
		}
		return scalar("!!int", v.String()), nil
	case string:
		return scalar("!!str", v), nil
	case bool:
		return scalar("!!bool", strconv.FormatBool(v)), nil
	default:
		return scalar("!!null", "null"), nil
	}
}

func scalar(tag, value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
}

func indentJSON(raw json.RawMessage) []byte {
	var b bytes.Buffer
	_ = json.Indent(&b, raw, "", "  ")
	b.WriteByte('\n')
	return b.Bytes()
}

type insertion struct {
	state    NewState
	body     *yaml.Node
	bodyFile string
	bodyData []byte
}

var errOneLineStates = errors.New("has its states on one line; add the state by hand")

func (ins insertion) into(data []byte, routeName string) ([]byte, int, error) {
	states, err := statesOf(data, routeName)
	if err != nil {
		return nil, 0, err
	}
	lastKey, lastVal := states.Content[len(states.Content)-2], states.Content[len(states.Content)-1]
	indent := lastKey.Column - 1
	child := defaultIndent
	if lastVal.Kind == yaml.MappingNode && lastVal.Style&yaml.FlowStyle == 0 && len(lastVal.Content) > 0 {
		child = lastVal.Content[0].Column - lastKey.Column
	}
	block, err := ins.render(indent, child, eolOf(data))
	if err != nil {
		return nil, 0, err
	}
	at := blockEnd(data, lastKey.Line, indent)
	var lead []byte
	if at == len(data) && at > 0 && data[at-1] != '\n' {
		lead = []byte(eolOf(data))
	}
	line := bytes.Count(data[:at], []byte("\n")) + 1
	if lead != nil {
		line++
	}
	return slices.Concat(data[:at], lead, block, data[at:]), line, nil
}

func statesOf(data []byte, routeName string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var route *yaml.Node
	for key, val := range pairs(doc.Content[0]) {
		if key.Value == routeName {
			route = val
		}
	}
	var states *yaml.Node
	for key, val := range pairs(route) {
		if key.Value == "states" {
			states = val
		}
	}
	if route.Style&yaml.FlowStyle != 0 || states.Style&yaml.FlowStyle != 0 {
		return nil, errOneLineStates
	}
	return states, nil
}

func eolOf(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func blockEnd(data []byte, keyLine, indent int) int {
	end := lineEnd(data, lineOffset(data, keyLine))
	for pos := end; pos < len(data); {
		next := lineEnd(data, pos)
		line := data[pos:next]
		trimmed := bytes.TrimLeft(line, " ")
		if len(bytes.TrimRight(trimmed, "\r\n")) > 0 {
			if len(line)-len(trimmed) <= indent {
				break
			}
			end = next
		}
		pos = next
	}
	return end
}

func lineEnd(data []byte, start int) int {
	if i := bytes.IndexByte(data[start:], '\n'); i >= 0 {
		return start + i + 1
	}
	return len(data)
}

func (ins insertion) render(indent, child int, eol string) ([]byte, error) {
	fields := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key string, val *yaml.Node) {
		fields.Content = append(fields.Content, scalar("!!str", key), val)
	}
	st := ins.state
	if st.Status != 0 {
		add("status", scalar("!!int", strconv.Itoa(st.Status)))
	}
	if len(st.Headers) > 0 {
		add("headers", headersNode(st.Headers))
	}
	if st.Latency != 0 {
		add("latency", scalar("!!str", st.Latency.String()))
	}
	if st.Generated {
		add("generated", scalar("!!bool", "true"))
	}
	switch {
	case ins.bodyFile != "":
		add("body", scalar("!!str", ins.bodyFile))
	case ins.body != nil:
		body, err := bodyNode(ins.body)
		if err != nil {
			return nil, err
		}
		add("body", body)
	}
	if len(fields.Content) == 0 {
		fields.Style = yaml.FlowStyle
	}
	doc := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("!!str", st.Name), fields}}
	text, err := encode(doc, child)
	if err != nil {
		return nil, err
	}
	pad := strings.Repeat(" ", indent)
	lines := strings.SplitAfter(strings.TrimSuffix(text, "\n"), "\n")
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(pad + strings.TrimSuffix(l, "\n") + eol)
	}
	return []byte(b.String()), nil
}

func headersNode(headers map[string]string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		n.Content = append(n.Content, scalar("!!str", k), scalar("!!str", headers[k]))
	}
	return n
}

func bodyNode(body *yaml.Node) (*yaml.Node, error) {
	setFlow(body, true)
	text, err := encode(body, defaultIndent)
	if err != nil {
		return nil, err
	}
	if line := strings.TrimSuffix(text, "\n"); len(line) > maxFlowBody || strings.Contains(line, "\n") {
		setFlow(body, false)
	}
	return body, nil
}

func setFlow(n *yaml.Node, flow bool) {
	if n.Kind == yaml.ScalarNode {
		return
	}
	n.Style = 0
	if flow || len(n.Content) == 0 {
		n.Style = yaml.FlowStyle
	}
	for _, c := range n.Content {
		setFlow(c, flow)
	}
}

func encode(n *yaml.Node, indent int) (string, error) {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(indent)
	if err := enc.Encode(n); err != nil {
		return "", err
	}
	return b.String(), enc.Close()
}

func checkNew(what, file string, fsys fs.FS) error {
	_, probs, err := LoadFS(fsys)
	if err != nil {
		return err
	}
	if own := problemsIn(probs, file); len(own) > 0 {
		return fmt.Errorf("the new %s would break %s:\n  %s", what, file, indentLines(own))
	}
	return nil
}

type overlayFS struct {
	fs.FS
	files fstest.MapFS
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if _, ok := o.files[name]; ok {
		return o.files.Open(name)
	}
	return o.FS.Open(name)
}

func writeWithBodies(full string, content []byte, bodyDir string, bodies map[string][]byte) error {
	perm := fs.FileMode(0o644)
	if info, err := os.Stat(full); err == nil {
		perm = info.Mode().Perm()
	}
	var written []string
	for name, data := range bodies {
		p := filepath.Join(bodyDir, name)
		if _, err := os.Stat(p); err == nil {
			return removeAll(written, fmt.Errorf("%s already exists; pick another state name", relBody(bodyDir, p)))
		}
		if err := os.MkdirAll(bodyDir, 0o755); err != nil {
			return removeAll(written, err)
		}
		if err := WriteFileAtomic(p, data, perm); err != nil {
			return removeAll(written, err)
		}
		written = append(written, p)
	}
	if err := WriteFileAtomic(full, content, perm); err != nil {
		return removeAll(written, err)
	}
	return nil
}

func removeAll(paths []string, err error) error {
	for _, p := range paths {
		err = errors.Join(err, os.Remove(p))
	}
	return err
}

func relBody(bodyDir, p string) string {
	return path.Join(routesDir, filepath.Base(bodyDir), filepath.Base(p))
}

func indentLines(probs Problems) string {
	return strings.ReplaceAll(probs.String(), "\n", "\n  ")
}
