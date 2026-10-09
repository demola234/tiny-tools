package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestLint(t *testing.T) {
	t.Parallel()

	users := testkit.Path(t, "projects", "users", ".mockmachina")
	broken := testkit.Path(t, "projects", "broken", ".mockmachina")
	usersWarnings := "routes/users.yaml:1: warning: users.list has no summary; add one line saying what it returns\n" +
		"routes/users.yaml:1: warning: users.list has no owners; reviews can't be routed\n"

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{
			name:       "warnings only pass",
			args:       []string{"lint", "--dir", users},
			wantCode:   cli.ExitOK,
			wantStdout: usersWarnings + "2 problems in 1 file\n",
		},
		{
			name:       "strict fails on warnings",
			args:       []string{"lint", "--dir", users, "--strict"},
			wantCode:   cli.ExitFailure,
			wantStdout: usersWarnings + "2 problems in 1 file\n",
		},
		{
			name:     "errors fail",
			args:     []string{"lint", "--dir", broken},
			wantCode: cli.ExitFailure,
			wantStdout: `routes/users.yaml:1: warning: users.list has no summary; add one line saying what it returns` + "\n" +
				`routes/users.yaml:1: warning: users.list has no 4xx or 5xx state; apps can't test errors` + "\n" +
				`routes/users.yaml:1: warning: users.list has no owners; reviews can't be routed` + "\n" +
				`routes/users.yaml:3: active state "empt" doesn't exist (did you mean "empty"?)` + "\n" +
				"4 problems in 1 file\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runCLI(t, tc.args...)
			if code != tc.wantCode || stdout != tc.wantStdout || stderr != "" {
				t.Errorf("exit %d, stdout:\n%s\nstderr %q\nwant exit %d, stdout:\n%s", code, stdout, stderr, tc.wantCode, tc.wantStdout)
			}
		})
	}
}

func TestLint_CleanProject(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "health.yaml"),
		"owners: { backend: [a] }\nget:\n  route: GET /health\n  summary: Health check\n  states:\n    ok: {}\n    down:\n      status: 503\n")
	code, stdout, _ := runCLI(t, "lint", "--dir", dir)
	if code != cli.ExitOK || stdout != "no problems\n" {
		t.Errorf("exit %d, stdout %q; want 0 and no problems", code, stdout)
	}
}

func TestLint_MissingFolder(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nope")
	code, _, stderr := runCLI(t, "lint", "--dir", dir)
	if code != cli.ExitFailure || stderr == "" {
		t.Errorf("exit %d, stderr %q; want 1 and an error", code, stderr)
	}
}

func TestLint_ExtraArgumentsAreUsageErrors(t *testing.T) {
	t.Parallel()

	if code, _, _ := runCLI(t, "lint", "extra"); code != cli.ExitUsage {
		t.Errorf("exit %d, want 2", code)
	}
}
