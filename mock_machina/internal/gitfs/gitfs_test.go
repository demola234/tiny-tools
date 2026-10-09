package gitfs_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/gitfs"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "app", ".mockmachina")
	git(t, root, "init", "-q", "-b", "main")
	write(t, filepath.Join(root, "README.md"), "app")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "before the contract")
	write(t, filepath.Join(project, "routes", "users.yaml"), "v1")
	write(t, filepath.Join(project, "routes", "users", "users.json"), "{}")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "v1")
	write(t, filepath.Join(project, "routes", "users.yaml"), "v2")
	git(t, root, "commit", "-q", "-am", "v2")
	return project
}

func read(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAt(t *testing.T) {
	t.Parallel()

	project := repo(t)
	for ref, want := range map[string]string{"HEAD": "v2", "HEAD~1": "v1", "main": "v2"} {
		fsys, err := gitfs.At(t.Context(), project, ref)
		if err != nil {
			t.Fatalf("At(%s): %v", ref, err)
		}
		if got := read(t, fsys, "routes/users.yaml"); got != want {
			t.Errorf("At(%s) routes/users.yaml = %q, want %q", ref, got, want)
		}
		if got := read(t, fsys, "routes/users/users.json"); got != "{}" {
			t.Errorf("At(%s) body file = %q", ref, got)
		}
	}
}

func TestAt_FromASubfolderOfTheProject(t *testing.T) {
	t.Parallel()

	project := repo(t)
	fsys, err := gitfs.At(t.Context(), project+string(filepath.Separator), "HEAD")
	if err != nil || read(t, fsys, "routes/users.yaml") != "v2" {
		t.Errorf("At with a trailing separator: %v", err)
	}
}

func TestAt_RefBeforeTheContractExisted(t *testing.T) {
	t.Parallel()

	project := repo(t)
	fsys, err := gitfs.At(t.Context(), project, "HEAD~2")
	if err != nil {
		t.Fatalf("At(HEAD~2): %v", err)
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil || len(entries) != 0 {
		t.Errorf("At(HEAD~2) = %v entries, %v; want an empty project", len(entries), err)
	}
}

func TestAt_Errors(t *testing.T) {
	t.Parallel()

	project := repo(t)
	tests := []struct{ dir, ref, want string }{
		{project, "no-such-branch", `unknown git ref "no-such-branch"`},
		{project, "--output=/tmp/x", `"--output=/tmp/x" isn't a git ref`},
		{t.TempDir(), "HEAD", "isn't inside a git repository"},
	}
	for _, tc := range tests {
		_, err := gitfs.At(t.Context(), tc.dir, tc.ref)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("At(%s, %s) error = %v, want it to contain %q", tc.dir, tc.ref, err, tc.want)
		}
	}
}

func TestAt_MissingFolder(t *testing.T) {
	t.Parallel()

	_, err := gitfs.At(t.Context(), filepath.Join(t.TempDir(), "missing"), "HEAD")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("At(missing folder) error = %v, want fs.ErrNotExist", err)
	}
}

func TestPathInRepo(t *testing.T) {
	t.Parallel()

	project := repo(t)
	got, err := gitfs.PathInRepo(t.Context(), project)
	if err != nil || got != "app/.mockmachina" {
		t.Errorf("PathInRepo = %q, %v; want app/.mockmachina", got, err)
	}
	if _, err := gitfs.PathInRepo(t.Context(), t.TempDir()); err == nil {
		t.Error("PathInRepo outside a repository returned no error")
	}
}

func TestAt_RelativeProjectPath(t *testing.T) {
	project := repo(t)
	t.Chdir(filepath.Dir(filepath.Dir(project)))
	fsys, err := gitfs.At(t.Context(), filepath.Join("app", ".mockmachina"), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, fsys, "routes/users.yaml"); got != "v2" {
		t.Errorf("routes/users.yaml = %q, want v2", got)
	}
}
