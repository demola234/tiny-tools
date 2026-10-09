package config_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func emptyProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), config.DirName)
	if err := os.MkdirAll(filepath.Join(dir, "routes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAddRoute_Names(t *testing.T) {
	t.Parallel()

	tests := []struct{ method, path, id, file string }{
		{"GET", "/users", "users.list", "routes/users.yaml"},
		{"GET", "/users/{id}", "users.get", "routes/users.yaml"},
		{"POST", "/users", "users.create", "routes/users.yaml"},
		{"PUT", "/users/{id}", "users.replace", "routes/users.yaml"},
		{"PATCH", "/users/{id}", "users.update", "routes/users.yaml"},
		{"DELETE", "/users/{id}", "users.delete", "routes/users.yaml"},
		{"HEAD", "/users", "users.head", "routes/users.yaml"},
		{"OPTIONS", "/users", "users.options", "routes/users.yaml"},
		{"GET", "/users/{id}/orders", "orders.list", "routes/orders.yaml"},
		{"GET", "/v1/Users", "users.list", "routes/users.yaml"},
		{"GET", "/user_profiles", "user-profiles.list", "routes/user-profiles.yaml"},
		{"GET", "/", "root.list", "routes/root.yaml"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			dir := emptyProject(t)
			added, err := config.AddRoute(dir, tc.method, tc.path, config.AddOptions{})
			if err != nil || added.ID != tc.id || added.File != tc.file || !added.CreatedFile {
				t.Fatalf("AddRoute = %+v, %v; want %s in a new %s", added, err, tc.id, tc.file)
			}
			p, probs, err := config.Load(dir)
			if err != nil || len(probs) > 0 {
				t.Fatalf("Load after AddRoute = %v, %v", probs, err)
			}
			if _, err := config.FindRoute(p, tc.id); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestAddRoute_FirstStateFitsTheMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method, path, state string
		status              int
		body                string
	}{
		{"GET", "/users", "success", 200, "{}"},
		{"POST", "/users", "created", 201, "{}"},
		{"DELETE", "/users/{id}", "deleted", 204, ""},
	}
	for _, tc := range tests {
		dir := emptyProject(t)
		added, err := config.AddRoute(dir, tc.method, tc.path, config.AddOptions{})
		if err != nil {
			t.Fatal(err)
		}
		p, _, _ := config.Load(dir)
		r, _ := config.FindRoute(p, added.ID)
		st, ok := r.States.Get(tc.state)
		if !ok || st.EffectiveStatus() != tc.status || string(st.Body.Data) != tc.body {
			t.Errorf("%s %s: first state = %+v (found %v), want %s %d %q", tc.method, tc.path, st, ok, tc.state, tc.status, tc.body)
		}
	}
}

func TestAddRoute_NewFile(t *testing.T) {
	t.Parallel()

	dir := emptyProject(t)
	if _, err := config.AddRoute(dir, "GET", "/users", config.AddOptions{Summary: "List users"}); err != nil {
		t.Fatal(err)
	}
	testkit.Golden(t, readFile(t, filepath.Join(dir, "routes", "users.yaml")), "add/users.yaml")
}

func TestAddRoute_AppendsWithoutTouchingTheRest(t *testing.T) {
	t.Parallel()

	for _, eol := range []string{"\n", "\r\n"} {
		existing := strings.ReplaceAll(usersFile, "\n", eol)
		dir := project(t, map[string]string{"users.yaml": existing})
		added, err := config.AddRoute(dir, "POST", "/users", config.AddOptions{Name: "invite"})
		if err != nil || added.ID != "users.invite" || added.CreatedFile {
			t.Fatalf("AddRoute = %+v, %v; want users.invite in the existing file", added, err)
		}
		want := existing + eol + strings.ReplaceAll("invite:\n  route: POST /users\n  states:\n    created:\n      status: 201\n      body: {}\n", "\n", eol)
		if got := routeFile(t, dir, "users.yaml"); got != want {
			t.Errorf("file after AddRoute (eol %q):\n%q\nwant:\n%q", eol, got, want)
		}
	}
}

func TestAddRoute_FileWithoutFinalNewline(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": strings.TrimSuffix(okRoute, "\n")})
	if _, err := config.AddRoute(dir, "DELETE", "/users/{id}", config.AddOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, probs, err := config.Load(dir); err != nil || len(probs) > 0 {
		t.Errorf("Load after appending to a file without a final newline = %v, %v", probs, err)
	}
}

func TestAddRoute_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, method, path string
		opts               config.AddOptions
		want               string
	}{
		{"same method and path", "GET", "/users", config.AddOptions{}, `GET /users is already defined by route users.list`},
		{"same parameterised path", "GET", "/users/{userId}", config.AddOptions{}, `GET /users/{} is already defined by route users.get`},
		{"name taken", "POST", "/users", config.AddOptions{Name: "list"}, `route users.list already exists in routes/users.yaml (pass --name to pick another)`},
		{"lowercase method", "get", "/things", config.AddOptions{}, `method "get" must be uppercase: GET`},
		{"unknown method", "FETCH", "/things", config.AddOptions{}, `method "FETCH" isn't an HTTP method (expected one of: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)`},
		{"bad path", "GET", "things", config.AddOptions{}, `path "things" must start with "/"`},
		{"bad name", "GET", "/things", config.AddOptions{Name: "Get Things"}, `route name "Get Things" must use lowercase letters, digits and dashes`},
		{"summary over two lines", "GET", "/things", config.AddOptions{Summary: "one\ntwo"}, `summary must be one line of text`},
	}
	dir := project(t, map[string]string{"users.yaml": usersFile})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.AddRoute(dir, tc.method, tc.path, tc.opts)
			if err == nil || err.Error() != tc.want {
				t.Errorf("AddRoute(%s %s) error = %v, want %q", tc.method, tc.path, err, tc.want)
			}
		})
	}
	if got := routeFile(t, dir, "users.yaml"); got != usersFile {
		t.Error("a failed AddRoute changed the file")
	}
}

