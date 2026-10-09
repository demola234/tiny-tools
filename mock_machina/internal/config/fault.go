package config

import (
	"slices"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var (
	faultTypes  = []string{string(model.FaultTimeout), string(model.FaultReset), string(model.FaultTruncated)}
	faultFields = []string{"type", "rate", "after"}
)

func (l *loader) fault(file string, v *yaml.Node) *model.Fault {
	f := &model.Fault{Rate: 1}
	typ := v
	if v.Kind == yaml.MappingNode {
		if typ = l.faultOptions(file, v, f); typ == nil {
			return nil
		}
	}
	if !slices.Contains(faultTypes, typ.Value) {
		l.add(file, typ.Line, "fault %q isn't valid (%s)", typ.Value, closeMatchOr(typ.Value, faultTypes, "faults: timeout, reset, truncated"))
		return nil
	}
	f.Type = model.FaultType(typ.Value)
	return f
}

func (l *loader) faultOptions(file string, v *yaml.Node, f *model.Fault) *yaml.Node {
	fields := l.fields(file, v, faultFields, nil)
	typ := fields["type"]
	if typ == nil {
		l.add(file, v.Line, "fault needs a type: timeout, reset or truncated")
		return nil
	}
	if r := fields["rate"]; r != nil {
		rate, err := strconv.ParseFloat(r.Value, 64)
		if err != nil || rate <= 0 || rate > 1 {
			l.add(file, r.Line, "fault rate must be more than 0 and at most 1, like 0.2")
			return nil
		}
		f.Rate = rate
	}
	if a := fields["after"]; a != nil {
		f.After = l.duration(file, "fault after", a)
	}
	return typ
}
