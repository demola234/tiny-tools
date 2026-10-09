package match_test

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/match"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func facts() match.Facts {
	return match.Facts{
		Path:    map[string]string{"id": "u_1"},
		Query:   url.Values{"page": {"4", "9"}, "q": {"abz"}, "flag": {"true"}},
		Header:  http.Header{"Authorization": {"Bearer t"}},
		Cookies: map[string]string{"session": "old"},
		Body:    map[string]any{"email": "a@b.co", "qty": 2.0, "items": []any{map[string]any{"sku": "x1"}}, "note": nil, "ok": true},
		Vars:    map[string]any{"signed_in": true, "count": 3.0},
		Call:    2,
	}
}

func cond(scope model.Scope, key string, op model.Op, value any) model.Condition {
	return model.Condition{Scope: scope, Key: key, Op: op, Value: value}
}

func TestCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    model.Condition
		want bool
	}{
		{"path equals", cond(model.ScopePath, "id", model.OpEq, "u_1"), true},
		{"path differs", cond(model.ScopePath, "id", model.OpEq, "u_2"), false},
		{"query uses the first value", cond(model.ScopeQuery, "page", model.OpEq, 4.0), true},
		{"query number compares as a number", cond(model.ScopeQuery, "page", model.OpGt, 3.0), true},
		{"query number not greater", cond(model.ScopeQuery, "page", model.OpGt, 4.0), false},
		{"gte equal", cond(model.ScopeQuery, "page", model.OpGte, 4.0), true},
		{"lt", cond(model.ScopeQuery, "page", model.OpLt, 5.0), true},
		{"lte", cond(model.ScopeQuery, "page", model.OpLte, 3.0), false},
		{"text isn't a number", cond(model.ScopeQuery, "q", model.OpGt, 1.0), false},
		{"string against a number value", cond(model.ScopeQuery, "page", model.OpEq, "4"), true},
		{"bool against a query string", cond(model.ScopeQuery, "flag", model.OpEq, true), true},
		{"header is case-insensitive", cond(model.ScopeHeader, "authorization", model.OpEq, "Bearer t"), true},
		{"header exists", cond(model.ScopeHeader, "authorization", model.OpExists, true), true},
		{"missing header exists false", cond(model.ScopeHeader, "x-missing", model.OpExists, false), true},
		{"missing header equals nothing", cond(model.ScopeHeader, "x-missing", model.OpEq, ""), false},
		{"missing value is not equal", cond(model.ScopeHeader, "x-missing", model.OpNe, "a"), true},
		{"ne same value", cond(model.ScopePath, "id", model.OpNe, "u_1"), false},
		{"cookie", cond(model.ScopeCookie, "session", model.OpEq, "old"), true},
		{"body field", cond(model.ScopeBody, "email", model.OpEq, "a@b.co"), true},
		{"body number", cond(model.ScopeBody, "qty", model.OpEq, 2.0), true},
		{"body number as string", cond(model.ScopeBody, "qty", model.OpEq, "2"), true},
		{"body array index", cond(model.ScopeBody, "items.0.sku", model.OpEq, "x1"), true},
		{"body index out of range", cond(model.ScopeBody, "items.5.sku", model.OpExists, false), true},
		{"body null exists", cond(model.ScopeBody, "note", model.OpExists, true), true},
		{"body null equals null", cond(model.ScopeBody, "note", model.OpEq, nil), true},
		{"body bool", cond(model.ScopeBody, "ok", model.OpEq, true), true},
		{"body through a string", cond(model.ScopeBody, "email.x", model.OpExists, false), true},
		{"var", cond(model.ScopeVar, "signed_in", model.OpEq, true), true},
		{"var number", cond(model.ScopeVar, "count", model.OpGte, 3.0), true},
		{"missing var", cond(model.ScopeVar, "nope", model.OpExists, false), true},
		{"call", cond(model.ScopeCall, "", model.OpEq, 2.0), true},
		{"call greater", cond(model.ScopeCall, "", model.OpGt, 2.0), false},
		{"in", model.Condition{Scope: model.ScopeCookie, Key: "session", Op: model.OpIn, Values: []any{"new", "old"}}, true},
		{"not in", model.Condition{Scope: model.ScopeCookie, Key: "session", Op: model.OpIn, Values: []any{"new"}}, false},
		{"matches", model.Condition{Scope: model.ScopeQuery, Key: "q", Op: model.OpMatches, Pattern: regexp.MustCompile("^a.*z$")}, true},
		{"matches a number", model.Condition{Scope: model.ScopeBody, Key: "qty", Op: model.OpMatches, Pattern: regexp.MustCompile(`^\d$`)}, true},
		{"missing never matches a regex", model.Condition{Scope: model.ScopeQuery, Key: "x", Op: model.OpMatches, Pattern: regexp.MustCompile(".*")}, false},
	}
	f := facts()
	for _, tc := range tests {
		if got := match.Condition(tc.c, f); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFirst(t *testing.T) {
	t.Parallel()

	rules := []model.Rule{
		{State: "a", When: []model.Condition{cond(model.ScopePath, "id", model.OpEq, "u_2")}},
		{State: "b", When: []model.Condition{cond(model.ScopePath, "id", model.OpEq, "u_1"), cond(model.ScopeCall, "", model.OpEq, 9.0)}},
		{State: "c", When: []model.Condition{cond(model.ScopePath, "id", model.OpEq, "u_1"), cond(model.ScopeCall, "", model.OpEq, 2.0)}},
		{State: "d", When: []model.Condition{cond(model.ScopePath, "id", model.OpEq, "u_1")}},
	}
	if i, ok := match.First(rules, facts()); !ok || i != 2 {
		t.Errorf("First = %d, %v; want rule 3 (index 2)", i, ok)
	}
	if _, ok := match.First(rules[:2], facts()); ok {
		t.Error("First matched when no rule should")
	}
}

func FuzzCondition(f *testing.F) {
	f.Add("items.0.sku", "x1", 1.0)
	f.Add("..", "", -1.0)
	f.Fuzz(func(_ *testing.T, key, s string, n float64) {
		fs := facts()
		for _, op := range model.Ops() {
			match.Condition(model.Condition{Scope: model.ScopeBody, Key: key, Op: op, Value: s, Values: []any{s, n}, Pattern: regexp.MustCompile(".")}, fs)
			match.Condition(model.Condition{Scope: model.ScopeQuery, Key: key, Op: op, Value: n}, fs)
		}
	})
}