func TestAddRoute_SummaryNeedingQuotes(t *testing.T) {
	t.Parallel()

	dir := emptyProject(t)
	added, err := config.AddRoute(dir, "GET", "/things", config.AddOptions{Summary: "Things: the #1 list"})
	if err != nil {
		t.Fatal(err)
	}
	p, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 {
		t.Fatalf("Load = %v, %v", probs, err)
	}
	r, _ := config.FindRoute(p, added.ID)
	if r.Summary != "Things: the #1 list" {
		t.Errorf("Summary = %q, want it unchanged", r.Summary)
	}
}

func TestAddRoute_FileWithProblems(t *testing.T) {
	t.Parallel()

	broken := "get:\n  route: get /r\n  states:\n    ok: {}\n"
	dir := project(t, map[string]string{"r.yaml": broken})
	_, err := config.AddRoute(dir, "POST", "/r", config.AddOptions{})
	want := "routes/r.yaml has problems; fix them first:\n" + `  routes/r.yaml:2: method "get" must be uppercase: GET`
	if err == nil || err.Error() != want {
		t.Errorf("AddRoute() error = %v, want %q", err, want)
	}
	if got := routeFile(t, dir, "r.yaml"); got != broken {
		t.Error("AddRoute changed a file that has problems")
	}
}

func TestAddRoute_MissingProject(t *testing.T) {
	t.Parallel()

	_, err := config.AddRoute(filepath.Join(t.TempDir(), "nope"), "GET", "/x", config.AddOptions{})
	if err == nil || errors.Is(err, config.ErrUnknownRoute) {
		t.Errorf("AddRoute on a missing folder = %v, want a read error", err)
	}
}

func TestAddRoute_WithStates(t *testing.T) {
	t.Parallel()

	dir := emptyProject(t)
	added, err := config.AddRoute(dir, "GET", "/users/{id}", config.AddOptions{
		Summary:   "One user",
		Generated: true,
		States: []config.NewState{
			{Name: "found", Body: json.RawMessage(`{"id":"u_1","name":"Ada"}`)},
			{Name: "not_found", Status: 404, Body: json.RawMessage(`{"error":"no such user"}`)},
		},
	})
	if err != nil || added.ID != "users.get" || !added.CreatedFile {
		t.Fatalf("AddRoute = %+v, %v", added, err)
	}
	want := "# yaml-language-server: $schema=https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/schema/route.schema.json\n" +
		"get:\n  route: GET /users/{id}\n  summary: One user\n  status: draft\n  generated: true\n  states:\n" +
		"    found:\n      body: {id: u_1, name: Ada}\n" +
		"    not_found:\n      status: 404\n      body: {error: no such user}\n"
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddRoute_GeneratedRoutesAreDraftsWhateverTheFileSays(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": "status: implemented\n" + usersFile})
	added, err := config.AddRoute(dir, "DELETE", "/users/{id}", config.AddOptions{
		Generated: true,
		States:    []config.NewState{{Name: "deleted", Status: 204}},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := loadDir(t, dir).Route(added.ID)
	if r.Status != model.StatusDraft || !r.Generated {
		t.Errorf("status %q, generated %v; want draft, true", r.Status, r.Generated)
	}
	list, _ := loadDir(t, dir).Route("users.list")
	if list.Status != model.StatusImplemented {
		t.Errorf("users.list status = %q, want implemented, untouched", list.Status)
	}
}

func TestAddRoute_LargeStateBodiesGoToFiles(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	body := `{"users":["` + strings.Repeat("x", 5000) + `"]}`
	if _, err := config.AddRoute(dir, "GET", "/v2/users", config.AddOptions{
		Name:   "export",
		States: []config.NewState{{Name: "ok", Body: json.RawMessage(body)}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := routeFile(t, dir, "users.yaml"); !strings.HasSuffix(got, "  states:\n    ok:\n      body: export.ok.json\n") {
		t.Errorf("users.yaml:\n%s", got)
	}
	r, _ := loadDir(t, dir).Route("users.export")
	st, _ := r.States.Get("ok")
	if st.Body.File != "export.ok.json" || len(st.Body.Data) < 5000 {
		t.Errorf("body = %q with %d bytes", st.Body.File, len(st.Body.Data))
	}
}

func TestAddRoute_StateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		states []config.NewState
		want   string
	}{
		{"listed twice", []config.NewState{{Name: "ok"}, {Name: "ok"}}, `state "ok" is listed twice`},
		{"bad name", []config.NewState{{Name: "Not OK"}}, `state "Not OK" must use lowercase letters, digits and underscores, starting with a letter`},
		{"text body", []config.NewState{{Name: "ok", Body: json.RawMessage(`"hi"`)}}, "body must be a JSON object or array"},
		{
			"bad status, with the line it would have",
			[]config.NewState{{Name: "ok"}, {Name: "odd", Status: 999}},
			"the new route would break routes/users.yaml:\n  routes/users.yaml:27: status 999 isn't an HTTP status code (100-599)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := project(t, map[string]string{"users.yaml": usersFile})
			_, err := config.AddRoute(dir, "GET", "/v2/users", config.AddOptions{Name: "export", States: tc.states})
			if err == nil || err.Error() != tc.want {
				t.Errorf("error = %v\nwant: %s", err, tc.want)
			}
			if got := routeFile(t, dir, "users.yaml"); got != usersFile {
				t.Errorf("users.yaml changed:\n%s", got)
			}
		})
	}
}
