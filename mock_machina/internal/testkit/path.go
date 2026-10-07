package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

func Path(tb testing.TB, elem ...string) string {
	tb.Helper()

	root, err := moduleRoot()
	if err != nil {
		tb.Fatalf("testkit: finding module root: %v", err)
	}
	return filepath.Join(append([]string{root, "testdata"}, elem...)...)
}

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
