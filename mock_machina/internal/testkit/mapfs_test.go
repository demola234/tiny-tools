package testkit_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestMapFS_Dedent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "indented raw string",
			in: `
				id: users.list
				states:
				  success: {}
			`,
			want: "id: users.list\nstates:\n  success: {}\n",
		},
		{
			name: "blank line inside keeps no indent",
			in:   "\n\t\ta: 1\n\n\t\tb: 2\n\t",
			want: "a: 1\n\nb: 2\n",
		},
		{
			name: "whitespace-only line inside becomes empty",
			in:   "\n\t\ta: 1\n\t\t\t\n\t\tb: 2\n",
			want: "a: 1\n\nb: 2\n",
		},
		{
			name: "no indent left alone",
			in:   "a: 1\nb: 2\n",
			want: "a: 1\nb: 2\n",
		},
		{
			name: "no final newline left alone",
			in:   "a: 1",
			want: "a: 1",
		},
		{
			name: "empty file",
			in:   "",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fsys := testkit.MapFS(t, map[string]string{"f.yaml": tc.in})
			got, err := fs.ReadFile(fsys, "f.yaml")
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMapFS_NestedPathsReadable(t *testing.T) {
	t.Parallel()

	fsys := testkit.MapFS(t, map[string]string{
		"routes/users.list/route.yaml":   "id: users.list\n",
		"routes/users.list/success.json": "{}\n",
	})
	entries, err := fs.ReadDir(fsys, "routes/users.list")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("ReadDir(routes/users.list) has %d entries, want 2", len(entries))
	}
}

func TestMapFS_InvalidPathStopsTheTest(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../escape.yaml", "/abs.yaml", "a//b.yaml"} {
		f := runFake(func(tb testing.TB) {
			tb.Helper()
			testkit.MapFS(tb, map[string]string{name: ""})
		})
		if !f.fatal || !strings.Contains(f.output(), name) {
			t.Errorf("MapFS(%q): fatal = %v, message = %q; want a fatal failure naming the path", name, f.fatal, f.output())
		}
	}
}
