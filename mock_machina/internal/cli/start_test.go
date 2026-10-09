package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

var servingURL = regexp.MustCompile(`http://\S+`)

func TestStart_ServesTheProject(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pr, pw := io.Pipe()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	dir := testkit.Path(t, "projects", "users", ".mockmachina")
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   []string{"start", "--dir", dir, "--port", "0"},
			Stdout: pw,
			Stderr: &stderr,
			Info:   testInfo,
		})
		_ = pw.Close()
	}()
	lines := readLines(pr)

	first := <-lines
	if !strings.HasPrefix(first, "serving 1 route from "+dir+" on http://127.0.0.1:") {
		t.Fatalf("first line = %q, want it to say where it's serving", first)
	}
	base := servingURL.FindString(first)

	status, body := get(t, base+"/users", "")
	if status != http.StatusOK || body != `{"users":[{"id":"u_1","name":"Ada"}]}`+"\n" {
		t.Errorf("GET /users = %d %q", status, body)
	}
	expectLine(t, lines, "GET /users 200 users.list:success (active)")

	status, body = get(t, base+"/users", "empty")
	if status != http.StatusOK || body != `{"users":[]}` {
		t.Errorf("GET /users with X-Mock-State: empty = %d %q", status, body)
	}
	expectLine(t, lines, "GET /users 200 users.list:empty (header)")

	get(t, base+"/users?__state=nope", "")
	expectLine(t, lines, "GET /users 400 users.list:nope (query) unknown state")

	get(t, base+"/usrs", "")
	expectLine(t, lines, "GET /usrs 404 no route (closest: GET /users)")

	cancel()
	if code := <-done; code != cli.ExitOK {
		t.Errorf("Run returned %d after cancel, want 0 (stderr: %q)", code, stderr.String())
	}
	expectLine(t, lines, "stopped")
}

func TestStart_ProjectWithProblems(t *testing.T) {
	t.Parallel()

	dir := testkit.Path(t, "projects", "broken", ".mockmachina")
	code, stdout, stderr := runStart(t, "--dir", dir)

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
	want := `routes/users.yaml:3: active state "empt" doesn't exist (did you mean "empty"?)` + "\n" +
		"1 problem in 1 file; not serving until it's fixed\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestStart_MissingFolder(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nope")
	code, _, stderr := runStart(t, "--dir", dir)

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: can't read project folder "+dir+": ") {
		t.Errorf("stderr = %q, want one plain line naming the folder", stderr)
	}
	if strings.Contains(stderr, "stat .") || strings.Count(stderr, "\n") != 1 {
		t.Errorf("stderr = %q, want one line without Go's internal stat details", stderr)
	}
}

func TestStart_PortInUse(t *testing.T) {
	t.Parallel()

	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	dir := testkit.Path(t, "projects", "users", ".mockmachina")
	code, _, stderr := runStart(t, "--dir", dir, "--port", port)

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: can't listen on 127.0.0.1:"+port+" (port ") {
		t.Errorf("stderr = %q, want it to name the address", stderr)
	}
	m := regexp.MustCompile(`port (\d+) is free, try --port (\d+)\)`).FindStringSubmatch(stderr)
	if m == nil || m[1] != m[2] || m[1] == port {
		t.Errorf("stderr = %q, want a different free port to try", stderr)
	}
}

func TestStart_ExtraArgumentsAreUsageErrors(t *testing.T) {
	t.Parallel()

	code, _, stderr := runStart(t, "extra")
	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want 2 (stderr: %q)", code, stderr)
	}
}

func TestStart_LogLinesNeverInterleave(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lines := make(chan string, 64)
	out := &unlockedRecorder{lines: lines}
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   []string{"start", "--dir", testkit.Path(t, "projects", "users", ".mockmachina"), "--port", "0"},
			Stdout: out,
			Stderr: io.Discard,
			Info:   testInfo,
		})
	}()
	base := servingURL.FindString(<-lines)

	const requests = 50
	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() { fire(t, base+"/users") })
	}
	wg.Wait()
	cancel()
	<-done
}

type unlockedRecorder struct {
	written []byte
	lines   chan<- string
}

func (r *unlockedRecorder) Write(p []byte) (int, error) {
	r.written = append(r.written, p...)
	r.lines <- strings.TrimSuffix(string(p), "\n")
	return len(p), nil
}

