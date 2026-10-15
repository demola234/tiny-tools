package control_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/control"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

const usersYAML = `list:
  route: GET /users
  active: full
  states:
    full:
      body: [{ id: 1 }]
    empty:
      status: 204
`

type fixture struct {
	dir  string
	mem  *server.Memory
	api  *control.API
	mock http.Handler
}

func newFixture(t *testing.T, token string) fixture {
	t.Helper()
	return newFixtureWith(t, token, map[string]string{"routes/users.yaml": usersYAML})
}

func newFixtureWith(t *testing.T, token string, files map[string]string) fixture {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	proj, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 {
		t.Fatalf("load: %v %v", probs, err)
	}
	mem := server.NewMemory()
	api := control.New(proj, control.Options{Dir: dir, Memory: mem, Token: token, Clock: fixedClock{}})
	mock, err := server.New(proj, server.Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{dir: dir, mem: mem, api: api, mock: mock}
}

func (f fixture) call(t *testing.T, method, target, body string, header map[string]string) (int, map[string]any) {
	t.Helper()
	rec := f.raw(t, method, target, body, header)
	var got map[string]any
	if rec.Body.Len() > 0 && !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("%s %s body %q: %v", method, target, rec.Body, err)
		}
	}
	return rec.Code, got
}

func (f fixture) raw(t *testing.T, method, target, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:4001"
	for k, v := range header {
		switch k {
		case "RemoteAddr":
			req.RemoteAddr = v
		case "Host":
			req.Host = v
		default:
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, req)
	return rec
}

func (f fixture) mockStatus(t *testing.T) int {
	t.Helper()
	rec := httptest.NewRecorder()
	f.mock.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/users", nil))
	return rec.Code
}

var listRoute = map[string]any{"id": "users.list", "method": "GET", "path": "/users", "mode": "active", "active": "full", "states": []any{"full", "empty"}}

func with(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{k: v}
	for key, val := range m {
		if key != k {
			out[key] = val
		}
	}
	return out
}

func TestRoutes_List(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/__mockmachina/routes", nil)
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:4001"
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, req)
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != 200 {
		t.Fatalf("GET routes = %d %s", rec.Code, rec.Body)
	}
	if diff := cmp.Diff([]map[string]any{listRoute}, got); diff != "" {
		t.Errorf("routes (-want +got):\n%s", diff)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type %q", ct)
	}
}

func TestRoutes_One(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	code, got := f.call(t, "GET", "/__mockmachina/routes/users.list", "", nil)
	if diff := cmp.Diff(listRoute, got); code != 200 || diff != "" {
		t.Errorf("GET route = %d (-want +got):\n%s", code, diff)
	}
	code, got = f.call(t, "GET", "/__mockmachina/routes/users.lst", "", nil)
	want := map[string]any{"error": "unknown_route", "route": "users.lst", "hint": `did you mean "users.list"?`}
	if diff := cmp.Diff(want, got); code != 404 || diff != "" {
		t.Errorf("unknown route = %d (-want +got):\n%s", code, diff)
	}
}

func TestRoutes_Swap(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	f.api.Swap(&model.Project{Routes: []*model.Route{{ID: "health.get", Method: model.MethodGet, Path: "/health", Mode: model.ModeActive, Active: "ok", States: model.States{{Name: "ok", State: &model.State{}}}}}})
	if code, _ := f.call(t, "GET", "/__mockmachina/routes/health.get", "", nil); code != 200 {
		t.Errorf("after Swap, GET health.get = %d", code)
	}
}

func TestPin(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	code, got := f.call(t, "PUT", "/__mockmachina/routes/users.list/state", `{"state":"empty"}`, nil)
	if diff := cmp.Diff(with(listRoute, "pinned", "empty"), got); code != 200 || diff != "" {
		t.Errorf("PUT state = %d (-want +got):\n%s", code, diff)
	}
	if s := f.mockStatus(t); s != 204 {
		t.Errorf("mock after pin = %d, want 204", s)
	}
	code, got = f.call(t, "DELETE", "/__mockmachina/routes/users.list/state", "", nil)
	if diff := cmp.Diff(listRoute, got); code != 200 || diff != "" {
		t.Errorf("DELETE state = %d (-want +got):\n%s", code, diff)
	}
	if s := f.mockStatus(t); s != 200 {
		t.Errorf("mock after unpin = %d, want 200", s)
	}
}

