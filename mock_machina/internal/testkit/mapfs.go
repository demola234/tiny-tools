package testkit

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func MapFS(tb testing.TB, files map[string]string) fstest.MapFS {
	tb.Helper()

	fsys := make(fstest.MapFS, len(files))
	for name, content := range files {
		if !fs.ValidPath(name) {
			tb.Fatalf("testkit.MapFS: %q is not a valid fs.FS path", name)
		}
		fsys[name] = &fstest.MapFile{Data: []byte(dedent(content))}
	}
	return fsys
}

func dedent(s string) string {
	s = strings.TrimPrefix(s, "\n")
	lines := strings.Split(s, "\n")

	prefix, found := "", false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if !found {
			prefix, found = indent, true
			continue
		}
		prefix = commonPrefix(prefix, indent)
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = line[len(prefix):]
	}
	return strings.Join(lines, "\n")
}

func commonPrefix(a, b string) string {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}
