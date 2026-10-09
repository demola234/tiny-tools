package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func usersProject() *model.Project {
	return &model.Project{Routes: []*model.Route{
		{
			ID:     "users.get",
			Method: model.MethodGet,
			Path:   "/users/{id}",
			Active: "found",
			States: model.States{
				{Name: "found", State: &model.State{Body: jsonBody(`{"id":"u_1"}`)}},
			},
		},
		{
			ID:     "users.list",
			Method: model.MethodGet,
			Path:   "/users",
			Active: "success",
			States: model.States{
				{Name: "success", State: &model.State{Body: jsonBody(`{"users":[{"id":"u_1"}]}`)}},
				{Name: "empty", State: &model.State{Body: jsonBody(`{"users":[]}`)}},
				{Name: "unauthorized", State: &model.State{
					Status:  401,
					Headers: map[string]string{"WWW-Authenticate": "Bearer"},
					Body:    jsonBody(`{"error":"expired"}`),
				}},
				{Name: "plain", State: &model.State{
					Headers: map[string]string{"Content-Type": "text/plain"},
					Body:    jsonBody(`hello`),
				}},
			},
		},
		{
			ID:     "users.delete",
			Method: model.MethodDelete,
			Path:   "/users/{id}",
			Active: "deleted",
			States: model.States{
				{Name: "deleted", State: &model.State{Status: 204}},
			},
		},
	}}
}

func jsonBody(s string) model.Body {
	return model.Body{Data: []byte(s), ContentType: "application/json"}
}

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := server.New(usersProject(), server.Options{})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	return h
}

func do(t *testing.T, h http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServer_ServesActiveState(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodGet, "/users", nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != `{"users":[{"id":"u_1"}]}` {
		t.Errorf("body = %q", got)
	}
	wantHeaders := map[string]string{
		"Content-Type": "application/json",
		"X-Mock-State": "success",
		"X-Mock-Route": "users.list",
	}
	for k, want := range wantHeaders {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
}

func TestServer_StatePrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		target    string
		header    map[string]string
		wantState string
	}{
		{"active when nothing asks", "/users", nil, "success"},
		{"query parameter", "/users?__state=empty", nil, "empty"},
		{"header", "/users", map[string]string{"X-Mock-State": "unauthorized"}, "unauthorized"},
		{"header beats query", "/users?__state=empty", map[string]string{"X-Mock-State": "unauthorized"}, "unauthorized"},
		{"other query parameters ignored", "/users?page=2", nil, "success"},
	}
	h := newHandler(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, h, http.MethodGet, tc.target, tc.header)
			if got := rec.Header().Get("X-Mock-State"); got != tc.wantState {
				t.Errorf("X-Mock-State = %q, want %q", got, tc.wantState)
			}
		})
	}
}

func TestServer_StateStatusHeadersAndBody(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodGet, "/users", map[string]string{"X-Mock-State": "unauthorized"})

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
	if got := rec.Body.String(); got != `{"error":"expired"}` {
		t.Errorf("body = %q", got)
	}
}

func TestServer_StateHeadersOverrideContentType(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodGet, "/users", map[string]string{"X-Mock-State": "plain"})
	if got := rec.Header().Get("Content-Type"); got != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", got)
	}
}

func TestServer_NoBody(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodDelete, "/users/42", nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Errorf("Content-Type = %q, want none", got)
	}
}

func TestServer_PathParametersAndMethods(t *testing.T) {
	t.Parallel()

	h := newHandler(t)
	if got := do(t, h, http.MethodGet, "/users/42", nil).Header().Get("X-Mock-Route"); got != "users.get" {
		t.Errorf("GET /users/42 served by %q, want users.get", got)
	}
	if got := do(t, h, http.MethodDelete, "/users/42", nil).Header().Get("X-Mock-Route"); got != "users.delete" {
		t.Errorf("DELETE /users/42 served by %q, want users.delete", got)
	}
}

func TestServer_UnknownState(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodGet, "/users", map[string]string{"X-Mock-State": "missing"})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	want := map[string]any{
		"error": "unknown_state",
		"route": "users.list",
		"state": "missing",
		"valid": []any{"success", "empty", "unauthorized", "plain"},
	}
	checkJSON(t, rec, want)
}

