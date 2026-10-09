package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

const usersFile = `# Users, newest first.
owners: { backend: [ademola] }

list:
  route: GET /users
  active: success # what the app sees by default
  states:
    success:
      body: { users: [{ id: "u_1" }] }   # keep in sync with the API
    empty:
      body: { users: [] }
    unauthorized:
      status: 401

get:
  route: GET /users/{id}
  states:
    found: {}
    not_found:
      status: 404
`

func project(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), config.DirName)
	for name, content := range files {
		path := filepath.Join(dir, "routes", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func routeFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "routes", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSetActive_ChangesOnlyThatRoutesActiveValue(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	previous, err := config.SetActive(dir, "users.list", "empty")
	if err != nil || previous != "success" {
		t.Fatalf("SetActive() = %q, %v; want success, nil", previous, err)
	}
	want := strings.Replace(usersFile, "active: success #", "active: empty #", 1)
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("file after SetActive:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetActive_AddsActiveWhenTheRouteHasNone(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	previous, err := config.SetActive(dir, "users.get", "not_found")
	if err != nil || previous != "found" {
		t.Fatalf("SetActive() = %q, %v; want found (the first state), nil", previous, err)
	}
	want := strings.Replace(usersFile, "  route: GET /users/{id}\n  states:", "  route: GET /users/{id}\n  active: not_found\n  states:", 1)
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("file after SetActive:\n%s\nwant:\n%s", got, want)
	}
	p, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 {
		t.Fatalf("Load after SetActive = %v, %v", probs, err)
	}
	if r, _ := p.Route("users.get"); r.Active != "not_found" {
		t.Errorf("Active = %q, want not_found", r.Active)
	}
}

func TestSetActive_QuotingStyles(t *testing.T) {
	t.Parallel()

	tests := []struct{ line, want string }{
		{"active: success", "active: empty"},
		{`active: "success"`, "active: empty"},
		{"active: 'success'", "active: empty"},
		{"active: success # note", "active: empty # note"},
		{"active:    success   ", "active:    empty   "},
		{`active: "success" # note`, "active: empty # note"},
	}
	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			t.Parallel()
			content := "get:\n  route: GET /r\n  " + tc.line + "\n  states:\n    success: {}\n    empty: {}\n"
			dir := project(t, map[string]string{"r.yaml": content})
			if _, err := config.SetActive(dir, "r.get", "empty"); err != nil {
				t.Fatal(err)
			}
			want := strings.Replace(content, tc.line, tc.want, 1)
			if got := routeFile(t, dir, "r.yaml"); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestSetActive_KeepsCRLFLineEndings(t *testing.T) {
	t.Parallel()

	content := strings.ReplaceAll(usersFile, "\n", "\r\n")
	dir := project(t, map[string]string{"users.yaml": content})
	if _, err := config.SetActive(dir, "users.list", "unauthorized"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SetActive(dir, "users.get", "not_found"); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(content, "active: success #", "active: unauthorized #", 1)
	want = strings.Replace(want, "  route: GET /users/{id}\r\n", "  route: GET /users/{id}\r\n  active: not_found\r\n", 1)
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetActive_KeepsPermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}

	dir := project(t, map[string]string{"users.yaml": usersFile})
	path := filepath.Join(dir, "routes", "users.yaml")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SetActive(dir, "users.list", "empty"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("permissions = %o, want 600", got)
	}
}

func TestSetActive_AlreadyActive(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ route, state string }{
		{"users.list", "success"},
		{"users.get", "found"},
	} {
		dir := project(t, map[string]string{"users.yaml": usersFile})
		previous, err := config.SetActive(dir, tc.route, tc.state)
		if err != nil || previous != tc.state {
			t.Errorf("SetActive(%s, %s) = %q, %v; want %q, nil", tc.route, tc.state, previous, err, tc.state)
		}
		if got := routeFile(t, dir, "users.yaml"); got != usersFile {
			t.Errorf("SetActive(%s, %s) rewrote the file although the state was already active", tc.route, tc.state)
		}
	}
}

func TestSetActive_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		routeID string
		state   string
		is      error
		message string
	}{
		{"route typo", "users.lst", "empty", config.ErrUnknownRoute, `no route "users.lst" (did you mean "users.list"?)`},
		{"unknown route", "orders", "empty", config.ErrUnknownRoute, `no route "orders" (routes: health.get, users.get, users.list)`},
		{"state typo", "users.list", "emty", config.ErrUnknownState, `route users.list has no state "emty" (did you mean "empty"?)`},
		{"unknown state", "users.list", "nope", config.ErrUnknownState, `route users.list has no state "nope" (states: success, empty, unauthorized)`},
	}
	dir := project(t, map[string]string{
		"users.yaml":  usersFile,
		"health.yaml": "get:\n  route: GET /health\n  states:\n    ok: {}\n",
	})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.SetActive(dir, tc.routeID, tc.state)
			if !errors.Is(err, tc.is) || err.Error() != tc.message {
				t.Errorf("SetActive(%s, %s) = %v; want %q", tc.routeID, tc.state, err, tc.message)
			}
		})
	}
	if got := routeFile(t, dir, "users.yaml"); got != usersFile {
		t.Error("a failed SetActive changed the file")
	}
}

