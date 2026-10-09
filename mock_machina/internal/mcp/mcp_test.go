package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/mcp"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const usersYAML = `owners: { backend: [ademola] }

list:
  route: GET /users
  summary: List users
  states:
    success:
      body: { users: [{ id: u_1 }] }
    empty:
      body: { users: [] }

get:
  route: GET /users/{id}
  examples: { id: u_1 }
  active: not_found
  states:
    found:
      headers: { X-Trace: abc }
      latency: 200ms
      body: { id: u_1, name: Ada }
    not_found:
      status: 404
      body: { error: user not found }
`

const healthYAML = `get:
  route: GET /health
  status: agreed
  states:
    up:
      body: ok.txt
`

func project(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func shop(t *testing.T) string {
	t.Helper()
	return project(t, map[string]string{
		"routes/users.yaml":    usersYAML,
		"routes/health.yaml":   healthYAML,
		"routes/health/ok.txt": "ok",
	})
}

func connect(t *testing.T, dir string) *sdk.ClientSession {
	t.Helper()
	return connectWith(t, mcp.Options{Dir: dir})
}

func connectWith(t *testing.T, opts mcp.Options) *sdk.ClientSession {
	t.Helper()
	ctx := t.Context()
	opts.Version = "v0.0.0-test"
	serverEnd, clientEnd := sdk.NewInMemoryTransports()
	if _, err := mcp.New(opts).Connect(ctx, serverEnd, nil); err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, clientEnd, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *sdk.ClientSession, tool string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	return res
}

func rawJSON(s string) json.RawMessage { return json.RawMessage(s) }

func indent(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func callGolden(t *testing.T, cs *sdk.ClientSession, tool string, args map[string]any, golden string) {
	t.Helper()
	res := call(t, cs, tool, args)
	if res.IsError {
		t.Fatalf("%s returned an error: %s", tool, text(res))
	}
	testkit.Golden(t, indent(t, res.StructuredContent), "mcp/"+golden)
}

func text(res *sdk.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func wantToolError(t *testing.T, cs *sdk.ClientSession, tool string, args map[string]any, want string) {
	t.Helper()
	res := call(t, cs, tool, args)
	if !res.IsError || text(res) != want {
		t.Errorf("%s(%v): isError %v, text %q; want an error %q", tool, args, res.IsError, text(res), want)
	}
}

func TestTools_ListedWithSchemas(t *testing.T) {
	t.Parallel()

	res, err := connect(t, shop(t)).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	testkit.Golden(t, indent(t, res.Tools), "mcp/tools.json")
}

func TestServer_IntroducesItself(t *testing.T) {
	t.Parallel()

	init := connect(t, shop(t)).InitializeResult()
	if init.ServerInfo.Name != "mockmachina" || init.ServerInfo.Version != "v0.0.0-test" {
		t.Errorf("server info = %+v", init.ServerInfo)
	}
	if !strings.Contains(init.Instructions, ".mockmachina") {
		t.Errorf("instructions = %q; want them to explain the project folder", init.Instructions)
	}
}

func TestListRoutes(t *testing.T) {
	t.Parallel()

	cs := connect(t, shop(t))
	callGolden(t, cs, "list_routes", nil, "list_routes.json")
	callGolden(t, cs, "list_routes", map[string]any{"resource": "health"}, "list_routes_health.json")
	wantToolError(t, cs, "list_routes", map[string]any{"resource": "usr"}, `no resource "usr" (did you mean "users"?)`)
	wantToolError(t, cs, "list_routes", map[string]any{"resource": "zzzzzz"}, `no resource "zzzzzz" (resources: health, users)`)
}

func TestListRoutes_CountsErrors(t *testing.T) {
	t.Parallel()

	cs := connect(t, project(t, map[string]string{
		"routes/users.yaml":  usersYAML,
		"routes/orders.yaml": "list:\n  route: get /orders\n  states:\n    ok: {}\n",
	}))
	res := call(t, cs, "list_routes", nil)
	got, _ := res.StructuredContent.(map[string]any)
	if got["errors"] != 1.0 {
		t.Errorf("errors = %v, want 1", got["errors"])
	}
}

func TestGetRoute(t *testing.T) {
	t.Parallel()

	cs := connect(t, shop(t))
	callGolden(t, cs, "get_route", map[string]any{"id": "users.get"}, "get_route_users.json")
	callGolden(t, cs, "get_route", map[string]any{"id": "health.get"}, "get_route_health.json")
	wantToolError(t, cs, "get_route", map[string]any{"id": "users.gte"}, `no route "users.gte" (did you mean "users.get"?)`)
}

func TestGetRoute_LeavesOutLargeBodies(t *testing.T) {
	t.Parallel()

	cs := connect(t, project(t, map[string]string{
		"routes/health.yaml":    "get:\n  route: GET /health\n  states:\n    up:\n      body: big.txt\n",
		"routes/health/big.txt": strings.Repeat("x", 40<<10),
	}))
	callGolden(t, cs, "get_route", map[string]any{"id": "health.get"}, "get_route_large.json")
}

func TestLint(t *testing.T) {
	t.Parallel()

	cs := connect(t, project(t, map[string]string{
		"routes/users.yaml":  usersYAML,
		"routes/orders.yaml": "list:\n  route: get /orders\n  states:\n    ok: {}\n",
	}))
	callGolden(t, cs, "lint", nil, "lint.json")
}

func TestTools_ReadTheFilesOnEveryCall(t *testing.T) {
	t.Parallel()

	dir := shop(t)
	cs := connect(t, dir)
	call(t, cs, "list_routes", nil)
	if err := os.Remove(filepath.Join(dir, "routes", "health.yaml")); err != nil {
		t.Fatal(err)
	}
	res := call(t, cs, "list_routes", nil)
	got, _ := res.StructuredContent.(map[string]any)
	if routes, _ := got["routes"].([]any); len(routes) != 2 {
		t.Errorf("after deleting health.yaml, %d routes; want 2", len(routes))
	}
}

func TestTools_UnreadableProject(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "nope")
	res := call(t, connect(t, missing), "list_routes", nil)
	if !res.IsError || !strings.Contains(text(res), "can't read project folder") {
		t.Errorf("isError %v, text %q", res.IsError, text(res))
	}
}

func TestGetRoute_Behavior(t *testing.T) {
	t.Parallel()

	cs := connect(t, project(t, map[string]string{
		"routes/pay.yaml": `create:
  route: POST /pay
  rules:
    - when: { body.amount: { gt: 100 }, header.x-test: { exists: true } }
      state: declined
  states:
    ok:
      latency: { base: 1s, jitter: 200ms }
      set: { paid: true }
      body: { receipt: "{{ uuid }}" }
    declined: { status: 402, fault: { type: reset, rate: 0.5, after: 2s } }
