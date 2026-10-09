package config

import (
	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

func (l *loader) set(file string, v *yaml.Node) []model.Assignment {
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "set must map variable names to values, like { signed_in: true }")
		return nil
	}
	var out []model.Assignment
	for key, val := range pairs(v) {
		name := key.Value
		switch {
		case !model.ValidStateName(name):
			l.add(file, key.Line, "variable %q must use lowercase letters, digits and underscores, starting with a letter", name)
			continue
		case val.Kind != yaml.ScalarNode:
			l.add(file, key.Line, "set.%s must be a single value, like true, 3 or \"{{ body.email }}\"", name)
			continue
		}
		value := scalarValue(val)
		if s, ok := value.(string); ok && tmpl.Has([]byte(s)) {
			if _, err := tmpl.Parse(s); err != nil {
				l.add(file, key.Line, "set.%s: %v", name, err)
				continue
			}
		}
		out = append(out, model.Assignment{Name: name, Value: value})
	}
	return out
}
