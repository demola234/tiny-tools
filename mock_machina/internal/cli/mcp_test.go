package cli_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type mcpSession struct {
	t      *testing.T
	in     *io.PipeWriter
	lines  <-chan string
	done   <-chan int
	stderr *strings.Builder
}

func startMCP(t *testing.T, args ...string) *mcpSession {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan int, 1)
	s := &mcpSession{t: t, in: inW, done: done, stderr: &strings.Builder{}}
	go func() {
		done <- cli.Run(t.Context(), cli.Env{
			Args:   append([]string{"mcp"}, args...),
			Stdin:  inR,
			Stdout: outW,
			Stderr: s.stderr,
			Info:   testInfo,
		})
		_ = outW.Close()
		_ = inR.Close()
	}()
	s.lines = readLines(outR)
	s.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`)
	return s
}

func (s *mcpSession) send(msg string) {
	s.t.Helper()
	if _, err := io.WriteString(s.in, msg+"\n"); err != nil {
		s.t.Fatal(err)
	}
}

func (s *mcpSession) receive(id int) json.RawMessage {
	s.t.Helper()
	line, ok := <-s.lines
	if !ok {
		s.t.Fatalf("stdout closed waiting for response %d; stderr %q", id, s.stderr.String())
	}
	var resp rpcResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		s.t.Fatalf("stdout line isn't JSON-RPC: %q", line)
	}
	if resp.ID != id || resp.Error != nil {
		s.t.Fatalf("response %s; want a result for id %d", line, id)
	}
	return resp.Result
}

func (s *mcpSession) close() {
	s.t.Helper()
	_ = s.in.Close()
	if code := <-s.done; code != cli.ExitOK {
		s.t.Errorf("exit %d after stdin closed, want 0; stderr %q", code, s.stderr.String())
	}
	for line := range s.lines {
		s.t.Errorf("unexpected stdout after stdin closed: %q", line)
	}
}

func TestMCP_ServesOverStdio(t *testing.T) {
	t.Parallel()

	s := startMCP(t, "--dir", testkit.Path(t, "projects", "users", ".mockmachina"))
	var init struct {
		ServerInfo struct{ Name, Version string } `json:"serverInfo"`
	}
	if err := json.Unmarshal(s.receive(1), &init); err != nil {
		t.Fatal(err)
	}
	if init.ServerInfo.Name != "mockmachina" || init.ServerInfo.Version != testInfo.String() {
		t.Errorf("serverInfo = %+v; want mockmachina %s", init.ServerInfo, testInfo)
	}

	s.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	s.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_routes","arguments":{}}}`)
	if got := string(s.receive(2)); !strings.Contains(got, `"id":"users.list"`) {
		t.Errorf("list_routes result = %s; want users.list in it", got)
	}
	s.close()
}

func TestMCP_ReadOnly(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		args       []string
		wantWrites bool
	}{
		{nil, true},
		{[]string{"--read-only"}, false},
	} {
		s := startMCP(t, append([]string{"--dir", testkit.Path(t, "projects", "users", ".mockmachina")}, tc.args...)...)
		s.receive(1)
		s.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
		s.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
		tools := string(s.receive(2))
		if got := strings.Contains(tools, `"name":"add_state"`); got != tc.wantWrites {
			t.Errorf("%v: add_state listed = %v, want %v", tc.args, got, tc.wantWrites)
		}
		s.close()
	}
}

func TestMCP_NoProject(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), ".mockmachina")
	code, stdout, stderr := runCLI(t, "mcp", "--dir", missing)
	if code != cli.ExitFailure || stdout != "" || !strings.Contains(stderr, "can't read project folder") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestMCP_LiveHeader(t *testing.T) {
	t.Parallel()

	auth := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth <- r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"users":[]}`))
	}))
	defer srv.Close()

	s := startMCP(t, "--dir", testkit.Path(t, "projects", "users", ".mockmachina"), "--live-header", "Authorization: Bearer secret")
	s.receive(1)
	s.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	s.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"diff_live","arguments":{"url":"` + srv.URL + `"}}}`)
	s.receive(2)
	s.close()
	close(auth)
	n := 0
	for a := range auth {
		n++
		if a != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", a)
		}
	}
	if n == 0 {
		t.Error("the live API got no requests")
	}
}