func TestStart_ReloadsWhenFilesChange(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	route := filepath.Join(dir, "routes", "users.yaml")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   []string{"start", "--dir", dir, "--port", "0", "--watch-interval", "10ms"},
			Stdout: pw,
			Stderr: io.Discard,
			Info:   testInfo,
		})
		_ = pw.Close()
	}()
	lines := readLines(pr)
	base := servingURL.FindString(<-lines)
	users := func() string {
		_, body := get(t, base+"/users", "")
		<-lines
		return body
	}

	replaceIn(t, route, "  route: GET /users\n", "  route: GET /users\n  active: empty\n")
	expectLine(t, lines, "reloaded after changes to routes/users.yaml (1 route)")
	if got := users(); got != `{"users":[]}` {
		t.Errorf("after switching active to empty, body = %q", got)
	}

	writeFile(t, filepath.Join(dir, "routes", "health.yaml"),
		"get:\n  route: GET /health\n  states:\n    ok:\n      body: { status: up }\n")
	expectLine(t, lines, "reloaded after changes to routes/health.yaml (2 routes)")
	if status, body := get(t, base+"/health", ""); status != http.StatusOK || body != `{"status":"up"}` {
		t.Errorf("GET /health = %d %q", status, body)
	}
	<-lines

	replaceIn(t, route, "active: empty", "active: empt")
	expectLine(t, lines, "reload failed, still serving the previous version:")
	expectLine(t, lines, `  routes/users.yaml:3: active state "empt" doesn't exist (did you mean "empty"?)`)
	if got := users(); got != `{"users":[]}` {
		t.Errorf("after a broken save, body = %q, want the previous version", got)
	}

	replaceIn(t, route, "active: empt", "active: success")
	expectLine(t, lines, "reloaded after changes to routes/users.yaml (2 routes)")
	if got := users(); !strings.Contains(got, "Ada") {
		t.Errorf("after fixing the file, body = %q, want the success state", got)
	}

	cancel()
	if code := <-done; code != cli.ExitOK {
		t.Errorf("Run returned %d, want 0", code)
	}
	expectLine(t, lines, "stopped")
}

const (
	routeA = "get:\n  route: GET /a/{x}\n  states:\n    ok: {}\n"
	routeB = "get:\n  route: GET /{y}/b\n  states:\n    ok: {}\n"
)

const overlapMsg = "routes a.get (GET /a/{x}) and b.get (GET /{y}/b) overlap: a request could match both"

func TestStart_OverlappingRoutesAtStartup(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "a.yaml"), routeA)
	writeFile(t, filepath.Join(dir, "routes", "b.yaml"), routeB)
	code, _, stderr := runStart(t, "--dir", dir, "--port", "0")

	if code != cli.ExitFailure || stderr != "error: "+overlapMsg+"\n" {
		t.Errorf("exit %d, stderr %q; want 1 and the overlap error", code, stderr)
	}
}

func TestStart_ReloadFailuresKeepServing(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   []string{"start", "--dir", dir, "--port", "0", "--watch-interval", "10ms"},
			Stdout: pw,
			Stderr: io.Discard,
			Info:   testInfo,
		})
		_ = pw.Close()
	}()
	lines := readLines(pr)
	<-lines

	writeFile(t, filepath.Join(dir, "routes", "a.yaml"), routeA)
	writeFile(t, filepath.Join(dir, "routes", "b.yaml"), routeB)
	first := <-lines
	if first == "reloaded after changes to routes/a.yaml (2 routes)" {
		first = <-lines
	}
	if first != "reload failed, still serving the previous version:" {
		t.Errorf("log line = %q, want the reload to fail", first)
	}
	expectLine(t, lines, "  "+overlapMsg)

	if err := os.Remove(filepath.Join(dir, "routes", "b.yaml")); err != nil {
		t.Fatal(err)
	}
	expectLine(t, lines, "reloaded after changes to routes/b.yaml (2 routes)")

	moved := dir + "-moved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	expectLine(t, lines, "reload failed, still serving the previous version:")
	if got := <-lines; !strings.HasPrefix(got, "  can't read project folder "+dir+": ") {
		t.Errorf("log line = %q, want it to say the folder can't be read", got)
	}
	if err := os.Rename(moved, dir); err != nil {
		t.Fatal(err)
	}
	expectLine(t, lines, "reloaded after changes to routes/a.yaml and 2 more (2 routes)")

	cancel()
	<-done
}

func TestStart_UsesConfigFile(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	port := freePort(t)
	writeFile(t, filepath.Join(dir, "config.yaml"), "ports:\n  mock: "+port+"\n")
	lines, stop := startInBackground(t, "--dir", dir)
	defer stop()

	if first := <-lines; !strings.Contains(first, "on http://127.0.0.1:"+port+" ") {
		t.Errorf("first line = %q, want the port from config.yaml (%s)", first, port)
	}
}

