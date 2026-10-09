package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const rulesRoute = `get:
  route: GET /users/{id}
  rules:
    - when: { path.id: u_404 }
      state: not_found
    - when:
        header.authorization: { exists: false }
        query.page: { gt: 3 }
      state: unauthorized
    - when:
        cookie.session: { in: [old, expired] }
        body.items.0.qty: { lte: 0 }
        var.signed_in: true
        call: { gte: 3 }
        query.q: { matches: "^a.*z$" }
        query.sort: { ne: name }
        query.limit: 10
        query.raw: { eq: "10" }
      state: unauthorized
  states:
    found: {}
    not_found: { status: 404 }
    unauthorized: { status: 401 }
`

func TestLoadFS_Rules(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/users.yaml": rulesRoute})
	r, _ := p.Route("users.get")
	if r.Mode != model.ModeRules {
		t.Errorf("Mode = %q, want rules (implied by rules)", r.Mode)
	}
	want := []model.Rule{
		{State: "not_found", Src: model.Source{File: "routes/users.yaml", Line: 4}, When: []model.Condition{
			{Scope: model.ScopePath, Key: "id", Op: model.OpEq, Value: "u_404"},
		}},
		{State: "unauthorized", Src: model.Source{File: "routes/users.yaml", Line: 6}, When: []model.Condition{
			{Scope: model.ScopeHeader, Key: "authorization", Op: model.OpExists, Value: false},
			{Scope: model.ScopeQuery, Key: "page", Op: model.OpGt, Value: 3.0},
		}},
		{State: "unauthorized", Src: model.Source{File: "routes/users.yaml", Line: 10}, When: []model.Condition{
			{Scope: model.ScopeCookie, Key: "session", Op: model.OpIn, Values: []any{"old", "expired"}},
			{Scope: model.ScopeBody, Key: "items.0.qty", Op: model.OpLte, Value: 0.0},
			{Scope: model.ScopeVar, Key: "signed_in", Op: model.OpEq, Value: true},
			{Scope: model.ScopeCall, Op: model.OpGte, Value: 3.0},
			{Scope: model.ScopeQuery, Key: "q", Op: model.OpMatches, Value: "^a.*z$"},
			{Scope: model.ScopeQuery, Key: "sort", Op: model.OpNe, Value: "name"},
			{Scope: model.ScopeQuery, Key: "limit", Op: model.OpEq, Value: 10.0},
			{Scope: model.ScopeQuery, Key: "raw", Op: model.OpEq, Value: "10"},
		}},
	}
	if diff := cmp.Diff(want, r.Rules, cmpopts.IgnoreFields(model.Condition{}, "Pattern")); diff != "" {
		t.Errorf("rules (-want +got):\n%s", diff)
	}
	if re := r.Rules[2].When[4].Pattern; re == nil || !re.MatchString("abcz") {
		t.Errorf("matches wasn't compiled: %v", re)
	}
}

func TestLoadFS_Modes(t *testing.T) {
	t.Parallel()

	for yaml, want := range map[string]model.Mode{
		"":                 model.ModeActive,
		"mode: active":     model.ModeActive,
		"mode: sequential": model.ModeSequential,
		"mode: random":     model.ModeRandom,
	} {
		file := "get:\n  route: GET /r\n  " + yaml + "\n  states:\n    ok: {}\n"
		r, _ := loadClean(t, map[string]string{"routes/r.yaml": file}).Route("r.get")
		if r.Mode != want {
			t.Errorf("%q: Mode = %q, want %q", yaml, r.Mode, want)
		}
	}
}

func TestLoadFS_RuleProblems(t *testing.T) {
	t.Parallel()

	route := func(rules string) string {
		return "get:\n  route: GET /users/{id}\n" + rules + "  states:\n    ok: {}\n    gone: { status: 410 }\n"
	}
	rule := func(when string) string {
		return route("  rules:\n    - when: " + when + "\n      state: gone\n")
	}
	tests := []struct{ name, file, want string }{
		{"unknown mode", "get:\n  route: GET /r\n  mode: rule\n  states:\n    ok: {}\n", `routes/users.yaml:3: mode "rule" isn't valid (did you mean "rules"?)`},
		{"rules mode without rules", route("  mode: rules\n"), `routes/users.yaml:3: mode rules needs rules`},
		{"rules with another mode", route("  mode: random\n  rules:\n    - when: { call: 1 }\n      state: gone\n"), `routes/users.yaml:5: rules only apply with mode: rules`},
		{"rules not a list", route("  rules: { when: {} }\n"), `routes/users.yaml:3: rules must be a list of { when, state }`},
		{"rule without state", route("  rules:\n    - when: { call: 1 }\n"), `routes/users.yaml:4: rule 1 needs "state"`},
		{"rule without when", route("  rules:\n    - state: gone\n"), `routes/users.yaml:4: rule 1 needs "when"`},
		{"empty when", rule("{}"), `routes/users.yaml:4: rule 1's when needs at least one condition`},
		{"unknown rule field", route("  rules:\n    - when: { call: 1 }\n      state: gone\n      stat: x\n"), `routes/users.yaml:6: unknown field "stat" (did you mean "state"?)`},
		{"unknown state", route("  rules:\n    - when: { call: 1 }\n      state: gon\n"), `routes/users.yaml:5: rule 1 points at state "gon", which doesn't exist (did you mean "gone"?)`},
		{"unknown scope", rule("{ heder.x: 1 }"), `routes/users.yaml:4: selector "heder.x" has an unknown scope "heder" (did you mean "header"?)`},
		{"no name", rule("{ query: 1 }"), `routes/users.yaml:4: selector "query" needs a name, like query.page`},
		{"call with a name", rule("{ call.x: 1 }"), `routes/users.yaml:4: selector "call.x" takes no name; write call`},
		{"not a path parameter", rule("{ path.ids: 1 }"), `routes/users.yaml:4: path.ids isn't a parameter of GET /users/{id} (did you mean "path.id"?)`},
		{"unknown operator", rule("{ call: { gtt: 3 } }"), `routes/users.yaml:4: unknown matcher "gtt" (did you mean "gt"?)`},
		{"two operators", rule("{ call: { gt: 1, lt: 5 } }"), `routes/users.yaml:4: the matcher for call has 2 operators (gt, lt); use one per condition`},
		{"in not a list", rule("{ call: { in: 3 } }"), `routes/users.yaml:4: in needs a list of values, like { in: [a, b] }`},
		{"bad regex", rule(`{ query.q: { matches: "(" } }`), `routes/users.yaml:4: matches "(" isn't a valid regex: missing closing ): ` + "`(`"},
		{"gt not a number", rule("{ call: { gt: many } }"), `routes/users.yaml:4: gt needs a number, like { gt: 3 }`},
		{"exists not a bool", rule("{ query.q: { exists: maybe } }"), `routes/users.yaml:4: exists needs true or false`},
		{"value not a scalar", rule("{ query.q: [a, b] }"), `routes/users.yaml:4: the value for query.q must be a single value, or a matcher like { in: [a, b] }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, map[string]string{"routes/users.yaml": tc.file})
			if got := probs.String(); got != tc.want {
				t.Errorf("problems:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
