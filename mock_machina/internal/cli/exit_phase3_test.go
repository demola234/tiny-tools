package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

const proxyProject = `list:
  route: GET /users
  states:
    success:
      body: { users: [] }
get:
  route: GET /users/{id}
  serve: proxy
  states:
    found:
      body: { id: u_1, from: mock }
`

func TestExitPhase3_Proxy(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var seenState []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenState = append(seenState, r.Header.Get("X-Mock-State"))
		mu.Unlock()
		w.Header().Set("Access-Control-Allow-Origin", "https://backend.example")
		_, _ = io.WriteString(w, "backend "+r.Method+" "+r.URL.RequestURI())
	}))
	defer backend.Close()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), proxyProject)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pr, pw := io.Pipe()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   []string{"start", "--dir", dir, "--port", "0", "--proxy", backend.URL},
			Stdout: pw,
			Stderr: &stderr,
			Info:   testInfo,
		})
		_ = pw.Close()
	}()
	lines := readLines(pr)
	first := nextLine(t, lines)
	if !strings.Contains(first, "proxying the rest to "+backend.URL) {
		t.Fatalf("first line = %q", first)
	}
	base := servingURL.FindString(first)

	for _, tc := range []struct {
		name, path, state, body, log string
	}{
		{"mocked route", "/users", "", `{"users":[]}`, "GET /users 200 users.list:success (active)"},
		{"unmatched route", "/orders?page=2", "", "backend GET /orders?page=2", "GET /orders?page=2 200 proxied"},
		{"serve: proxy route", "/users/u_1", "", "backend GET /users/u_1", "GET /users/u_1 200 users.get proxied"},
		{"serve: proxy route asked for a state", "/users/u_1", "found", `{"id":"u_1","from":"mock"}`, "GET /users/u_1 200 users.get:found (header)"},
	} {
		if status, body := get(t, base+tc.path, tc.state); status != http.StatusOK || body != tc.body {
			t.Errorf("%s: %d %q, want %q", tc.name, status, body, tc.body)
		}
		if got := nextLine(t, lines); got != tc.log {
			t.Errorf("%s: log line %q, want %q", tc.name, got, tc.log)
		}
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/orders", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if got := res.Header.Values("Access-Control-Allow-Origin"); len(got) != 1 || got[0] != "http://localhost:3000" {
		t.Errorf("proxied CORS headers = %q; want only the mock's own", got)
	}
	<-lines

	mu.Lock()
	for _, s := range seenState {
		if s != "" {
			t.Errorf("backend got X-Mock-State %q", s)
		}
	}
	mu.Unlock()
	cancel()
	if code := <-done; code != cli.ExitOK {
		t.Errorf("exit %d; stderr %q", code, stderr.String())
	}
}

func TestExitPhase3_ActionFiles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"action.yml", filepath.Join("action", "run.sh")} {
		if _, err := os.Stat(filepath.Join("..", "..", name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func startProject(t *testing.T, dir string, args ...string) (<-chan string, func() int) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   append([]string{"start", "--dir", dir, "--port", "0"}, args...),
			Stdout: pw,
			Stderr: io.Discard,
			Info:   testInfo,
		})
		_ = pw.Close()
	}()
	lines := readLines(pr)
	return lines, func() int {
		cancel()
		for range lines {
		}
		return <-done
	}
}

func TestStart_ProxyFromConfig(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "backend "+r.URL.Path)
	}))
	defer backend.Close()
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), proxyProject)
	writeFile(t, filepath.Join(dir, "config.yaml"), "proxy: "+backend.URL+"\n")
	lines, stop := startProject(t, dir)
	defer stop()
	first := nextLine(t, lines)
	if !strings.HasSuffix(first, " and proxying the rest to "+backend.URL+" (Ctrl+C to stop)") {
		t.Fatalf("first line = %q", first)
	}
	if _, body := get(t, servingURL.FindString(first)+"/orders", ""); body != "backend /orders" {
		t.Errorf("GET /orders = %q", body)
	}
}

func TestStart_ServeProxyWithoutATarget(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), proxyProject)
	lines, stop := startProject(t, dir)
	defer stop()
	nextLine(t, lines)
	want := "warning: users.get has serve: proxy, but there's no proxy target (--proxy, or proxy in config.yaml), so it's mocked"
	if got := nextLine(t, lines); got != want {
		t.Errorf("second line = %q\nwant %q", got, want)
	}
}

func TestStart_BadProxyFlag(t *testing.T) {
	t.Parallel()

	code, _, stderr := runCLI(t, "start", "--dir", t.TempDir(), "--proxy", "staging")
	if code != cli.ExitUsage || !strings.Contains(stderr, `"staging" must be an http or https URL`) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func nextLine(t *testing.T, lines <-chan string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case line := <-lines:
		return line
	case <-ctx.Done():
		t.Fatal("no log line within 5s")
		return ""
	}
}
