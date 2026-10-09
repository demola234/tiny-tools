package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func signInProject() *model.Project {
	signedOut := []model.Condition{{Scope: model.ScopeVar, Key: "signed_in", Op: model.OpExists, Value: false}}
	return &model.Project{Routes: []*model.Route{
		{
			ID: "session.create", Method: model.MethodPost, Path: "/session", Active: "ok", Mode: model.ModeRules,
			Rules: []model.Rule{{State: "wrong", When: []model.Condition{{Scope: model.ScopeBody, Key: "password", Op: model.OpNe, Value: "secret"}}}},
			States: model.States{
				{Name: "ok", State: &model.State{Status: 201, Set: []model.Assignment{
					{Name: "signed_in", Value: true}, {Name: "user", Value: "{{ body.email }}"},
				}, Body: jsonBody(`{"hello":"{{ var.user }}"}`)}},
				{Name: "wrong", State: &model.State{Status: 401}},
			},
		},
		{
			ID: "session.delete", Method: model.MethodDelete, Path: "/session", Active: "ok",
			States: model.States{{Name: "ok", State: &model.State{Status: 204, Set: []model.Assignment{{Name: "signed_in", Value: nil}}}}},
		},
		{
			ID: "me.get", Method: model.MethodGet, Path: "/me", Active: "ok", Mode: model.ModeRules,
			Rules:  []model.Rule{{State: "signed_out", When: signedOut}},
			States: model.States{{Name: "ok", State: &model.State{Body: jsonBody(`{"email":"{{ var.user }}"}`)}}, {Name: "signed_out", State: &model.State{Status: 401}}},
		},
	}}
}

func request(t *testing.T, h http.Handler, method, target, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestVars_SignInFlow(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	h, err := server.New(signInProject(), server.Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		method, target, body string
		code                 int
		want                 string
	}{
		{"GET", "/me", "", 401, ""},
		{"POST", "/session", `{"email":"ada@example.com","password":"nope"}`, 401, ""},
		{"GET", "/me", "", 401, ""},
		{"POST", "/session", `{"email":"ada@example.com","password":"secret"}`, 201, `{"hello":"ada@example.com"}`},
		{"GET", "/me", "", 200, `{"email":"ada@example.com"}`},
		{"DELETE", "/session", "", 204, ""},
		{"GET", "/me", "", 401, ""},
	}
	for i, s := range steps {
		code, body := request(t, h, s.method, s.target, s.body)
		if code != s.code || (s.want != "" && body != s.want) {
			t.Errorf("step %d %s %s = %d %s, want %d %s", i+1, s.method, s.target, code, body, s.code, s.want)
		}
	}
	request(t, h, "POST", "/session", `{"email":"b@example.com","password":"secret"}`)
	reloaded, err := server.New(signInProject(), server.Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	if code, body := request(t, reloaded, "GET", "/me", ""); code != 200 || body != `{"email":"b@example.com"}` {
		t.Errorf("after a reload: %d %s; variables should be kept", code, body)
	}
}
