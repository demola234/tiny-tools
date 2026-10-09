package openapi

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var routeID = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)\.([a-z0-9][a-z0-9-]*)$`)

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

func native(v any) any {
	switch x := v.(type) {
	case json.Number:
		f, _ := x.Float64()
		return f
	case []any:
		out := make([]any, len(x))
		for i, c := range x {
			out[i] = native(c)
		}
		return out
	}
	return v
}

func duration(v any) time.Duration {
	s, _ := v.(string)
	d, _ := time.ParseDuration(s)
	return d
}

func applyRoute(r *model.Route, ext *omap) {
	if ext == nil {
		return
	}
	if s := ext.str("status"); s != "" {
		r.Status = model.Status(s)
	}
	if o := ext.obj("owners"); o != nil {
		r.Owners = model.Owners{Backend: textList(o.get("backend")), Frontend: textList(o.get("frontend"))}
	}
	if a := ext.str("active"); a != "" {
		r.Active = a
	}
	if rules, ok := ext.get("rules").([]any); ok {
		r.Rules, r.Mode = rulesIn(rules), model.ModeRules
	}
	if m := ext.str("mode"); m != "" {
		r.Mode = model.Mode(m)
	}
	if ext.str("serve") == string(model.ServeProxy) {
		r.Serve = model.ServeProxy
	}
	if c := ext.obj("crud"); c != nil {
		r.CRUD = &model.CRUD{Collection: c.str("collection"), IDField: orDefault(c.str("idField"), "id")}
	}
}

func textList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, _ := item.(string)
		out = append(out, s)
	}
	return out
}

func rulesIn(list []any) []model.Rule {
	out := make([]model.Rule, 0, len(list))
	for _, raw := range list {
		r, _ := raw.(*omap)
		rule := model.Rule{State: r.str("state")}
		when := r.obj("when")
		for i, selector := range when.keysOrNil() {
			rule.When = append(rule.When, condition(selector, when.vals[i]))
		}
		out = append(out, rule)
	}
	return out
}

func condition(selector string, v any) model.Condition {
	scope, key, _ := strings.Cut(selector, ".")
	c := model.Condition{Scope: model.Scope(scope), Key: key, Op: model.OpEq, Value: native(v)}
	m, ok := v.(*omap)
	if !ok || len(m.keys) != 1 {
		return c
	}
	c.Op, c.Value = model.Op(m.keys[0]), native(m.vals[0])
	switch c.Op {
	case model.OpIn:
		c.Values, _ = native(m.vals[0]).([]any)
		c.Value = nil
	case model.OpMatches:
		if s, ok := c.Value.(string); ok {
			c.Pattern, _ = regexp.Compile(s)
		}
	default:
	}
	return c
}

func applyState(st *model.State, ext *omap) {
	if ext.get("generate") == true {
		st.Body = model.Body{Generate: true, ContentType: "application/json"}
	}
	if n, ok := number(ext.get("status")); ok {
		st.Status = int(n)
	}
	if h := ext.obj("headers"); h != nil {
		st.Headers = map[string]string{}
		for i, k := range h.keys {
			st.Headers[k], _ = h.vals[i].(string)
		}
	}
	if l := ext.obj("latency"); l != nil {
		st.Latency, st.Jitter = duration(l.get("base")), duration(l.get("jitter"))
	}
	if f := ext.obj("fault"); f != nil {
		rate, _ := number(f.get("rate"))
		st.Fault = &model.Fault{Type: model.FaultType(f.str("type")), Rate: rate, After: duration(f.get("after"))}
	}
	if set := ext.obj("set"); set != nil {
		for i, k := range set.keys {
			st.Set = append(st.Set, model.Assignment{Name: k, Value: native(set.vals[i])})
		}
	}
	st.Verbatim = ext.get("template") == false
	st.SkipRequestValidation = ext.get("validateRequest") == false
}
