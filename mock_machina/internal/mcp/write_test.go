package mcp_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/mcp"
)

func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		writes := !tool.Annotations.ReadOnlyHint
		if writes != slices.Contains([]string{"add_route", "add_state", "set_state"}, tool.Name) {
			t.Errorf("%s: readOnlyHint %v", tool.Name, tool.Annotations.ReadOnlyHint)
		}
		if writes && (tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint) {
			t.Errorf("%s doesn't say it only adds", tool.Name)
		}
	}
	slices.Sort(names)
	return names
}

func TestTools_ReadOnlyLeavesOutWrites(t *testing.T) {
	t.Parallel()

	dir := shop(t)
	all := toolNames(t, connect(t, dir))
	if want := []string{"add_route", "add_state", "diff", "diff_live", "get_route", "lint", "list_routes", "set_state"}; !slices.Equal(all, want) {
		t.Errorf("tools = %v, want %v", all, want)
	}
	readOnly := toolNames(t, connectWith(t, mcp.Options{Dir: dir, ReadOnly: true}))
	if want := []string{"diff", "diff_live", "get_route", "lint", "list_routes"}; !slices.Equal(readOnly, want) {
		t.Errorf("read-only tools = %v, want %v", readOnly, want)
	}
}

func readRoutes(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "routes", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAddState(t *testing.T) {
	t.Parallel()

	dir := shop(t)
	cs := connect(t, dir)
	res := call(t, cs, "add_state", map[string]any{
		"route": "users.get", "name": "gone", "status": 410,
		"headers": map[string]any{"Retry-After": "60"},
		"latency": "300ms",
		"body":    rawJSON(`{"error":"user deleted","code":"gone"}`),
	})
	if res.IsError {
		t.Fatalf("add_state: %s", text(res))
	}
	want := `added state "gone" to users.get at routes/users.yaml:24, marked generated: true until a person reviews it`
	if text(res) != want {
		t.Errorf("text = %q\nwant %q", text(res), want)
	}
	wantFile := usersYAML + "    gone:\n      status: 410\n      headers: {Retry-After: \"60\"}\n" +
		"      latency: 300ms\n      generated: true\n      body: {error: user deleted, code: gone}\n"
	if got := readRoutes(t, dir, "users.yaml"); got != wantFile {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, wantFile)
	}
	route, _ := call(t, cs, "get_route", map[string]any{"id": "users.get"}).StructuredContent.(map[string]any)
	states, _ := route["states"].([]any)
	if gone, _ := states[len(states)-1].(map[string]any); gone["name"] != "gone" || gone["generated"] != true {
		t.Errorf("get_route doesn't show the new state as generated: %v", gone)
	}
	got, _ := res.StructuredContent.(map[string]any)
	if got["route"] != "users.get" || got["state"] != "gone" || got["file"] != "routes/users.yaml" || got["line"] != 24.0 {
		t.Errorf("structured content = %v", got)
	}
}

func TestAddState_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"unknown route", map[string]any{"route": "users.gte", "name": "x"}, `no route "users.gte" (did you mean "users.get"?)`},
		{"state exists", map[string]any{"route": "users.get", "name": "found"}, `route users.get already has a state "found"`},
		{"bad latency", map[string]any{"route": "users.get", "name": "slow", "latency": "800"}, `latency "800" isn't a duration, like 250ms or 2s`},
		{"unknown argument", map[string]any{"route": "users.get", "name": "x", "shade": "red"}, `invalid arguments: json: unknown field "shade"`},
		{"text body", map[string]any{"route": "users.get", "name": "x", "body": "hello"}, "body must be a JSON object or array"},
	}
	dir := shop(t)
	cs := connect(t, dir)
	for _, tc := range tests {
		wantToolError(t, cs, "add_state", tc.args, tc.want)
	}
	if got := readRoutes(t, dir, "users.yaml"); got != usersYAML {
		t.Errorf("users.yaml changed after refusals:\n%s", got)
	}
}

func TestAddRoute(t *testing.T) {
	t.Parallel()

	dir := shop(t)
	cs := connect(t, dir)
	res := call(t, cs, "add_route", map[string]any{
		"method": "GET", "path": "/orders/{id}", "summary": "One order",
		"states": []any{
			map[string]any{"name": "found", "body": rawJSON(`{"id":"o_1","total":12.5,"items":[]}`)},
			map[string]any{"name": "not_found", "status": 404, "body": rawJSON(`{"error":"no such order"}`)},
		},
	})
	if res.IsError {
		t.Fatalf("add_route: %s", text(res))
	}
	want := "added orders.get (GET /orders/{id}) in a new file, routes/orders.yaml, marked generated: true and status: draft until a person reviews it"
	if text(res) != want {
		t.Errorf("text = %q\nwant %q", text(res), want)
	}
	got := readRoutes(t, dir, "orders.yaml")
	wantTail := "get:\n  route: GET /orders/{id}\n  summary: One order\n  status: draft\n  generated: true\n  states:\n" +
		"    found:\n      body: {id: o_1, total: 12.5, items: []}\n" +
		"    not_found:\n      status: 404\n      body: {error: no such order}\n"
	if !strings.HasSuffix(got, wantTail) {
		t.Errorf("orders.yaml:\n%s\nwant it to end with:\n%s", got, wantTail)
	}
	callGolden(t, cs, "get_route", map[string]any{"id": "orders.get"}, "get_route_added.json")
}

func TestAddRoute_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no states", map[string]any{"method": "GET", "path": "/orders", "states": []any{}}, "states needs at least one state"},
		{"taken", map[string]any{"method": "GET", "path": "/users", "states": []any{map[string]any{"name": "ok"}}}, "GET /users is already defined by route users.list"},
		{"bad state latency", map[string]any{"method": "GET", "path": "/orders", "states": []any{map[string]any{"name": "ok", "latency": "soon"}}}, `latency "soon" isn't a duration, like 250ms or 2s`},
	}
	dir := shop(t)
	cs := connect(t, dir)
	for _, tc := range tests {
		wantToolError(t, cs, "add_route", tc.args, tc.want)
	}
	if _, err := os.Stat(filepath.Join(dir, "routes", "orders.yaml")); err == nil {
		t.Error("a refused add_route created orders.yaml")
	}
}

func TestSetState(t *testing.T) {
	t.Parallel()

	dir := shop(t)
	cs := connect(t, dir)
	res := call(t, cs, "set_state", map[string]any{"route": "users.get", "state": "found"})
	if res.IsError || text(res) != `users.get now serves "found" (was "not_found")` {
		t.Errorf("set_state: isError %v, text %q", res.IsError, text(res))
	}
	if got := readRoutes(t, dir, "users.yaml"); !strings.Contains(got, "  active: found\n") {
		t.Errorf("users.yaml:\n%s", got)
	}
	wantToolError(t, cs, "set_state", map[string]any{"route": "users.get", "state": "fond"},
		`route users.get has no state "fond" (did you mean "found"?)`)
}
