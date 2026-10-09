package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

const listProject = `list:
  route: GET /users
  states:
    success: {}
    empty: {}
    unauthorized:
      status: 401
delete:
  route: DELETE /users/{id}
  active: not_found
  states:
    deleted:
      status: 204
    not_found:
      status: 404
`

func listDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), listProject)
	writeFile(t, filepath.Join(dir, "routes", "health.yaml"), "get:\n  route: GET /health\n  states:\n    up: {}\n")
	return dir
}

func TestStateList(t *testing.T) {
	t.Parallel()

	dir := listDir(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "every route",
			args: []string{"state", "list", "--dir", dir},
			want: "ROUTE         METHOD  PATH         ACTIVE\n" +
				"health.get    GET     /health      up\n" +
				"users.delete  DELETE  /users/{id}  not_found\n" +
				"users.list    GET     /users       success\n",
		},
		{
			name: "one route",
			args: []string{"state", "list", "users.list", "--dir", dir},
			want: "users.list  GET /users\n" +
				"* success       200\n" +
				"  empty         200\n" +
				"  unauthorized  401\n",
		},
		{
			name: "every route as JSON",
			args: []string{"state", "list", "--json", "--dir", dir},
			want: `[{"route":"health.get","method":"GET","path":"/health","active":"up","states":["up"]},` +
				`{"route":"users.delete","method":"DELETE","path":"/users/{id}","active":"not_found","states":["deleted","not_found"]},` +
				`{"route":"users.list","method":"GET","path":"/users","active":"success","states":["success","empty","unauthorized"]}]` + "\n",
		},
		{
			name: "one route as JSON",
			args: []string{"state", "list", "users.delete", "--json", "--dir", dir},
			want: `{"route":"users.delete","method":"DELETE","path":"/users/{id}","active":"not_found",` +
				`"states":[{"name":"deleted","status":204},{"name":"not_found","status":404}]}` + "\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runCLI(t, tc.args...)
			if code != cli.ExitOK || stdout != tc.want || stderr != "" {
				t.Errorf("exit %d, stderr %q, stdout:\n%s\nwant:\n%s", code, stderr, stdout, tc.want)
			}
		})
	}
}

func TestStateList_UnknownRoute(t *testing.T) {
	t.Parallel()

	code, _, stderr := runCLI(t, "state", "list", "users.lst", "--dir", listDir(t))
	want := "error: no route \"users.lst\" (did you mean \"users.list\"?)\n"
	if code != cli.ExitFailure || stderr != want {
		t.Errorf("exit %d, stderr %q; want 1 and %q", code, stderr, want)
	}
}

func TestStateList_ProjectWithErrors(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: get /users\n  states:\n    ok: {}\n")
	code, stdout, stderr := runCLI(t, "state", "list", "--dir", dir)
	want := "routes/users.yaml:2: method \"get\" must be uppercase: GET\n1 problem in 1 file; fix it first\n"
	if code != cli.ExitFailure || stdout != "" || stderr != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1 and %q", code, stdout, stderr, want)
	}
}

func TestStateList_TooManyArguments(t *testing.T) {
	t.Parallel()

	if code, _, _ := runCLI(t, "state", "list", "a", "b"); code != cli.ExitUsage {
		t.Errorf("exit %d, want 2", code)
	}
}
