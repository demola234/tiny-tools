package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

// Path returns the absolute path of elem under the module-root testdata folder.
// It does not check that the path exists.
func Path(tb testing.TB, elem ...string) string {
	tb.Helper()

	root, err := moduleRoot()
	if err != nil {
		tb.Fatalf("testkit: finding module root: %v", err)
	}
	return filepath.Join(append([]string{root, "testdata"}, elem...)...)
}

// moduleRoot walks up from the working directory, which go test sets to the
// package folder, to the first folder holding go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
