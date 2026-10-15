package cli_test

import (
	"bufio"
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func send(t *testing.T, method, url, body string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func TestStart_ServesTheControlAPI(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	port := freePort(t)
	lines, running := startInBackground(t, "--dir", dir, "--port", port, "--watch-interval", "20ms")
	stop := sync.OnceFunc(running)
	defer stop()
	base := "http://127.0.0.1:" + port
	<-lines
	go func() {
		for range lines {
		}
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/__mockmachina/events", nil)
	events, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer events.Body.Close()

	if code := send(t, http.MethodPut, base+"/__mockmachina/routes/users.list/state", `{"state":"unauthorized"}`); code != http.StatusOK {
		t.Fatalf("PUT state = %d", code)
	}
	if code, _ := get(t, base+"/users", ""); code != http.StatusUnauthorized {
		t.Errorf("GET /users after the pin = %d, want 401", code)
	}
	stream := bufio.NewScanner(events.Body)
	data := nextData(t, stream)
	if !strings.Contains(data, `"route":"users.list"`) || !strings.Contains(data, `"reason":"control"`) {
		t.Errorf("event = %s", data)
	}

	replaceIn(t, filepath.Join(dir, "routes", "users.yaml"), "    empty:", "    gone:\n      status: 410\n    empty:")
	eventually(t, "the reload to reach the control API", func() bool {
		_, body := get(t, base+"/__mockmachina/routes/users.list", "")
		return strings.Contains(body, `"gone"`)
	})
	if code, _ := get(t, base+"/users", ""); code != http.StatusUnauthorized {
		t.Errorf("GET /users after a reload = %d, want the pin to survive", code)
	}

	stop()
	for stream.Scan() {
	}
	if err := stream.Err(); err != nil {
		t.Errorf("the event stream didn't end cleanly on stop: %v", err)
	}
}

func nextData(t *testing.T, lines *bufio.Scanner) string {
	t.Helper()
	for lines.Scan() {
		if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
			return data
		}
	}
	t.Fatalf("event stream ended: %v", lines.Err())
	return ""
}

func TestStart_ControlTokenForOtherMachines(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	lines, stop := startInBackground(t, "--dir", dir, "--host", "0.0.0.0", "--port", "0")
	defer stop()
	<-lines
	note := <-lines
	if !strings.Contains(note, "/__mockmachina/") || !strings.Contains(note, "Authorization: Bearer ") {
		t.Errorf("note = %q, want the control API token", note)
	}
}

func lineWith(t *testing.T, lines <-chan string, part string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var seen []string
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("output ended without %q; saw %q", part, seen)
			}
			if strings.Contains(l, part) {
				return l
			}
			seen = append(seen, l)
		case <-ctx.Done():
			t.Fatalf("no line with %q; saw %q", part, seen)
		}
	}
}

func TestStart_PrintsTheDashboard(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	lines, stop := startInBackground(t, "--dir", copyUsersProject(t), "--port", port)
	defer stop()
	if got, want := lineWith(t, lines, "dashboard"), "dashboard on http://127.0.0.1:"+port+"/__mockmachina/ui/"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStart_PrintsAnOpenableDashboardForAnyHost(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	lines, stop := startInBackground(t, "--dir", copyUsersProject(t), "--host", "0.0.0.0", "--port", port)
	defer stop()
	if got, want := lineWith(t, lines, "dashboard"), "dashboard on http://localhost:"+port+"/__mockmachina/ui/; other machines sign in with the token"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStart_OpeningTheRootInABrowserShowsTheDashboard(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	lines, stop := startInBackground(t, "--dir", copyUsersProject(t), "--port", port)
	defer stop()
	<-lines
	go func() {
		for range lines {
		}
	}()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:"+port+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Request.URL.Path != "/__mockmachina/ui/" {
		t.Errorf("GET / from a browser ended at %s with %d, want the dashboard", res.Request.URL.Path, res.StatusCode)
	}
	if code, _ := get(t, "http://127.0.0.1:"+port+"/", ""); code != http.StatusNotFound {
		t.Errorf("GET / from a client = %d, want the mock's 404", code)
	}
}
