package cli_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

const liveContract = `list:
  route: GET /users
  states:
    success:
      body: { users: [{ id: u_1 }], nextPage: 2 }
create:
  route: POST /users
  states:
    created:
      status: 201
`

func liveAPI(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t0ken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[{"id":"u_9"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func liveProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), liveContract)
	return dir
}

func TestDiffLive(t *testing.T) {
	t.Parallel()

	url := liveAPI(t)
	dir := liveProject(t)
	auth := []string{"--header", "Authorization: Bearer t0ken"}
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{
			name: "text",
			args: auth,
			code: cli.ExitOK,
			want: "2 differences between the contract and " + url + ": 1 breaking, 1 info\n" +
				"breaking  users.list: state \"success\": field \"nextPage\" is missing live\n" +
				"info      users.create: not checked: POST isn't sent to a live API without --include-writes\n",
		},
		{
			name: "markdown",
			args: append([]string{"--format", "markdown"}, auth...),
			code: cli.ExitOK,
			want: "### MockMachina contract changes\n\n" +
				"2 differences between the contract and `" + url + "`: **1 breaking**, 1 info.\n\n" +
				"| Severity | Route | Change |\n| --- | --- | --- |\n" +
				"| **breaking** | `users.list` | state \"success\": field \"nextPage\" is missing live |\n" +
				"| info | `users.create` | not checked: POST isn't sent to a live API without --include-writes |\n",
		},
		{
			name: "json",
			args: append([]string{"--format", "json"}, auth...),
			code: cli.ExitOK,
			want: `{"base":"contract","head":"` + url + `","changes":[` +
				`{"severity":"breaking","route":"users.list","state":"success","message":"state \"success\": field \"nextPage\" is missing live","file":"routes/users.yaml","line":1},` +
				`{"severity":"info","route":"users.create","message":"not checked: POST isn't sent to a live API without --include-writes","file":"routes/users.yaml","line":6}]}` + "\n",
		},
		{
			name: "without auth, every state is missed",
			args: nil,
			code: cli.ExitOK,
			want: "2 differences between the contract and " + url + ": 1 breaking, 1 info\n" +
				"breaking  users.list: live API returned 401; no state returns it (states return 200)\n" +
				"info      users.create: not checked: POST isn't sent to a live API without --include-writes\n",
		},
		{
			name: "fail on breaking",
			args: append([]string{"--fail-on", "breaking"}, auth...),
			code: cli.ExitFailure,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"diff", "--dir", dir, "--live", url}, tc.args...)
			code, stdout, stderr := runCLI(t, args...)
			if code != tc.code || (tc.want != "" && stdout != tc.want) || stderr != "" {
				t.Errorf("exit %d (want %d), stderr %q, stdout:\n%s\nwant:\n%s", code, tc.code, stderr, stdout, tc.want)
			}
		})
	}
}

func TestDiffLive_NoDifferences(t *testing.T) {
	t.Parallel()

	url := liveAPI(t)
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: GET /users\n  states:\n    success:\n      body: { users: [{ id: u_1 }] }\n")
	code, stdout, _ := runCLI(t, "diff", "--dir", dir, "--live", url, "--header", "Authorization: Bearer t0ken")
	if code != cli.ExitOK || stdout != "no differences between the contract and "+url+"\n" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestDiffLive_ParamsAndWrites(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "invite:\n  route: POST /users/{id}/invite\n  states:\n    created:\n      status: 201\n")

	code, stdout, _ := runCLI(t, "diff", "--dir", dir, "--live", srv.URL, "--include-writes", "--param", "id=u_7", "--timeout", "2s")
	if code != cli.ExitOK || !strings.HasPrefix(stdout, "no differences") || len(paths) != 1 || paths[0] != "POST /users/u_7/invite" {
		t.Errorf("exit %d, stdout %q, requests %v", code, stdout, paths)
	}
}

func TestDiffLive_UsageErrors(t *testing.T) {
	t.Parallel()

	dir := liveProject(t)
	for _, args := range [][]string{
		{"diff", "main", "--live", "http://localhost:1"},
		{"diff", "--live", "not a url"},
		{"diff", "--live", "ftp://example.com"},
		{"diff", "--live", "http://localhost:1", "--header", "no colon"},
		{"diff", "--live", "http://localhost:1", "--param", "noequals"},
	} {
		if code, _, stderr := runCLI(t, append(args, "--dir", dir)...); code != cli.ExitUsage {
			t.Errorf("mockmachina %v: exit %d (%q), want 2", args, code, stderr)
		}
	}
}