func TestPin_Errors(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	tests := []struct {
		name, target, body string
		code               int
		want               map[string]any
	}{
		{"unknown state", "/__mockmachina/routes/users.list/state", `{"state":"emty"}`, 400, map[string]any{"error": "unknown_state", "route": "users.list", "state": "emty", "valid": []any{"full", "empty"}, "hint": `did you mean "empty"?`}},
		{"no state", "/__mockmachina/routes/users.list/state", `{}`, 400, map[string]any{"error": "bad_request", "detail": `send {"state": "<name>"}`}},
		{"not JSON", "/__mockmachina/routes/users.list/state", `state=empty`, 400, map[string]any{"error": "bad_request", "detail": `send {"state": "<name>"}`}},
		{"unknown route", "/__mockmachina/routes/orders/state", `{"state":"empty"}`, 404, map[string]any{"error": "unknown_route", "route": "orders"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, got := f.call(t, "PUT", tt.target, tt.body, nil)
			if diff := cmp.Diff(tt.want, got); code != tt.code || diff != "" {
				t.Errorf("code %d, want %d (-want +got):\n%s", code, tt.code, diff)
			}
		})
	}
	if _, ok := f.mem.Pinned("users.list"); ok {
		t.Error("a failed PUT pinned the route")
	}
}

func TestPin_Persist(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	f.mem.Pin("users.list", "full")
	code, got := f.call(t, "PUT", "/__mockmachina/routes/users.list/state", `{"state":"empty","persist":true}`, nil)
	if diff := cmp.Diff(with(listRoute, "active", "empty"), got); code != 200 || diff != "" {
		t.Errorf("PUT persist = %d (-want +got):\n%s", code, diff)
	}
	data, _ := os.ReadFile(filepath.Join(f.dir, "routes", "users.yaml"))
	if !strings.Contains(string(data), "active: empty") {
		t.Errorf("file not updated:\n%s", data)
	}
	if _, ok := f.mem.Pinned("users.list"); ok {
		t.Error("persist left the old pin in place")
	}
}

func TestReset(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	f.mem.Pin("users.list", "empty")
	if code, _ := f.call(t, "POST", "/__mockmachina/reset", "", nil); code != 204 {
		t.Errorf("POST reset = %d, want 204", code)
	}
	if _, ok := f.mem.Pinned("users.list"); ok {
		t.Error("reset kept the pin")
	}
}

func TestUnknownEndpoint(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	code, got := f.call(t, "GET", "/__mockmachina/nope", "", nil)
	want := map[string]any{"error": "not_found", "endpoints": []any{
		"GET /__mockmachina/routes", "GET /__mockmachina/routes/{id}", "PUT /__mockmachina/routes/{id}/state",
		"DELETE /__mockmachina/routes/{id}/state", "POST /__mockmachina/reset", "GET /__mockmachina/events",
		"GET /__mockmachina/requests", "GET /__mockmachina/coverage",
		"GET /__mockmachina/collections", "GET /__mockmachina/collections/{name}", "DELETE /__mockmachina/collections/{name}/{id}",
		"GET /__mockmachina/scenarios", "POST /__mockmachina/scenarios/{name}/apply",
	}}
	if diff := cmp.Diff(want, got); code != 404 || diff != "" {
		t.Errorf("code %d (-want +got):\n%s", code, diff)
	}
}

func TestAccess(t *testing.T) {
	t.Parallel()

	const remote = "192.168.1.20:50000"
	tests := []struct {
		name   string
		token  string
		header map[string]string
		code   int
	}{
		{"this machine", "", nil, 200},
		{"this machine over IPv6", "", map[string]string{"RemoteAddr": "[::1]:50000"}, 200},
		{"another machine, no token configured", "", map[string]string{"RemoteAddr": remote}, 403},
		{"another machine, no token sent", "s3cret", map[string]string{"RemoteAddr": remote}, 401},
		{"another machine, wrong token", "s3cret", map[string]string{"RemoteAddr": remote, "Authorization": "Bearer nope"}, 401},
		{"another machine, right token", "s3cret", map[string]string{"RemoteAddr": remote, "Authorization": "Bearer s3cret"}, 200},
		{"another site in a browser", "", map[string]string{"Origin": "https://evil.example"}, 403},
		{"the same origin", "", map[string]string{"Origin": "http://127.0.0.1:4001"}, 200},
		{"this machine as localhost", "", map[string]string{"Host": "localhost:4001", "Origin": "http://localhost:4001"}, 200},
		{"this machine as [::1]", "", map[string]string{"Host": "[::1]:4001"}, 200},
		{"this machine with no port in Host", "", map[string]string{"Host": "localhost"}, 200},
		{"a site whose name points at this machine", "", map[string]string{"Host": "evil.example:4001", "Origin": "http://evil.example:4001"}, 403},
		{"a site whose name points at this machine, token set", "s3cret", map[string]string{"Host": "evil.example:4001"}, 401},
		{"a site whose name points at this machine, right token", "s3cret", map[string]string{"Host": "evil.example:4001", "Authorization": "Bearer s3cret"}, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, tt.token)
			if code, _ := f.call(t, "GET", "/__mockmachina/routes/users.list", "", tt.header); code != tt.code {
				t.Errorf("code %d, want %d", code, tt.code)
			}
		})
	}
}

