package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/mcp"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	dir := shop(t)
	root := filepath.Dir(dir)
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "routes", "health.yaml"),
		[]byte("get:\n  route: GET /healthz\n  states:\n    up: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := connect(t, dir)
	callGolden(t, cs, "diff", nil, "diff.json")
	callGolden(t, cs, "diff", map[string]any{"base": "HEAD", "head": "main"}, "diff_none.json")
	wantToolError(t, cs, "diff", map[string]any{"base": "nope"}, `unknown git ref "nope"`)
}

type seen struct {
	mu       sync.Mutex
	requests []string
	auth     []string
}

func (s *seen) add(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.Method+" "+r.URL.Path)
	s.auth = append(s.auth, r.Header.Get("Authorization"))
}

func driftedAPI(t *testing.T) (*httptest.Server, *seen) {
	t.Helper()
	var s seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.add(r)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users":
			_, _ = w.Write([]byte(`{"users":[{"id":1}]}`))
		case "/health":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("ok"))
		default:
			_, _ = w.Write([]byte(`{"id":"u_1","name":"Ada"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &s
}

func TestDiffLive(t *testing.T) {
	t.Parallel()

	srv, seen := driftedAPI(t)
	dir := project(t, map[string]string{
		"routes/users.yaml":    usersYAML + "\ncreate:\n  route: POST /users\n  states:\n    created:\n      status: 201\n",
		"routes/health.yaml":   healthYAML,
		"routes/health/ok.txt": "ok",
	})
	cs := connectWith(t, mcp.Options{Dir: dir, LiveHeaders: http.Header{"Authorization": {"Bearer secret"}}})
	res := call(t, cs, "diff_live", map[string]any{"url": srv.URL, "params": map[string]any{"id": "u_9"}})
	if res.IsError {
		t.Fatalf("diff_live: %s", text(res))
	}
	got, _ := res.StructuredContent.(map[string]any)
	if got["base"] != "contract" || got["head"] != srv.URL {
		t.Errorf("base %v, head %v", got["base"], got["head"])
	}
	changes, _ := got["changes"].([]any)
	messages := make([]string, 0, len(changes))
	for _, c := range changes {
		ch := c.(map[string]any)
		messages = append(messages, ch["severity"].(string)+" "+ch["route"].(string)+": "+ch["message"].(string))
	}
	for _, want := range []string{
		`breaking users.list: state "success": field "users[].id" is a number live, a string in the contract`,
		"info users.create: not checked: POST isn't sent to a live API without --include-writes",
	} {
		if !slices.Contains(messages, want) {
			t.Errorf("changes %q\nwant one to be %q", messages, want)
		}
	}
	seen.mu.Lock()
	defer seen.mu.Unlock()
	slices.Sort(seen.requests)
	if want := []string{"GET /health", "GET /users", "GET /users/u_9"}; !slices.Equal(seen.requests, want) {
		t.Errorf("live API saw %v, want %v", seen.requests, want)
	}
	for _, a := range seen.auth {
		if a != "Bearer secret" {
			t.Errorf("Authorization = %q, want the launch header", a)
		}
	}
}

func TestDiffLive_Refusals(t *testing.T) {
	t.Parallel()

	cs := connect(t, shop(t))
	wantToolError(t, cs, "diff_live", map[string]any{"url": "ftp://example.com"}, `url "ftp://example.com" must be an http or https URL`)
	wantToolError(t, cs, "diff_live", map[string]any{"url": "http://x", "include_writes": true}, `invalid arguments: json: unknown field "include_writes"`)
}

func TestDiffTools_AreReadOnly(t *testing.T) {
	t.Parallel()

	res, err := connectWith(t, mcp.Options{Dir: shop(t), ReadOnly: true}).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	open := map[string]bool{}
	for _, tool := range res.Tools {
		if tool.Annotations.OpenWorldHint != nil {
			open[tool.Name] = *tool.Annotations.OpenWorldHint
		}
	}
	if _, ok := open["diff"]; !ok || open["diff"] || !open["diff_live"] {
		t.Errorf("openWorldHint by tool = %v; want diff closed and diff_live open", open)
	}
}
