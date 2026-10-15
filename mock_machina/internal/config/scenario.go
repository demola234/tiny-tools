package config

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	scenariosDir    = "scenarios"
	scenarioExample = "states: { users.list: empty }"
)

var scenarioFields = []string{"title", "description", "states"}

func (l *loader) scenarios(p *model.Project) []*model.Scenario {
	entries, err := fs.ReadDir(l.fsys, scenariosDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			l.add(scenariosDir, 0, "can't read this folder: %v", err)
		}
		return nil
	}
	var out []*model.Scenario
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || e.IsDir() {
			continue
		}
		file := path.Join(scenariosDir, name)
		switch path.Ext(name) {
		case resourceExt:
			if s := l.scenario(p, file, strings.TrimSuffix(name, resourceExt)); s != nil {
				out = append(out, s)
			}
		case misspeltExt:
			l.add(file, 0, "use the %s extension, so mockmachina reads this file", resourceExt)
		}
	}
	slices.SortFunc(out, func(a, b *model.Scenario) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (l *loader) scenario(p *model.Project, file, name string) *model.Scenario {
	if !resourceName.MatchString(name) {
		l.add(file, 0, "file name %q must use lowercase letters, digits and dashes", name)
		return nil
	}
	root := l.parse(file)
	if root == nil {
		return nil
	}
	if root.Kind != yaml.MappingNode {
		l.add(file, root.Line, "a scenario must be a mapping, like %s", scenarioExample)
		return nil
	}
	s := &model.Scenario{Name: name, Src: model.Source{File: file, Line: root.Line}}
	f := l.fields(file, root, scenarioFields, nil)
	if v := f["title"]; v != nil {
		s.Title = l.oneLine(file, "title", v)
	}
	if v := f["description"]; v != nil {
		s.Description = l.oneLine(file, "description", v)
	}
	switch v := f["states"]; {
	case v == nil:
		l.add(file, root.Line, "a scenario needs %q, like %s", "states", scenarioExample)
	case v.Kind != yaml.MappingNode || len(v.Content) == 0:
		l.add(file, v.Line, "states needs at least one route, like %s", scenarioExample)
	default:
		s.States = l.scenarioStates(p, file, v)
	}
	return s
}

func (l *loader) scenarioStates(p *model.Project, file string, v *yaml.Node) []model.ScenarioState {
	ids := make([]string, len(p.Routes))
	for i, r := range p.Routes {
		ids[i] = r.ID
	}
	var out []model.ScenarioState
	firstLine := map[string]int{}
	for key, val := range pairs(v) {
		id := key.Value
		if first, seen := firstLine[id]; seen {
			l.add(file, key.Line, "%q is listed twice (first on line %d)", id, first)
			continue
		}
		firstLine[id] = key.Line
		if val.Kind != yaml.ScalarNode || val.Value == "" {
			l.add(file, key.Line, "the state for %s must be a state name, like %s: ok", id, id)
			continue
		}
		r, ok := p.Route(id)
		if !ok {
			l.add(file, key.Line, "no route %q (%s)", id, closeMatchOr(id, ids, "routes: "+strings.Join(ids, ", ")))
			continue
		}
		if _, ok := r.States.Get(val.Value); !ok {
			names := r.States.Names()
			l.add(file, key.Line, "route %s has no state %q (%s)", id, val.Value, closeMatchOr(val.Value, names, "states: "+strings.Join(names, ", ")))
			continue
		}
		out = append(out, model.ScenarioState{Route: id, State: val.Value})
	}
	return out
}