func subscribe(t *testing.T, f fixture) *bufio.Scanner {
	t.Helper()
	srv := httptest.NewServer(f.api)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/__mockmachina/events", nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type %q", ct)
	}
	return bufio.NewScanner(resp.Body)
}

func nextEvent(lines *bufio.Scanner) (event, data string) {
	for data == "" && lines.Scan() {
		line := lines.Text()
		if v, ok := strings.CutPrefix(line, "event: "); ok {
			event = v
		}
		if v, ok := strings.CutPrefix(line, "data: "); ok {
			data = v
		}
	}
	return event, data
}

func TestEvents(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	lines := subscribe(t, f)
	f.api.Publish(server.Request{Method: "GET", Path: "/users", Status: 204, Route: "users.list", State: "empty", Reason: server.ReasonControl, Duration: 120 * time.Millisecond})
	event, data := nextEvent(lines)
	var got map[string]any
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("data %q: %v", data, err)
	}
	if _, ok := got["time"].(string); !ok {
		t.Errorf("event has no time: %v", got)
	}
	delete(got, "time")
	want := map[string]any{"method": "GET", "path": "/users", "status": float64(204), "route": "users.list", "state": "empty", "reason": "control", "duration_ms": float64(120)}
	if diff := cmp.Diff(want, got); event != "request" || diff != "" {
		t.Errorf("event %q (-want +got):\n%s", event, diff)
	}
	f.api.Close()
	for lines.Scan() {
	}
}

func TestEvents_Reload(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	lines := subscribe(t, f)
	f.api.Swap(&model.Project{})
	if event, data := nextEvent(lines); event != "reload" || data != "{}" {
		t.Errorf("after Swap, event %q data %q, want reload {}", event, data)
	}
}

func TestRequests(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	f.api.Publish(server.Request{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "full", Reason: server.ReasonActive})
	f.api.Publish(server.Request{Method: "GET", Path: "/nope", Status: 404, Problem: server.ProblemNoRoute})
	code, got := f.call(t, "GET", "/__mockmachina/requests", "", nil)
	requests, _ := got["requests"].([]any)
	if code != 200 || got["total"] != float64(2) || got["errors"] != float64(1) || len(requests) != 2 {
		t.Fatalf("GET requests = %d %v", code, got)
	}
	if newest, _ := requests[0].(map[string]any); newest["path"] != "/nope" {
		t.Errorf("first request %v, want the newest, /nope", newest)
	}

	if code, _ := f.call(t, "POST", "/__mockmachina/reset", "", nil); code != 204 {
		t.Fatalf("reset = %d", code)
	}
	_, got = f.call(t, "GET", "/__mockmachina/requests", "", nil)
	if diff := cmp.Diff(map[string]any{"total": float64(0), "errors": float64(0), "requests": []any{}}, got); diff != "" {
		t.Errorf("after reset (-want +got):\n%s", diff)
	}
}

