package cli

import (
	"net/http"
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

func TestWebhookLines(t *testing.T) {
	ok := server.Request{Method: "POST", Path: "http://localhost:3000/events", Status: 200, Route: "payments.create", State: "pending", Webhook: true}
	if got := formatRequest(ok); got != "webhook POST http://localhost:3000/events 200 (from payments.create:pending)" {
		t.Errorf("formatRequest = %q", got)
	}
	down := server.Request{Method: "POST", Path: "http://localhost:3000/events", Route: "payments.create", State: "pending", Webhook: true, Problem: server.ProblemBackend, Detail: "connection refused"}
	if got := formatRequest(down); got != "webhook POST http://localhost:3000/events failed (from payments.create:pending): connection refused" {
		t.Errorf("formatRequest = %q", got)
	}
	if got := screenRequest(ok, time.Time{}); got.Note != "webhook" || got.Method != http.MethodPost {
		t.Errorf("screenRequest = %+v", got)
	}
}

func TestGRPCLines(t *testing.T) {
	r := server.Request{Method: "GRPC", Path: "shop.v1.OrderService/GetOrder", Route: "orders.get", State: "not_found", Reason: server.ReasonRule, Rule: 1, GRPCCode: "NOT_FOUND"}
	if got := formatRequest(r); got != "GRPC shop.v1.OrderService/GetOrder NOT_FOUND orders.get:not_found (rule 1)" {
		t.Errorf("formatRequest = %q", got)
	}
}