func TestSetActive_FileWithProblems(t *testing.T) {
	t.Parallel()

	broken := "get:\n  route: get /r\n  states:\n    a: {}\n    b: {}\n"
	dir := project(t, map[string]string{
		"r.yaml":     broken,
		"other.yaml": "get:\n  route: GET other\n  states:\n    ok: {}\n",
	})
	_, err := config.SetActive(dir, "r.get", "b")
	want := "routes/r.yaml has problems; fix them first:\n" +
		`  routes/r.yaml:2: method "get" must be uppercase: GET`
	if err == nil || err.Error() != want {
		t.Errorf("SetActive() error = %v, want %q", err, want)
	}
	if got := routeFile(t, dir, "r.yaml"); got != broken {
		t.Error("SetActive changed a file that has problems")
	}
}

func TestSetActive_MissingProject(t *testing.T) {
	t.Parallel()

	if _, err := config.SetActive(filepath.Join(t.TempDir(), "nope"), "r.get", "a"); err == nil {
		t.Error("SetActive on a missing folder returned no error")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "users.yaml")
	if err := config.WriteFileAtomic(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteFileAtomic(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "v2" {
		t.Errorf("content = %q, want v2", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("folder has %d entries, want only users.yaml (temp files left behind?)", len(entries))
	}
}

func TestWriteFileAtomic_FailureLeavesNoTempFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "users.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteFileAtomic(target, []byte("x"), 0o644); err == nil {
		t.Fatal("WriteFileAtomic over a folder returned no error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Errorf("folder contents changed: %v", entries)
	}
}

func TestWriteFileAtomic_MissingFolder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "users.yaml")
	if err := config.WriteFileAtomic(path, []byte("x"), 0o644); err == nil {
		t.Error("WriteFileAtomic into a missing folder returned no error")
	}
}

func TestSetActive_OneLineRoute(t *testing.T) {
	t.Parallel()

	withActive := "get: { route: GET /r, active: a, states: { a: {}, b: {} } }\n"
	dir := project(t, map[string]string{"r.yaml": withActive})
	if _, err := config.SetActive(dir, "r.get", "b"); err != nil {
		t.Fatal(err)
	}
	if got := routeFile(t, dir, "r.yaml"); got != strings.Replace(withActive, "active: a", "active: b", 1) {
		t.Errorf("got %q", got)
	}

	withoutActive := "get: { route: GET /r, states: { a: {}, b: {} } }\n"
	dir = project(t, map[string]string{"r.yaml": withoutActive})
	_, err := config.SetActive(dir, "r.get", "b")
	want := `route r.get is written on one line; add "active: b" to it by hand`
	if err == nil || err.Error() != want {
		t.Errorf("SetActive() error = %v, want %q", err, want)
	}
	if got := routeFile(t, dir, "r.yaml"); got != withoutActive {
		t.Error("SetActive changed a one-line route it couldn't edit safely")
	}
}