func TestCoverage(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	_, got := f.call(t, "GET", "/__mockmachina/coverage", "", nil)
	want := map[string]any{"served": float64(0), "total": float64(2), "percent": float64(0), "routes": []any{
		map[string]any{"id": "users.list", "method": "GET", "path": "/users", "served": []any{}, "unserved": []any{"full", "empty"}},
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("before any request (-want +got):\n%s", diff)
	}

	f.mockStatus(t)
	code, got := f.call(t, "GET", "/__mockmachina/coverage", "", nil)
	want = map[string]any{"served": float64(1), "total": float64(2), "percent": float64(50), "routes": []any{
		map[string]any{"id": "users.list", "method": "GET", "path": "/users", "served": []any{"full"}, "unserved": []any{"empty"}},
	}}
	if diff := cmp.Diff(want, got); code != 200 || diff != "" {
		t.Errorf("after GET /users = %d (-want +got):\n%s", code, diff)
	}
}

const cartYAML = `items:
  route: CRUD /cart
  crud: { collection: cart }
  states:
    ok: {}
`

func TestCollections(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, "", map[string]string{
		"routes/cart.yaml": cartYAML,
		"data/cart.json":   `[{"id":"c_1","sku":"TEE"},{"id":"c_2","sku":"SCARF"}]`,
	})
	summary := map[string]any{"name": "cart", "id_field": "id", "seed": "data/cart.json", "routes": []any{"CRUD /cart"}, "count": float64(2)}
	code, got := f.call(t, "GET", "/__mockmachina/collections", "", nil)
	if diff := cmp.Diff(map[string]any{"collections": []any{summary}}, got); code != 200 || diff != "" {
		t.Errorf("GET collections = %d (-want +got):\n%s", code, diff)
	}

	if code, _ := f.call(t, "DELETE", "/__mockmachina/collections/cart/c_1", "", nil); code != 204 {
		t.Errorf("DELETE c_1 = %d, want 204", code)
	}
	code, got = f.call(t, "GET", "/__mockmachina/collections/cart", "", nil)
	want := with(with(summary, "count", float64(1)), "items", []any{map[string]any{"id": "c_2", "sku": "SCARF"}})
	if diff := cmp.Diff(want, got); code != 200 || diff != "" {
		t.Errorf("GET collections/cart = %d (-want +got):\n%s", code, diff)
	}

	code, got = f.call(t, "DELETE", "/__mockmachina/collections/cart/c_1", "", nil)
	if diff := cmp.Diff(map[string]any{"error": "unknown_item", "collection": "cart", "id": "c_1"}, got); code != 404 || diff != "" {
		t.Errorf("DELETE c_1 again = %d (-want +got):\n%s", code, diff)
	}
	code, got = f.call(t, "GET", "/__mockmachina/collections/carts", "", nil)
	if diff := cmp.Diff(map[string]any{"error": "unknown_collection", "collection": "carts", "hint": `did you mean "cart"?`}, got); code != 404 || diff != "" {
		t.Errorf("GET collections/carts = %d (-want +got):\n%s", code, diff)
	}
}

func TestCollections_None(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	if _, got := f.call(t, "GET", "/__mockmachina/collections", "", nil); fmt.Sprint(got["collections"]) != "[]" {
		t.Errorf("collections %v, want []", got)
	}
}

func TestScenarios(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, "", map[string]string{
		"routes/users.yaml":       usersYAML,
		"scenarios/new-user.yaml": "title: New user\ndescription: Nothing yet.\nstates:\n  users.list: empty\n",
	})
	newUser := map[string]any{"name": "new-user", "title": "New user", "description": "Nothing yet.", "states": []any{map[string]any{"route": "users.list", "state": "empty"}}}
	code, got := f.call(t, "GET", "/__mockmachina/scenarios", "", nil)
	if diff := cmp.Diff(map[string]any{"scenarios": []any{newUser}}, got); code != 200 || diff != "" {
		t.Errorf("GET scenarios = %d (-want +got):\n%s", code, diff)
	}

	if code, _ := f.call(t, "PUT", "/__mockmachina/routes/users.list/state", `{"state":"full"}`, nil); code != 200 {
		t.Fatalf("pin = %d", code)
	}
	code, got = f.call(t, "POST", "/__mockmachina/scenarios/new-user/apply", "", nil)
	if diff := cmp.Diff(map[string]any{"applied": "new-user", "scenarios": []any{newUser}}, got); code != 200 || diff != "" {
		t.Errorf("apply = %d (-want +got):\n%s", code, diff)
	}
	if code := f.mockStatus(t); code != 204 {
		t.Errorf("GET /users after apply = %d, want 204 (empty)", code)
	}

	code, got = f.call(t, "POST", "/__mockmachina/scenarios/new-usr/apply", "", nil)
	if diff := cmp.Diff(map[string]any{"error": "unknown_scenario", "scenario": "new-usr", "hint": `did you mean "new-user"?`}, got); code != 404 || diff != "" {
		t.Errorf("apply typo = %d (-want +got):\n%s", code, diff)
	}

	f.call(t, "POST", "/__mockmachina/reset", "", nil)
	if _, got := f.call(t, "GET", "/__mockmachina/scenarios", "", nil); got["applied"] != nil {
		t.Errorf("after reset, applied = %v", got["applied"])
	}
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 10, 14, 9, 12, 3, 512e6, time.FixedZone("WAT", 3600))
}

func (fixedClock) Sleep(context.Context, time.Duration) error { return nil }