func TestStart_FlagsOverrideConfigFile(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	configPort := freePort(t)
	writeFile(t, filepath.Join(dir, "config.yaml"), "ports:\n  mock: "+configPort+"\n")
	lines, stop := startInBackground(t, "--dir", dir, "--port", "0")
	defer stop()

	if first := <-lines; strings.Contains(first, ":"+configPort+" ") {
		t.Errorf("first line = %q, want --port 0 to win over config.yaml", first)
	}
}

func TestStart_ConfigAddressChangeNeedsARestart(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	port := freePort(t)
	writeFile(t, filepath.Join(dir, "config.yaml"), "ports:\n  mock: "+port+"\n")
	lines, stop := startInBackground(t, "--dir", dir, "--watch-interval", "10ms")
	defer stop()
	<-lines

	writeFile(t, filepath.Join(dir, "config.yaml"), "ports:\n  mock: 5999\n")
	expectLine(t, lines, "reloaded after changes to config.yaml (1 route)")
	expectLine(t, lines, "config.yaml now says 127.0.0.1:5999; restart to use it (still serving on 127.0.0.1:"+port+")")
}

func startInBackground(t *testing.T, args ...string) (<-chan string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.Run(ctx, cli.Env{Args: append([]string{"start"}, args...), Stdout: pw, Stderr: pw, Info: testInfo})
		_ = pw.Close()
	}()
	lines := readLines(pr)
	return lines, func() {
		cancel()
		for range lines {
		}
		<-done
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func TestStart_WatchIntervalMustBePositive(t *testing.T) {
	t.Parallel()

	code, _, stderr := runStart(t, "--watch-interval", "0s")
	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "atch interval must be more than 0 (--watch-interval)") {
		t.Errorf("stderr = %q, want it to explain the interval", stderr)
	}
}

func copyUsersProject(t *testing.T) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), ".mockmachina")
	if err := os.CopyFS(dst, os.DirFS(testkit.Path(t, "projects", "users", ".mockmachina"))); err != nil {
		t.Fatal(err)
	}
	return dst
}

func replaceIn(t *testing.T, path, old, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s doesn't contain %q", path, old)
	}
	writeFile(t, path, strings.Replace(string(data), old, replacement, 1))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runStart(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Run(t.Context(), cli.Env{
		Args:   append([]string{"start"}, args...),
		Stdout: &out,
		Stderr: &errOut,
		Info:   testInfo,
	})
	return code, out.String(), errOut.String()
}

func readLines(r io.Reader) <-chan string {
	lines := make(chan string, 16)
	go func() {
		defer close(lines)
		s := bufio.NewScanner(r)
		for s.Scan() {
			lines <- s.Text()
		}
	}()
	return lines
}

func expectLine(t *testing.T, lines <-chan string, want string) {
	t.Helper()
	if got := <-lines; got != want {
		t.Errorf("log line = %q, want %q", got, want)
	}
}

func fire(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Error(err)
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("GET %s: %v", url, err)
		return
	}
	_ = res.Body.Close()
}

func get(t *testing.T, url, state string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if state != "" {
		req.Header.Set("X-Mock-State", state)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

func TestStart_CORSIsOnUnlessDisabled(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "http://localhost:5173"},
		{[]string{"--no-cors"}, ""},
	} {
		dir := copyUsersProject(t)
		lines, stop := startInBackground(t, append([]string{"--dir", dir, "--port", "0"}, tc.args...)...)
		base := servingURL.FindString(<-lines)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/users", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "http://localhost:5173")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Errorf("start %v: Access-Control-Allow-Origin = %q, want %q", tc.args, got, tc.want)
		}
		stop()
	}
}

func TestStart_NoMockHeaders(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	lines, stop := startInBackground(t, "--dir", dir, "--port", "0", "--no-mock-headers")
	defer stop()
	base := servingURL.FindString(<-lines)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/users", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if got := res.Header.Get("X-Mock-State"); got != "" {
		t.Errorf("X-Mock-State = %q with --no-mock-headers, want none", got)
	}
}

func TestStart_ListenFailsWithNoFreePortNearby(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	code, _, stderr := runStart(t, "--dir", dir, "--host", "203.0.113.1", "--port", "4001")
	if code != cli.ExitFailure || !strings.HasPrefix(stderr, "error: can't listen on 203.0.113.1:4001: ") {
		t.Errorf("exit %d, stderr %q; want 1 and the listen error without a port suggestion", code, stderr)
	}
}

func TestStart_PlainFlagIsKnown(t *testing.T) {
	t.Parallel()

	code, _, stderr := runStart(t, "--plain", "--watch-interval", "0s")
	if code != cli.ExitUsage || !strings.Contains(stderr, "atch interval must be more than 0") {
		t.Errorf("exit %d, stderr %q: want --plain accepted and the interval rejected", code, stderr)
	}
}
