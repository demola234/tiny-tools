package testkit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestPath_PointsAtModuleTestdata(t *testing.T) {
	t.Parallel()

	got := testkit.Path(t, "script")
	if filepath.Base(filepath.Dir(got)) != "testdata" || filepath.Base(got) != "script" {
		t.Errorf("Path(script) = %q, want a path ending in testdata/script", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("Path(script) = %q, want an absolute path", got)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Errorf("Path(script) = %q, which is not an existing folder (err: %v)", got, err)
	}
}

func TestPath_JoinsElements(t *testing.T) {
	t.Parallel()

	got := testkit.Path(t, "golden", "testkit", "hello.txt")
	want := filepath.Join(testkit.Path(t), "golden", "testkit", "hello.txt")
	if got != want {
		t.Errorf("Path(golden, testkit, hello.txt) = %q, want %q", got, want)
	}
}
