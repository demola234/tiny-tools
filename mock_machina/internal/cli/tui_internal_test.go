package cli

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func TestScreenRequest(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	base := server.Request{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "success", Reason: server.ReasonActive}
	with := func(f func(*server.Request)) server.Request {
		r := base
		f(&r)
		return r
	}
	tests := []struct {
		name string
		in   server.Request
		note string
	}{
		{"active state", base, ""},
		{"rule", with(func(r *server.Request) { r.Reason, r.Rule = server.ReasonRule, 2 }), "rule 2"},
		{"header", with(func(r *server.Request) { r.Reason = server.ReasonHeader }), "X-Mock-State header"},
		{"query", with(func(r *server.Request) { r.Reason = server.ReasonQuery }), "?__state"},
		{"sequential", with(func(r *server.Request) { r.Reason = server.ReasonSequential }), "sequential"},
		{"fault", with(func(r *server.Request) { r.Fault = model.FaultType("reset") }), "fault: reset"},
		{"invalid request", with(func(r *server.Request) {
			r.Problem, r.Detail = server.ProblemInvalidRequest, "body.name is required"
		}), "invalid request: body.name is required"},
		{"proxied", with(func(r *server.Request) { r.Proxied, r.Route, r.State = true, "", "" }), "proxied"},
		{"no route", with(func(r *server.Request) {
			r.Route, r.State, r.Status, r.Problem, r.Suggestion = "", "", 404, server.ProblemNoRoute, "GET /users"
		}), "no route (closest: GET /users)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := screenRequest(tt.in, at)
			want := tui.Request{Time: at, Method: tt.in.Method, Path: tt.in.Path, Status: tt.in.Status, Route: tt.in.Route, State: tt.in.State, Note: tt.note}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
