package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestFindProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	project := filepath.Join(root, "app", ".mockmachina")
	deep := filepath.Join(root, "app", "lib", "src")
	for _, dir := range []string{project, deep} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, start := range []string{filepath.Join(root, "app"), deep} {
		got, err := cli.FindProject(start)
		if err != nil || got != project {
			t.Errorf("FindProject(%s) = %q, %v; want %q", start, got, err, project)
		}
	}
}

func TestFindProject_NotFound(t *testing.T) {
	t.Parallel()

	_, err := cli.FindProject(t.TempDir())
	if !errors.Is(err, cli.ErrNoProject) {
		t.Errorf("FindProject(empty folder) error = %v, want ErrNoProject", err)
	}
}

func TestFindProject_SkipsAFileWithTheSameName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inner := filepath.Join(root, "inner")
	if err := os.MkdirAll(filepath.Join(root, ".mockmachina"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(inner, ".mockmachina"), "not a folder")

	got, err := cli.FindProject(inner)
	if err != nil || got != filepath.Join(root, ".mockmachina") {
		t.Errorf("FindProject = %q, %v; want the folder in the parent", got, err)
	}
}
