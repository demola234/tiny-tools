package server_test

import (
	"net/http"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func coverageProject() *model.Project {
	return &model.Project{Routes: []*model.Route{{
		ID:     "users.list",
		Method: model.MethodGet,
		Path:   "/users",
		Active: "full",
		States: model.States{
			{Name: "full", State: &model.State{Body: jsonBody(`[1]`)}},
			{Name: "empty", State: &model.State{Body: jsonBody(`[]`)}},
			{Name: "broken", State: &model.State{Status: 500}},
		},
	}}}
}

func TestCoverage_RecordsServedStates(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	h, err := server.New(coverageProject(), server.Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	do(t, h, http.MethodGet, "/users", nil)
	do(t, h, http.MethodGet, "/users", map[string]string{"X-Mock-State": "broken"})
	do(t, h, http.MethodGet, "/users", map[string]string{"X-Mock-State": "nope"})
	do(t, h, http.MethodGet, "/nowhere", nil)

	for state, want := range map[string]bool{"full": true, "broken": true, "empty": false, "nope": false} {
		if got := mem.Served("users.list", state); got != want {
			t.Errorf("Served(users.list, %s) = %v, want %v", state, got, want)
		}
	}
	mem.Reset()
	if mem.Served("users.list", "full") {
		t.Error("reset kept coverage")
	}
}
