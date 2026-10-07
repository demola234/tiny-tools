package testkit

import (
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

func Golden(tb testing.TB, got []byte, name string) {
	tb.Helper()

	path := Path(tb, "golden", filepath.FromSlash(name))
	if *update {
		WriteGolden(tb, got, path)
		return
	}
	CompareGolden(tb, got, path)
}

func CompareGolden(tb testing.TB, got []byte, path string) {
	tb.Helper()

	want, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		tb.Fatalf("golden file %s is missing: write it by hand, or run the tests with -update and review the result", path)
		return
	}
	if err != nil {
		tb.Fatalf("reading golden file: %v", err)
		return
	}
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		tb.Errorf("output differs from golden file %s (-want +got):\n%s", path, diff)
	}
}

func WriteGolden(tb testing.TB, got []byte, path string) {
	tb.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatalf("creating golden folder: %v", err)
		return
	}
	if err := os.WriteFile(path, got, 0o644); err != nil {
		tb.Fatalf("writing golden file: %v", err)
	}
}
