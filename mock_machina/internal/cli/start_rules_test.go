package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const rulesProject = `get:
  route: GET /users/{id}
  rules:
    - when: { path.id: u_404 }
      state: not_found
  states:
    found: { body: { id: u_1 } }
    not_found: { status: 404 }
retry:
  route: GET /retry
  rules:
    - when: { call: { lte: 1 } }
      state: down
  states:
    ok: {}
    down: { status: 503 }
`

func TestStart_Rules(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	file := filepath.Join(dir, "routes", "users.yaml")
	writeFile(t, file, rulesProject)
	lines, stop := startProject(t, dir, "--watch-interval", "10ms")
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))

	if status, _ := get(t, base+"/users/u_404", ""); status != http.StatusNotFound {
		t.Errorf("GET /users/u_404 = %d, want 404", status)
	}
	if got, want := nextLine(t, lines), "GET /users/u_404 404 users.get:not_found (rule 1)"; got != want {
		t.Errorf("log line %q, want %q", got, want)
	}
	if status, _ := get(t, base+"/retry", ""); status != http.StatusServiceUnavailable {
		t.Errorf("first GET /retry = %d, want 503", status)
	}
	nextLine(t, lines)

	if err := os.WriteFile(file, []byte(rulesProject+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nextLine(t, lines)
	if status, _ := get(t, base+"/retry", ""); status != http.StatusOK {
		t.Errorf("second GET /retry after a reload = %d, want 200 (calls are kept)", status)
	}
}

func TestStart_LogsFaults(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "pay.yaml"), "create:\n  route: POST /pay\n  states:\n    flaky: { fault: reset }\n")
	lines, stop := startProject(t, dir)
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/pay", nil)
	if res, err := http.DefaultClient.Do(req); err == nil {
		_ = res.Body.Close()
		t.Error("the request succeeded; want a reset")
	}
	if got, want := nextLine(t, lines), "POST /pay pay.create:flaky (active) fault: reset"; got != want {
		t.Errorf("log line %q, want %q", got, want)
	}
}

const validatedRoute = `create:
  route: POST /session
  request:
    body: { type: object, required: [email] }
  states:
    ok: { status: 201 }
`

func TestStart_RequestValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		args []string
		code int
		log  string
	}{
		{nil, http.StatusBadRequest, `POST /session 400 session.create:ok (active) invalid request: body: is required`},
		{[]string{"--no-request-validation"}, http.StatusCreated, "POST /session 201 session.create:ok (active)"},
	} {
		dir := filepath.Join(t.TempDir(), ".mockmachina")
		writeFile(t, filepath.Join(dir, "routes", "session.yaml"), validatedRoute)
		lines, stop := startProject(t, dir, tc.args...)
		base := servingURL.FindString(nextLine(t, lines))
		if code := post(t, base+"/session"); code != tc.code {
			t.Errorf("%v: %d, want %d", tc.args, code, tc.code)
		}
		if got := nextLine(t, lines); got != tc.log {
			t.Errorf("%v: log %q\nwant %q", tc.args, got, tc.log)
		}
		stop()
	}
}
