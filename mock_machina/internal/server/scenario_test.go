package server_test

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func scenarioProject() *model.Project {
	p := coverageProject()
	p.Routes = append(p.Routes, &model.Route{
		ID: "orders.list", Method: model.MethodGet, Path: "/orders", Active: "ok",
		States: model.States{
			{Name: "ok", State: &model.State{Body: jsonBody(`[1]`)}},
			{Name: "down", State: &model.State{Status: 503}},
		},
	})
	p.Scenarios = []*model.Scenario{
		{Name: "bad-day", States: []model.ScenarioState{{Route: "users.list", State: "empty"}, {Route: "orders.list", State: "down"}}},
		{Name: "new-user", States: []model.ScenarioState{{Route: "users.list", State: "empty"}}},
	}
	return p
}

func TestScenario_PicksTheState(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	mem.Pin("users.list", "broken")
	var got []server.Request
	h, err := server.New(scenarioProject(), server.Options{Memory: mem, Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		target string
		header map[string]string
		code   int
		state  string
		reason server.Reason
	}{
		{"/users", map[string]string{"X-Mock-Scenario": "bad-day"}, 200, "empty", server.ReasonScenario},
		{"/orders", map[string]string{"X-Mock-Scenario": "bad-day"}, 503, "down", server.ReasonScenario},
		{"/orders", map[string]string{"X-Mock-Scenario": "new-user"}, 200, "ok", server.ReasonActive},
		{"/users", map[string]string{"X-Mock-Scenario": "bad-day", "X-Mock-State": "full"}, 200, "full", server.ReasonHeader},
		{"/users?__state=full", map[string]string{"X-Mock-Scenario": "bad-day"}, 200, "full", server.ReasonQuery},
		{"/users", nil, 500, "broken", server.ReasonControl},
	}
	for i, tt := range tests {
		got = nil
		rec := do(t, h, http.MethodGet, tt.target, tt.header)
		if rec.Code != tt.code || len(got) != 1 || got[0].State != tt.state || got[0].Reason != tt.reason {
			t.Errorf("%d: %s %v = %d, reported %+v; want %d %s %s", i+1, tt.target, tt.header, rec.Code, got, tt.code, tt.state, tt.reason)
		}
	}
}

func TestScenario_Unknown(t *testing.T) {
	t.Parallel()

	var got []server.Request
	h, err := server.New(scenarioProject(), server.Options{Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodGet, "/orders", map[string]string{"X-Mock-Scenario": "bad-dya"})
	want := `{"error":"unknown_scenario","scenario":"bad-dya","valid":["bad-day","new-user"],"hint":"did you mean \"bad-day\"?"}` + "\n"
	if rec.Code != 400 || rec.Body.String() != want {
		t.Errorf("unknown scenario = %d %s, want 400 %s", rec.Code, rec.Body, want)
	}
	if diff := cmp.Diff([]server.Request{{Method: "GET", Path: "/orders", Status: 400, Problem: server.ProblemUnknownScenario}}, got, ignoreDuration); diff != "" {
		t.Errorf("reported (-want +got):\n%s", diff)
	}
}