items:
  route: CRUD /cart/items
  crud: { collection: cart_items }
  states:
    ok: {}
`,
	}))
	callGolden(t, cs, "get_route", map[string]any{"id": "pay.create"}, "get_route_behavior.json")
	callGolden(t, cs, "get_route", map[string]any{"id": "pay.items"}, "get_route_crud.json")
}

func TestGetRoute_Contract(t *testing.T) {
	t.Parallel()

	cs := connect(t, project(t, map[string]string{
		"schemas/users.yaml": "User:\n  type: object\n  required: [id]\n  properties: { id: { type: string } }\n",
		"routes/users.yaml": `get:
  route: GET /users/{id}
  request:
    params: { id: { type: string, pattern: "^u_" } }
    query: { expand: { type: boolean, required: true } }
    headers: { Authorization: { type: string } }
  responses:
    200: User
    404: { type: object, properties: { error: { type: string } } }
  states:
    found: { body: generate }
    missing: { status: 404, body: { error: nope } }
create:
  route: POST /users
  request: { body: User }
  responses: { 201: User }
  states:
    created: { status: 201, body: { id: u_2 } }
`,
	}))
	callGolden(t, cs, "get_route", map[string]any{"id": "users.get"}, "get_route_contract.json")
	callGolden(t, cs, "get_route", map[string]any{"id": "users.create"}, "get_route_contract_body.json")
}
