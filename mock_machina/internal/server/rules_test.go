package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func eq(scope model.Scope, key string, v any) model.Condition {
	return model.Condition{Scope: scope, Key: key, Op: model.OpEq, Value: v}
}

func rulesProject() *model.Project {
	status := func(code int) *model.State { return &model.State{Status: code} }
	return &model.Project{Routes: []*model.Route{
		{
			ID: "users.get", Method: model.MethodGet, Path: "/users/{id}", Active: "found", Mode: model.ModeRules,
			Rules: []model.Rule{
				{State: "not_found", When: []model.Condition{eq(model.ScopePath, "id", "u_404")}},
				{State: "unauthorized", When: []model.Condition{{Scope: model.ScopeHeader, Key: "authorization", Op: model.OpExists, Value: false}}},
				{State: "expired", When: []model.Condition{eq(model.ScopeCookie, "session", "old")}},
			},
			States: model.States{{Name: "found", State: status(200)}, {Name: "not_found", State: status(404)}, {Name: "unauthorized", State: status(401)}, {Name: "expired", State: status(419)}},
		},
		{
			ID: "session.create", Method: model.MethodPost, Path: "/session", Active: "ok", Mode: model.ModeRules,
			Rules: []model.Rule{
				{State: "wrong", When: []model.Condition{{Scope: model.ScopeBody, Key: "password", Op: model.OpNe, Value: "secret"}}},
			},
			States: model.States{{Name: "ok", State: status(201)}, {Name: "wrong", State: status(401)}},
		},
		{
			ID: "retry.get", Method: model.MethodGet, Path: "/retry", Active: "ok", Mode: model.ModeRules,
			Rules: []model.Rule{
				{State: "down", When: []model.Condition{{Scope: model.ScopeCall, Op: model.OpLte, Value: 2.0}}},
			},
			States: model.States{{Name: "ok", State: status(200)}, {Name: "down", State: status(503)}},
		},
	}}
}

func send(t *testing.T, h http.Handler, method, target, body string, header map[string]string) int {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRules_PickTheState(t *testing.T) {
	t.Parallel()

	var got []server.Request
	h, err := server.New(rulesProject(), server.Options{Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatal(err)
	}
	auth := map[string]string{"Authorization": "Bearer t"}
	tests := []struct {
		name, method, target, body string
		header                     map[string]string
		want                       int
	}{
		{"first rule", "GET", "/users/u_404", "", auth, 404},
		{"second rule", "GET", "/users/u_1", "", nil, 401},
		{"cookie rule", "GET", "/users/u_1", "", map[string]string{"Authorization": "x", "Cookie": "session=old"}, 419},
		{"no rule matches", "GET", "/users/u_1", "", auth, 200},
		{"header beats rules", "GET", "/users/u_404", "", map[string]string{"X-Mock-State": "found"}, 200},
		{"query beats rules", "GET", "/users/u_404?__state=found", "", nil, 200},
		{"body rule", "POST", "/session", `{"password":"nope"}`, nil, 401},
		{"body rule not matched", "POST", "/session", `{"password":"secret"}`, nil, 201},
		{"body that isn't JSON", "POST", "/session", `password=secret`, nil, 401},
		{"call 1", "GET", "/retry", "", nil, 503},
		{"call 2", "GET", "/retry", "", nil, 503},
		{"call 3", "GET", "/retry", "", nil, 200},
	}
	for _, tc := range tests {
		if code := send(t, h, tc.method, tc.target, tc.body, tc.header); code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, code, tc.want)
		}
	}
	want := []server.Request{
		{Method: "GET", Path: "/users/u_404", Status: 404, Route: "users.get", State: "not_found", Reason: server.ReasonRule, Rule: 1},
		{Method: "GET", Path: "/users/u_1", Status: 401, Route: "users.get", State: "unauthorized", Reason: server.ReasonRule, Rule: 2},
		{Method: "GET", Path: "/users/u_1", Status: 419, Route: "users.get", State: "expired", Reason: server.ReasonRule, Rule: 3},
		{Method: "GET", Path: "/users/u_1", Status: 200, Route: "users.get", State: "found", Reason: server.ReasonActive},
	}
	if diff := cmp.Diff(want, got[:4]); diff != "" {
		t.Errorf("reported (-want +got):\n%s", diff)
	}
}

func TestRules_CallsAreKeptInMemory(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	for _, want := range []int{503, 503, 200} {
		h, err := server.New(rulesProject(), server.Options{Memory: mem})
		if err != nil {
			t.Fatal(err)
		}
		if code := send(t, h, "GET", "/retry", "", nil); code != want {
			t.Errorf("after a reload: %d, want %d", code, want)
		}
	}
}
