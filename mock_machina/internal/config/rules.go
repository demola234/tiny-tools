package config

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var ruleFields = []string{"when", "state"}

func names[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

func (l *loader) mode(f map[string]*yaml.Node, r *model.Route) {
	r.Mode = model.ModeActive
	if f["rules"] != nil {
		r.Mode = model.ModeRules
	}
	v := f["mode"]
	if v == nil {
		return
	}
	modes := names(model.Modes())
	if !slices.Contains(modes, v.Value) {
		l.add(r.Src.File, v.Line, "mode %q isn't valid (%s)", v.Value, closeMatchOr(v.Value, modes, "modes: "+strings.Join(modes, ", ")))
		return
	}
	r.Mode = model.Mode(v.Value)
	if r.Mode == model.ModeRules && f["rules"] == nil {
		l.add(r.Src.File, v.Line, "mode rules needs rules")
	}
}

func (l *loader) rules(f map[string]*yaml.Node, r *model.Route) {
	v := f["rules"]
	if v == nil {
		return
	}
	file := r.Src.File
	switch {
	case v.Kind != yaml.SequenceNode:
		l.add(file, v.Line, "rules must be a list of { when, state }")
		return
	case r.Mode != model.ModeRules:
		l.add(file, v.Line, "rules only apply with mode: rules")
		return
	}
	for i, item := range v.Content {
		if rule, ok := l.rule(r, i+1, item); ok {
			r.Rules = append(r.Rules, rule)
		}
	}
}

func (l *loader) rule(r *model.Route, n int, item *yaml.Node) (model.Rule, bool) {
	file := r.Src.File
	if item.Kind != yaml.MappingNode {
		l.add(file, item.Line, "rule %d must be a mapping of when and state", n)
		return model.Rule{}, false
	}
	f := l.fields(file, item, ruleFields, nil)
	for _, field := range ruleFields {
		if f[field] == nil {
			l.add(file, item.Line, "rule %d needs %q", n, field)
			return model.Rule{}, false
		}
	}
	rule := model.Rule{State: f["state"].Value, Src: model.Source{File: file, Line: item.Line}}
	if _, ok := r.States.Get(rule.State); !ok && len(r.States) > 0 {
		states := r.States.Names()
		l.add(file, f["state"].Line, "rule %d points at state %q, which doesn't exist (%s)", n, rule.State,
			closeMatchOr(rule.State, states, "states: "+strings.Join(states, ", ")))
	}
	when := f["when"]
	if when.Kind != yaml.MappingNode {
		l.add(file, when.Line, "rule %d's when must map selectors to values, like { path.id: u_1 }", n)
		return model.Rule{}, false
	}
	if len(when.Content) == 0 {
		l.add(file, when.Line, "rule %d's when needs at least one condition", n)
		return model.Rule{}, false
	}
	for key, val := range pairs(when) {
		if c, ok := l.condition(r, key, val); ok {
			rule.When = append(rule.When, c)
		}
	}
	return rule, true
}

func (l *loader) condition(r *model.Route, key, val *yaml.Node) (model.Condition, bool) {
	c, msg := selector(r, key.Value)
	if msg == "" {
		msg = matcher(key.Value, val, &c)
	}
	if msg != "" {
		l.add(r.Src.File, key.Line, "%s", msg)
		return model.Condition{}, false
	}
	return c, true
}

func selector(r *model.Route, s string) (model.Condition, string) {
	scope, key, _ := strings.Cut(s, ".")
	scopes := names(model.Scopes())
	c := model.Condition{Scope: model.Scope(scope), Key: key}
	switch {
	case !slices.Contains(scopes, scope):
		return c, fmt.Sprintf("selector %q has an unknown scope %q (%s)", s, scope, closeMatchOr(scope, scopes, "scopes: "+strings.Join(scopes, ", ")))
	case c.Scope == model.ScopeCall && key != "":
		return c, fmt.Sprintf("selector %q takes no name; write call", s)
	case c.Scope != model.ScopeCall && key == "":
		return c, fmt.Sprintf("selector %q needs a name, like %s", s, map[model.Scope]string{
			model.ScopePath: "path.id", model.ScopeQuery: "query.page", model.ScopeHeader: "header.authorization",
			model.ScopeCookie: "cookie.session", model.ScopeBody: "body.email", model.ScopeVar: "var.signed_in",
		}[c.Scope])
	case c.Scope == model.ScopePath:
		return c, pathParamProblem(r, key)
	}
	return c, ""
}

func pathParamProblem(r *model.Route, key string) string {
	found := pathParamName.FindAllStringSubmatch(r.Path, -1)
	params := make([]string, 0, len(found))
	for _, m := range found {
		params = append(params, "path."+m[1])
	}
	if slices.Contains(params, "path."+key) {
		return ""
	}
	hint := "it has none"
	if len(params) > 0 {
		hint = closeMatchOr("path."+key, params, "parameters: "+strings.Join(params, ", "))
	}
	return fmt.Sprintf("path.%s isn't a parameter of %s %s (%s)", key, r.Method, r.Path, hint)
}

func matcher(sel string, val *yaml.Node, c *model.Condition) string {
	switch val.Kind {
	case yaml.ScalarNode:
		c.Op, c.Value = model.OpEq, scalarValue(val)
		return ""
	case yaml.MappingNode:
	default:
		return fmt.Sprintf("the value for %s must be a single value, or a matcher like { in: [a, b] }", sel)
	}
	if n := len(val.Content) / 2; n != 1 {
		var ops []string
		for k := range pairs(val) {
			ops = append(ops, k.Value)
		}
		return fmt.Sprintf("the matcher for %s has %d operators (%s); use one per condition", sel, n, strings.Join(ops, ", "))
	}
	op, arg := val.Content[0].Value, val.Content[1]
	ops := names(model.Ops())
	if !slices.Contains(ops, op) {
		return fmt.Sprintf("unknown matcher %q (%s)", op, closeMatchOr(op, ops, "matchers: "+strings.Join(ops, ", ")))
	}
	c.Op = model.Op(op)
	return operand(c, arg)
}

func operand(c *model.Condition, arg *yaml.Node) string {
	switch c.Op {
	case model.OpIn:
		if arg.Kind != yaml.SequenceNode {
			return "in needs a list of values, like { in: [a, b] }"
		}
		for _, item := range arg.Content {
			c.Values = append(c.Values, scalarValue(item))
		}
	case model.OpMatches:
		re, err := regexp.Compile(arg.Value)
		if err != nil {
			return fmt.Sprintf("matches %q isn't a valid regex: %s", arg.Value, regexProblem(err))
		}
		c.Value, c.Pattern = arg.Value, re
	case model.OpExists:
		if arg.Tag != "!!bool" {
			return "exists needs true or false"
		}
		c.Value = scalarValue(arg)
	case model.OpGt, model.OpGte, model.OpLt, model.OpLte:
		if arg.Tag != "!!int" && arg.Tag != "!!float" {
			return fmt.Sprintf("%s needs a number, like { %s: 3 }", c.Op, c.Op)
		}
		c.Value = scalarValue(arg)
	default:
		c.Value = scalarValue(arg)
	}
	return ""
}

func regexProblem(err error) string {
	var se *syntax.Error
	if errors.As(err, &se) {
		return string(se.Code) + ": `" + se.Expr + "`"
	}
	return err.Error()
}

func scalarValue(n *yaml.Node) any {
	switch n.Tag {
	case "!!int", "!!float":
		f, err := strconv.ParseFloat(strings.ReplaceAll(n.Value, "_", ""), 64)
		if err == nil {
			return f
		}
	case "!!bool":
		var b bool
		if n.Decode(&b) == nil {
			return b
		}
	case "!!null":
		return nil
	}
	return n.Value
}
