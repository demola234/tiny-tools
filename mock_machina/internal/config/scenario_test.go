package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const scenarioRoutes = `get:
  route: GET /r
  states:
    ok: {}
    down: { status: 503 }
list:
  route: GET /rs
  states:
    ok: {}
`

func TestLoadFS_Scenarios(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/r.yaml": scenarioRoutes,
		"scenarios/checkout-fails.yaml": `title: Checkout fails
description: Card declined at payment.
states:
  r.get: down
  r.list: ok
`,
		"scenarios/all-good.yaml": "states: { r.get: ok }\n",
	})
	want := []*model.Scenario{
		{Name: "all-good", States: []model.ScenarioState{{Route: "r.get", State: "ok"}}, Src: model.Source{File: "scenarios/all-good.yaml", Line: 1}},
		{
			Name: "checkout-fails", Title: "Checkout fails", Description: "Card declined at payment.",
			States: []model.ScenarioState{{Route: "r.get", State: "down"}, {Route: "r.list", State: "ok"}},
			Src:    model.Source{File: "scenarios/checkout-fails.yaml", Line: 1},
		},
	}
	if diff := cmp.Diff(want, p.Scenarios); diff != "" {
		t.Errorf("scenarios (-want +got):\n%s", diff)
	}
	if s, ok := p.Scenario("checkout-fails"); !ok || s.Title != "Checkout fails" {
		t.Errorf("Scenario(checkout-fails) = %v, %v", s, ok)
	}
	if _, ok := p.Scenario("nope"); ok {
		t.Error("Scenario(nope) found something")
	}
}

func TestLoadFS_ScenarioProblems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scenario string
		want     []string
	}{
		{"unknown route", "states:\n  r.gte: ok\n", []string{`scenarios/s.yaml:2: no route "r.gte" (did you mean "r.get"?)`}},
		{"unknown state", "states:\n  r.get: dwn\n", []string{`scenarios/s.yaml:2: route r.get has no state "dwn" (did you mean "down"?)`}},
		{"no states", "title: Empty\n", []string{`scenarios/s.yaml:1: a scenario needs "states", like states: { users.list: empty }`}},
		{"empty states", "states: {}\n", []string{`scenarios/s.yaml:1: states needs at least one route, like states: { users.list: empty }`}},
		{"states not a mapping", "states: [r.get]\n", []string{`scenarios/s.yaml:1: states needs at least one route, like states: { users.list: empty }`}},
		{"state not a name", "states:\n  r.get: [ok]\n", []string{`scenarios/s.yaml:2: the state for r.get must be a state name, like r.get: ok`}},
		{"route twice", "states:\n  r.get: ok\n  r.get: down\n", []string{`scenarios/s.yaml:3: "r.get" is listed twice (first on line 2)`}},
		{"unknown field", "titel: x\nstates: { r.get: ok }\n", []string{`scenarios/s.yaml:1: unknown field "titel" (did you mean "title"?)`}},
		{"title on two lines", "title: |\n  a\n  b\nstates: { r.get: ok }\n", []string{`scenarios/s.yaml:1: title must be one line of text`}},
		{"not a mapping", "- r.get\n", []string{`scenarios/s.yaml:1: a scenario must be a mapping, like states: { users.list: empty }`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, map[string]string{"routes/r.yaml": scenarioRoutes, "scenarios/s.yaml": tt.scenario})
			if diff := cmp.Diff(tt.want, problemStrings(probs)); diff != "" {
				t.Errorf("problems (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadFS_ScenarioFileNames(t *testing.T) {
	t.Parallel()

	_, probs := load(t, map[string]string{
		"routes/r.yaml":        scenarioRoutes,
		"scenarios/Bad.yaml":   "states: { r.get: ok }\n",
		"scenarios/other.yml":  "states: { r.get: ok }\n",
		"scenarios/.hidden":    "x",
		"scenarios/notes.md":   "x",
		"scenarios/dir/a.yaml": "x",
	})
	want := []string{
		`scenarios/Bad.yaml: file name "Bad" must use lowercase letters, digits and dashes`,
		`scenarios/other.yml: use the .yaml extension, so mockmachina reads this file`,
	}
	if diff := cmp.Diff(want, problemStrings(probs)); diff != "" {
		t.Errorf("problems (-want +got):\n%s", diff)
	}
}

func problemStrings(probs config.Problems) []string {
	got := make([]string, len(probs))
	for i, p := range probs {
		got[i] = p.String()
	}
	return got
}