func TestMCP_BadLiveHeader(t *testing.T) {
	t.Parallel()

	code, _, stderr := runCLI(t, "mcp", "--dir", testkit.Path(t, "projects", "users", ".mockmachina"), "--live-header", "Bearer secret")
	if code != cli.ExitUsage || !strings.Contains(stderr, `"Bearer secret" must look like "Name: value"`) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestMCP_PrintConfig(t *testing.T) {
	t.Parallel()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.Path(t, "projects", "users", ".mockmachina")
	for _, client := range []string{"claude-code", "claude-desktop", "cursor", "vscode"} {
		code, stdout, stderr := runCLI(t, "mcp", "--dir", dir, "--print-config", client)
		if code != cli.ExitOK || stderr != "" {
			t.Fatalf("%s: exit %d, stderr %q", client, code, stderr)
		}
		out := masked(masked(stdout, exe, "EXE"), dir, "DIR")
		testkit.Golden(t, []byte(out), "mcp/config/"+client+".txt")
	}
}

func masked(s, path, name string) string {
	quoted, _ := json.Marshal(path)
	for _, form := range []string{"'" + path + "'", string(quoted[1 : len(quoted)-1]), path} {
		s = strings.ReplaceAll(s, form, name)
	}
	return s
}

func TestMCP_PrintConfigKeepsFlags(t *testing.T) {
	t.Parallel()

	dir := testkit.Path(t, "projects", "users", ".mockmachina")
	code, stdout, _ := runCLI(t, "mcp", "--dir", dir, "--print-config", "claude-code", "--read-only", "--live-header", "Authorization: Bearer x")
	if code != cli.ExitOK || !strings.HasSuffix(stdout, " --read-only --live-header 'Authorization: Bearer x'\n") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestMCP_PrintConfigUnknownClient(t *testing.T) {
	t.Parallel()

	code, _, stderr := runCLI(t, "mcp", "--dir", testkit.Path(t, "projects", "users", ".mockmachina"), "--print-config", "cursr")
	if code != cli.ExitUsage || !strings.Contains(stderr, `"cursr" (did you mean "cursor"?`) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func postInitialize(t *testing.T, url string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func startMCPHTTP(t *testing.T, args ...string) (url string, stop func() int) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	outR, outW := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   append([]string{"mcp", "--dir", testkit.Path(t, "projects", "users", ".mockmachina")}, args...),
			Stdout: outW,
			Stderr: io.Discard,
			Info:   testInfo,
		})
		_ = outW.Close()
	}()
	lines := readLines(outR)
	first := <-lines
	rest, ok := strings.CutPrefix(first, "serving MCP on ")
	if !ok {
		cancel()
		t.Fatalf("first line %q", first)
	}
	url, _, _ = strings.Cut(rest, " ")
	return url, func() int {
		cancel()
		for range lines {
		}
		return <-done
	}
}

func TestMCP_HTTP(t *testing.T) {
	t.Parallel()

	url, stop := startMCPHTTP(t, "--http", "127.0.0.1:0")
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(url, "/mcp") {
		t.Errorf("url = %q", url)
	}
	if code, body := postInitialize(t, url, nil); code != http.StatusOK || !strings.Contains(body, `"name":"mockmachina"`) {
		t.Errorf("initialize: %d %s", code, body)
	}
	if code := stop(); code != cli.ExitOK {
		t.Errorf("exit %d after stopping, want 0", code)
	}
}

func TestMCP_HTTPTokenFromEnvironment(t *testing.T) {
	t.Setenv("MOCKMACHINA_MCP_TOKEN", "s3cret")

	url, stop := startMCPHTTP(t, "--http", "127.0.0.1:0")
	defer stop()
	if code, _ := postInitialize(t, url, nil); code != http.StatusUnauthorized {
		t.Errorf("without the token: %d, want 401", code)
	}
	if code, _ := postInitialize(t, url, map[string]string{"Authorization": "Bearer s3cret"}); code != http.StatusOK {
		t.Errorf("with the token: %d, want 200", code)
	}
}

func TestMCP_HTTPUsageErrors(t *testing.T) {
	t.Parallel()

	dir := testkit.Path(t, "projects", "users", ".mockmachina")
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--http", "0.0.0.0:4002"}, "0.0.0.0:4002 can be reached from other machines; set --token too"},
		{[]string{"--http", ":4002"}, ":4002 can be reached from other machines; set --token too"},
		{[]string{"--http", "nope"}, `"nope" must be host:port, like 127.0.0.1:4002`},
		{[]string{"--token", "x"}, "only applies with --http"},
	}
	for _, tc := range tests {
		code, _, stderr := runCLI(t, append([]string{"mcp", "--dir", dir}, tc.args...)...)
		if code != cli.ExitUsage || !strings.Contains(stderr, tc.want) {
			t.Errorf("%v: exit %d, stderr %q; want %q", tc.args, code, stderr, tc.want)
		}
	}
}

func TestDocs_Serve(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "shop", ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: GET /users\n  states:\n    ok: {}\n")
	ctx, cancel := context.WithCancel(t.Context())
	outR, outW := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{Args: []string{"docs", "--serve", "--port", "0", "--dir", dir}, Stdout: outW, Stderr: io.Discard, Info: testInfo})
		_ = outW.Close()
	}()
	lines := readLines(outR)
	first := nextLine(t, lines)
	base := servingURL.FindString(first)
	if !strings.HasPrefix(first, "docs for shop at http://127.0.0.1:") {
		t.Fatalf("first line %q", first)
	}
	if _, body := get(t, base+"/", ""); !strings.Contains(body, "<title>shop</title>") {
		t.Errorf("index = %.80q", body)
	}
	if _, spec := get(t, base+"/openapi.yaml", ""); !strings.Contains(spec, "operationId: users.list") {
		t.Errorf("spec = %.200q", spec)
	}
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: GET /users\n  states:\n    ok: {}\nget:\n  route: GET /users/{id}\n  states:\n    ok: {}\n")
	if _, spec := get(t, base+"/openapi.yaml", ""); !strings.Contains(spec, "operationId: users.get") {
		t.Error("the spec didn't follow an edit")
	}
	cancel()
	for range lines {
	}
	if code := <-done; code != cli.ExitOK {
		t.Errorf("exit %d", code)
	}
}