func TestServer_NoRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method, path string
		closest      []any
	}{
		{http.MethodGet, "/usrs", []any{"GET /users", "GET /users/{id}", "DELETE /users/{id}"}},
		{http.MethodPost, "/users", []any{"GET /users", "DELETE /users/{id}", "GET /users/{id}"}},
		{http.MethodGet, "/", []any{"GET /users", "GET /users/{id}", "DELETE /users/{id}"}},
	}
	h := newHandler(t)
	for _, tc := range tests {
		rec := do(t, h, tc.method, tc.path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", tc.method, tc.path, rec.Code)
		}
		checkJSON(t, rec, map[string]any{"error": "no_route", "method": tc.method, "path": tc.path, "closest": tc.closest})
	}
}

func TestServer_NoRouteInAnEmptyProject(t *testing.T) {
	t.Parallel()

	h, err := server.New(&model.Project{}, server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	checkJSON(t, do(t, h, http.MethodGet, "/x", nil), map[string]any{"error": "no_route", "method": "GET", "path": "/x"})
}

func TestServer_UnknownStateHint(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodGet, "/users", map[string]string{"X-Mock-State": "empt"})
	checkJSON(t, rec, map[string]any{
		"error": "unknown_state",
		"route": "users.list",
		"state": "empt",
		"valid": []any{"success", "empty", "unauthorized", "plain"},
		"hint":  `did you mean "empty"?`,
	})
}

func TestServer_ReportsEachRequest(t *testing.T) {
	t.Parallel()

	var got []server.Request
	h, err := server.New(usersProject(), server.Options{Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	do(t, h, http.MethodGet, "/users", nil)
	do(t, h, http.MethodGet, "/users?__state=empty", nil)
	do(t, h, http.MethodGet, "/users", map[string]string{"X-Mock-State": "plain"})
	do(t, h, http.MethodGet, "/users", map[string]string{"X-Mock-State": "nope"})
	do(t, h, http.MethodGet, "/usrs", nil)

	want := []server.Request{
		{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "success", Reason: server.ReasonActive},
		{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "empty", Reason: server.ReasonQuery},
		{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "plain", Reason: server.ReasonHeader},
		{Method: "GET", Path: "/users", Status: 400, Route: "users.list", State: "nope", Reason: server.ReasonHeader, Problem: server.ProblemUnknownState},
		{Method: "GET", Path: "/usrs", Status: 404, Problem: server.ProblemNoRoute, Suggestion: "GET /users"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("reported requests (-want +got):\n%s", diff)
	}
}

func TestNew_ConflictingRoutesIsAnError(t *testing.T) {
	t.Parallel()

	p := &model.Project{Routes: []*model.Route{
		{ID: "a.get", Method: model.MethodGet, Path: "/a/{x}", Active: "ok", States: model.States{{Name: "ok", State: &model.State{}}}},
		{ID: "b.get", Method: model.MethodGet, Path: "/{y}/b", Active: "ok", States: model.States{{Name: "ok", State: &model.State{}}}},
	}}
	_, err := server.New(p, server.Options{})
	if err == nil {
		t.Fatal("server.New with conflicting routes returned no error")
	}
	for _, want := range []string{"a.get", "b.get"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %s", err, want)
		}
	}
}

func TestNew_InvalidPathIsAnError(t *testing.T) {
	t.Parallel()

	p := &model.Project{Routes: []*model.Route{
		{ID: "bad.get", Method: model.MethodGet, Path: "/users/{id", Active: "ok", States: model.States{{Name: "ok", State: &model.State{}}}},
	}}
	_, err := server.New(p, server.Options{})
	if err == nil || !strings.Contains(err.Error(), "bad.get") {
		t.Errorf("server.New with an invalid path: error = %v, want one naming bad.get", err)
	}
}

func checkJSON(t *testing.T, rec *httptest.ResponseRecorder, want map[string]any) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("JSON body (-want +got):\n%s", diff)
	}
}

func TestServer_NoMockHeaders(t *testing.T) {
	t.Parallel()

	h, err := server.New(usersProject(), server.Options{NoMockHeaders: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodGet, "/users", nil)
	for _, k := range []string{"X-Mock-State", "X-Mock-Route"} {
		if got := rec.Header().Get(k); got != "" {
			t.Errorf("%s = %q with NoMockHeaders, want none", k, got)
		}
	}
	if rec.Header().Get("Content-Type") != "application/json" || rec.Code != http.StatusOK {
		t.Errorf("response changed beyond the mock headers: %d %v", rec.Code, rec.Header())
	}
}
