package match

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type Facts struct {
	Path    map[string]string
	Query   url.Values
	Header  http.Header
	Cookies map[string]string
	Body    any
	Vars    map[string]any
	Call    int
}

func First(rules []model.Rule, f Facts) (int, bool) {
	for i, r := range rules {
		if all(r.When, f) {
			return i, true
		}
	}
	return 0, false
}

func all(conds []model.Condition, f Facts) bool {
	for _, c := range conds {
		if !Condition(c, f) {
			return false
		}
	}
	return true
}

func Condition(c model.Condition, f Facts) bool {
	actual, present := Lookup(c.Scope, c.Key, f)
	switch c.Op {
	case model.OpExists:
		return present == (c.Value == true)
	case model.OpNe:
		return !present || !equal(c.Value, actual)
	case model.OpEq:
		return present && equal(c.Value, actual)
	case model.OpIn:
		return present && slices.ContainsFunc(c.Values, func(v any) bool { return equal(v, actual) })
	case model.OpMatches:
		return present && c.Pattern != nil && c.Pattern.MatchString(text(actual))
	case model.OpGt, model.OpGte, model.OpLt, model.OpLte:
		return present && compare(c.Op, c.Value, actual)
	}
	return false
}

func Lookup(scope model.Scope, key string, f Facts) (any, bool) {
	switch scope {
	case model.ScopePath:
		v, ok := f.Path[key]
		return v, ok
	case model.ScopeQuery:
		if vs, ok := f.Query[key]; ok && len(vs) > 0 {
			return vs[0], true
		}
	case model.ScopeHeader:
		if vs := f.Header.Values(key); len(vs) > 0 {
			return vs[0], true
		}
	case model.ScopeCookie:
		v, ok := f.Cookies[key]
		return v, ok
	case model.ScopeBody:
		return walk(f.Body, key)
	case model.ScopeVar:
		v, ok := f.Vars[key]
		return v, ok
	case model.ScopeCall:
		return float64(f.Call), true
	}
	return nil, false
}

func walk(v any, path string) (any, bool) {
	for part := range strings.SplitSeq(path, ".") {
		switch node := v.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return nil, false
			}
			v = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			v = node[i]
		default:
			return nil, false
		}
	}
	return v, true
}

func equal(want, actual any) bool {
	switch w := want.(type) {
	case nil:
		return actual == nil
	case float64:
		n, ok := number(actual)
		return ok && n == w
	case bool:
		return actual == w || text(actual) == strconv.FormatBool(w)
	default:
		return actual != nil && text(actual) == text(want)
	}
}

func compare(op model.Op, want, actual any) bool {
	w, ok1 := want.(float64)
	a, ok2 := number(actual)
	if !ok1 || !ok2 {
		return false
	}
	switch op {
	case model.OpGt:
		return a > w
	case model.OpGte:
		return a >= w
	case model.OpLt:
		return a < w
	default:
		return a <= w
	}
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n, err == nil
	}
	return 0, false
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	}
	return ""
}
