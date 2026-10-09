package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

const (
	baseUsers = "list:\n  route: GET /users\n  states:\n    success: {}\n    empty: {}\n"
	headUsers = "list:\n  route: GET /users\n  states:\n    success: {}\nget:\n  route: GET /users/{id}\n  states:\n    found: {}\n"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func diffRepo(t *testing.T) (root, project string) {
	t.Helper()
	root = t.TempDir()
	project = filepath.Join(root, ".mockmachina")
	gitIn(t, root, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(project, "routes", "users.yaml"), baseUsers)
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "base")
	writeFile(t, filepath.Join(project, "routes", "users.yaml"), headUsers)
	return root, project
}

func TestDiff_Formats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "text",
			args: nil,
			want: "2 changes between HEAD and the working tree: 1 breaking, 1 safe\n" +
				"breaking  users.list: state \"empty\" removed\n" +
				"safe      users.get: route added (GET /users/{id})\n",
		},
		{
			name: "json",
			args: []string{"--format", "json"},
			want: `{"base":"HEAD","head":"working tree","changes":[` +
				`{"severity":"breaking","route":"users.list","state":"empty","message":"state \"empty\" removed","file":"routes/users.yaml","line":1},` +
				`{"severity":"safe","route":"users.get","message":"route added (GET /users/{id})","file":"routes/users.yaml","line":5}]}` + "\n",
		},
		{
			name: "markdown",
			args: []string{"--format", "markdown"},
			want: "### MockMachina contract changes\n\n" +
				"2 changes between `HEAD` and the working tree: **1 breaking**, 1 safe.\n\n" +
				"| Severity | Route | Change |\n" +
				"| --- | --- | --- |\n" +
				"| **breaking** | `users.list` | state \"empty\" removed |\n" +
				"| safe | `users.get` | route added (GET /users/{id}) |\n",
		},
		{
			name: "github",
			args: []string{"--format", "github"},
			want: "::error file=.mockmachina/routes/users.yaml,line=1,title=Breaking contract change::users.list: state \"empty\" removed\n" +
				"::notice file=.mockmachina/routes/users.yaml,line=5,title=Contract change::users.get: route added (GET /users/{id})\n",
		},
	}
	_, project := diffRepo(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runCLI(t, append([]string{"diff", "--dir", project}, tc.args...)...)
			if code != cli.ExitOK || stdout != tc.want || stderr != "" {
				t.Errorf("exit %d, stderr %q, stdout:\n%s\nwant:\n%s", code, stderr, stdout, tc.want)
			}
		})
	}
}

func TestDiff_NoChanges(t *testing.T) {
	t.Parallel()

	root, project := diffRepo(t)
	gitIn(t, root, "commit", "-q", "-am", "head")
	for format, want := range map[string]string{
		"text":     "no contract changes between HEAD and the working tree\n",
		"markdown": "### MockMachina contract changes\n\nNo contract changes between `HEAD` and the working tree.\n",
		"json":     `{"base":"HEAD","head":"working tree","changes":[]}` + "\n",
		"github":   "",
	} {
		code, stdout, _ := runCLI(t, "diff", "--dir", project, "--format", format)
		if code != cli.ExitOK || stdout != want {
			t.Errorf("%s: exit %d, stdout %q; want %q", format, code, stdout, want)
		}
	}
}

func TestDiff_BetweenRefs(t *testing.T) {
	t.Parallel()

	root, project := diffRepo(t)
	gitIn(t, root, "commit", "-q", "-am", "head")
	code, stdout, _ := runCLI(t, "diff", "HEAD~1..HEAD", "--dir", project)
	if code != cli.ExitOK || !strings.HasPrefix(stdout, "2 changes between HEAD~1 and HEAD: 1 breaking, 1 safe\n") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
	code, stdout, _ = runCLI(t, "diff", "main", "--dir", project)
	if code != cli.ExitOK || stdout != "no contract changes between main and the working tree\n" {
		t.Errorf("diff main: exit %d, stdout %q", code, stdout)
	}
}

func TestDiff_FailOn(t *testing.T) {
	t.Parallel()

	_, project := diffRepo(t)
	tests := []struct {
		failOn string
		want   int
	}{
		{"none", cli.ExitOK},
		{"breaking", cli.ExitFailure},
		{"warning", cli.ExitFailure},
	}
	for _, tc := range tests {
		code, stdout, stderr := runCLI(t, "diff", "--dir", project, "--fail-on", tc.failOn)
		if code != tc.want || !strings.Contains(stdout, "1 breaking") || stderr != "" {
			t.Errorf("--fail-on %s: exit %d (want %d), stdout %q, stderr %q", tc.failOn, code, tc.want, stdout, stderr)
		}
	}

	root, safeOnly := diffRepo(t)
	gitIn(t, root, "commit", "-q", "-am", "head")
	writeFile(t, filepath.Join(safeOnly, "routes", "health.yaml"), "get:\n  route: GET /health\n  states:\n    up: {}\n")
	if code, _, _ := runCLI(t, "diff", "--dir", safeOnly, "--fail-on", "warning"); code != cli.ExitOK {
		t.Errorf("--fail-on warning with only safe changes: exit %d, want 0", code)
	}
}

func TestDiff_Errors(t *testing.T) {
	t.Parallel()

	_, project := diffRepo(t)
	notGit := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(notGit, "routes", "users.yaml"), baseUsers)
	brokenRoot, broken := diffRepo(t)
	writeFile(t, filepath.Join(broken, "routes", "users.yaml"), "list:\n  route: get /users\n  states:\n    ok: {}\n")
	_ = brokenRoot

	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"unknown format", []string{"diff", "--dir", project, "--format", "xml"}, cli.ExitUsage, ""},
		{"unknown fail-on", []string{"diff", "--dir", project, "--fail-on", "always"}, cli.ExitUsage, ""},
		{"empty base", []string{"diff", "..HEAD", "--dir", project}, cli.ExitUsage, ""},
		{"too many arguments", []string{"diff", "a", "b", "--dir", project}, cli.ExitUsage, ""},
		{"unknown ref", []string{"diff", "nope", "--dir", project}, cli.ExitFailure, `error: unknown git ref "nope"`},
		{"not a git repository", []string{"diff", "--dir", notGit}, cli.ExitFailure, "isn't inside a git repository"},
		{
			"head has errors",
			[]string{"diff", "--dir", broken},
			cli.ExitFailure,
			"error: the contract in the working tree has problems; fix them first:\n  routes/users.yaml:2: method \"get\" must be uppercase: GET",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, _, stderr := runCLI(t, tc.args...)
			if code != tc.code || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d (want %d), stderr %q; want it to contain %q", code, tc.code, stderr, tc.want)
			}
		})
	}
}
