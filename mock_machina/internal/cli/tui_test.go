package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return ansi.Strip(b.buf.String())
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		_ = clock.Real{}.Sleep(t.Context(), 20*time.Millisecond)
	}
}

func httpGet(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func servedState(t *testing.T, url string) string {
	t.Helper()
	res, err := httpGet(t, url)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.Header.Get("X-Mock-State")
}

func TestTUI_SwitchingAStateChangesTheNextResponse(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	port := freePort(t)
	url := "http://127.0.0.1:" + port + "/users"
	keys, typing := io.Pipe()
	out := &lockedBuffer{}
	code := make(chan int, 1)
	go func() {
		code <- cli.Run(t.Context(), cli.Env{
			Args:   []string{"tui", "--dir", dir, "--port", port, "--watch-interval", "20ms"},
			Stdin:  keys,
			Stdout: out,
			Stderr: out,
			Info:   testInfo,
		})
	}()

	eventually(t, "the mock to serve success", func() bool { return servedState(t, url) == "success" })
	eventually(t, "the screen to draw", func() bool { return strings.Contains(out.String(), "mockmachina") })

	for _, k := range []string{"\x1b[B", "\x1b[B", "\r"} {
		if _, err := io.WriteString(typing, k); err != nil {
			t.Fatal(err)
		}
		_ = clock.Real{}.Sleep(t.Context(), 50*time.Millisecond)
	}
	eventually(t, "the next response to be empty", func() bool { return servedState(t, url) == "empty" })

	data, err := os.ReadFile(filepath.Join(dir, "routes", "users.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "active: empty") {
		t.Errorf("routes/users.yaml should record the switch:\n%s", data)
	}

	if _, err := io.WriteString(typing, "q"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	select {
	case c := <-code:
		if c != cli.ExitOK {
			t.Errorf("exit code = %d, want 0\n%s", c, out.String())
		}
	case <-ctx.Done():
		t.Fatal("q didn't stop the UI")
	}
}

func TestTUI_ChecksFlagsLikeStart(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	code := cli.Run(t.Context(), cli.Env{Args: []string{"tui", "--tls-cert", "cert.pem"}, Stdout: &out, Stderr: &out, Info: testInfo})
	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(out.String(), "and --tls-key go together") {
		t.Errorf("output = %q, want the TLS pairing rule", out.String())
	}
}

func TestTUI_ProjectWithProblemsDoesNotStart(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: GET /users\n  states: {}\n")
	var out bytes.Buffer
	code := cli.Run(t.Context(), cli.Env{Args: []string{"tui", "--dir", dir}, Stdout: &out, Stderr: &out, Info: testInfo})
	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d\n%s", code, cli.ExitFailure, out.String())
	}
	if !strings.Contains(out.String(), "not serving until it's fixed") {
		t.Errorf("output = %q, want the problems", out.String())
	}
}
