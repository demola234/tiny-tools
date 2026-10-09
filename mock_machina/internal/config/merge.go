package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type MergeReport struct {
	Added     []string
	Updated   []string
	NewStates []string
	NotInSpec []string
	Files     []string
}

type merger struct {
	dir   string
	files map[string][]byte
	extra map[string][]byte
}

func MergeContract(dir string, routes []*model.Route, schemas []*model.Schema, dryRun bool) (MergeReport, error) {
	existing, probs, err := Load(dir)
	if err != nil {
		return MergeReport{}, fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	if probs.HasErrors() {
		return MergeReport{}, fmt.Errorf("the project has problems; fix them first:\n  %s", indentLines(probs.Sorted()))
	}
	m := &merger{dir: dir, files: map[string][]byte{}, extra: map[string][]byte{}}
	var rep MergeReport
	imported := map[string]bool{}
	var added []*model.Route
	for _, r := range routes {
		imported[r.ID] = true
		old, ok := existing.Route(r.ID)
		if !ok {
			added = append(added, r)
			rep.Added = append(rep.Added, r.ID)
			continue
		}
		updated, states, err := m.update(old, r)
		if err != nil {
			return MergeReport{}, fmt.Errorf("route %s: %w", r.ID, err)
		}
		if updated {
			rep.Updated = append(rep.Updated, r.ID)
		}
		rep.NewStates = append(rep.NewStates, states...)
	}
	for _, r := range existing.Routes {
		if !imported[r.ID] {
			rep.NotInSpec = append(rep.NotInSpec, r.ID)
		}
	}
	if err := m.add(added); err != nil {
		return MergeReport{}, err
	}
	if err := m.schemas(schemas); err != nil {
		return MergeReport{}, err
	}
	rep.Files = slices.Sorted(maps.Keys(m.files))
	if dryRun {
		return rep, nil
	}
	return rep, m.write()
}

func (m *merger) read(file string) ([]byte, error) {
	if data, ok := m.files[file]; ok {
		return data, nil
	}
	data, err := os.ReadFile(filepath.Join(m.dir, filepath.FromSlash(file)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func (m *merger) update(old, r *model.Route) (bool, []string, error) {
	file := old.Src.File
	data, err := m.read(file)
	if err != nil {
		return false, nil, err
	}
	name := strings.TrimPrefix(old.ID, old.Group+".")
	before := data
	if data, err = updateFields(data, name, old, r); err != nil {
		return false, nil, err
	}
	updated := !bytes.Equal(before, data)
	data, states, err := m.addStates(data, name, old, r)
	if err != nil {
		return false, nil, err
	}
	if !bytes.Equal(before, data) {
		m.files[file] = data
	}
	return updated, states, nil
}

func updateFields(data []byte, name string, old, r *model.Route) ([]byte, error) {
	for _, field := range []string{"request", "responses"} {
		was, err := fieldText(field, old)
		if err != nil {
			return nil, err
		}
		now, err := fieldText(field, r)
		if err != nil {
			return nil, err
		}
		if was != now {
			if data, err = replaceField(data, name, field, now); err != nil {
				return nil, err
			}
		}
	}
	return data, nil
}

func (m *merger) addStates(data []byte, name string, old, r *model.Route) ([]byte, []string, error) {
	var states []string
	for _, ns := range r.States {
		if _, ok := old.States.Get(ns.Name); ok {
			continue
		}
		node, body, err := stateNode(r, name, ns)
		if err != nil {
			return nil, nil, err
		}
		if body != nil {
			m.extra[body.name] = body.data
		}
		if data, err = insertState(data, name, ns.Name, node); err != nil {
			return nil, nil, err
		}
		states = append(states, r.ID+"."+ns.Name)
	}
	return data, states, nil
}

func fieldText(field string, r *model.Route) (string, error) {
	var n *yaml.Node
	var err error
	switch {
	case field == "request" && r.Request != nil:
		n, err = requestNode(r.Request)
	case field == "responses" && len(r.Responses) > 0:
		n, err = responsesNode(r.Responses)
	default:
		return "", nil
	}
	if err != nil {
		return "", err
	}
	doc := mapping()
	doc.Content = append(doc.Content, str(field), n)
	return encode(doc, defaultIndent)
}

func routeNode(data []byte, routeName string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for key, val := range pairs(doc.Content[0]) {
		if key.Value == routeName {
			if val.Style&yaml.FlowStyle != 0 {
				return nil, errors.New("is written on one line, so re-import can't update it; split it over several lines")
			}
			return val, nil
		}
	}
	return nil, fmt.Errorf("route %q isn't in the file", routeName)
}

func replaceField(data []byte, routeName, field, text string) ([]byte, error) {
	route, err := routeNode(data, routeName)
	if err != nil {
		return nil, err
	}
	var key, states *yaml.Node
	for k := range pairs(route) {
		switch k.Value {
		case field:
			key = k
		case "states":
			states = k
		}
	}
	eol := eolOf(data)
	switch {
	case key != nil:
		start := lineOffset(data, key.Line)
		end := blockEnd(data, key.Line, key.Column-1)
		return slices.Concat(data[:start], []byte(padLines(text, key.Column-1, eol)), data[end:]), nil
	case text == "":
		return data, nil
	case states != nil:
		at := lineOffset(data, states.Line)
		return slices.Concat(data[:at], []byte(padLines(text, states.Column-1, eol)), data[at:]), nil
	}
	return nil, errors.New("has no states")
}

func insertState(data []byte, routeName, stateName string, node *yaml.Node) ([]byte, error) {
	states, err := statesOf(data, routeName)
	if err != nil {
		return nil, err
	}
	lastKey := states.Content[len(states.Content)-2]
	doc := mapping()
	doc.Content = append(doc.Content, str(stateName), node)
	text, err := encode(doc, defaultIndent)
	if err != nil {
		return nil, err
	}
	at := blockEnd(data, lastKey.Line, lastKey.Column-1)
	var lead []byte
	if at == len(data) && at > 0 && data[at-1] != '\n' {
		lead = []byte(eolOf(data))
	}
	return slices.Concat(data[:at], lead, []byte(padLines(text, lastKey.Column-1, eolOf(data))), data[at:]), nil
}

func padLines(text string, indent int, eol string) string {
	if text == "" {
		return ""
	}
	pad := strings.Repeat(" ", indent)
	var b strings.Builder
	for l := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
		b.WriteString(pad + l + eol)
	}
	return b.String()
}

func (m *merger) add(routes []*model.Route) error {
	var order []string
	byGroup := map[string][]*model.Route{}
	for _, r := range routes {
		if _, ok := byGroup[r.Group]; !ok {
			order = append(order, r.Group)
		}
		byGroup[r.Group] = append(byGroup[r.Group], r)
	}
	for _, group := range order {
		file := path.Join(routesDir, group+resourceExt)
		existing, err := m.read(file)
		if err != nil {
			return err
		}
		rendered, err := renderGroup(group, byGroup[group])
		if err != nil {
			return err
		}
		for _, f := range rendered[:len(rendered)-1] {
			m.extra[f.name] = f.data
		}
		text := rendered[len(rendered)-1].data
		if existing == nil {
			m.files[file] = text
			continue
		}
		block := strings.TrimPrefix(string(text), schemaLine("route"))
		m.files[file] = appended(existing, strings.TrimSuffix(block, "\n")+"\n")
	}
	return nil
}

func (m *merger) schemas(schemas []*model.Schema) error {
	if len(schemas) == 0 {
		return nil
	}
	f, err := renderSchemas(schemas)
	if err != nil {
		return err
	}
	current, err := m.read(f.name)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, f.data) {
		m.files[f.name] = f.data
	}
	return nil
}

func (m *merger) write() error {
	all := maps.Clone(m.files)
	maps.Copy(all, m.extra)
	for _, name := range slices.Sorted(maps.Keys(all)) {
		full := filepath.Join(m.dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := WriteFileAtomic(full, all[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}
