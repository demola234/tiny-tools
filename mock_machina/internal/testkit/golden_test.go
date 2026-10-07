package testkit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestCompareGolden_Matches(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "hello\n")
	f := runFake(func(tb testing.TB) {
		tb.Helper()
		testkit.CompareGolden(tb, []byte("hello\n"), path)
	})
	if f.failed {
		t.Errorf("CompareGolden reported a failure for matching content:\n%s", f.output())
	}
}

func TestCompareGolden_MismatchShowsDiff(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "first\nhello\nlast\n")
	f := runFake(func(tb testing.TB) {
		tb.Helper()
		testkit.CompareGolden(tb, []byte("first\nbye\nlast\n"), path)
	})
	if !f.failed {
		t.Fatal("CompareGolden reported no failure for different content")
	}

	for _, want := range []string{"(-want +got)", "hello", "bye", path} {
		if !strings.Contains(f.output(), want) {
			t.Errorf("failure message = %q, want it to contain %q", f.output(), want)
		}
	}
}

func TestCompareGolden_LongFileShowsOnlyTheChange(t *testing.T) {
	t.Parallel()

	var want, got strings.Builder
	for i := range 200 {
		line := fmt.Sprintf("line %d: some typical golden output\n", i)
		want.WriteString(line)
		if i == 150 {
			line = "line 150: changed\n"
		}
		got.WriteString(line)
	}
	path := writeTemp(t, want.String())
	f := runFake(func(tb testing.TB) {
		tb.Helper()
		testkit.CompareGolden(tb, []byte(got.String()), path)
	})
	if !f.failed {
		t.Fatal("CompareGolden reported no failure for different content")
	}
	if !strings.Contains(f.output(), "line 150: changed") {
		t.Errorf("failure message doesn't show the changed line:\n%s", f.output())
	}
	if len(f.output()) > want.Len()/4 {
		t.Errorf("failure message is %d bytes for a one-line change in a %d-byte file; want only the change and its context",
			len(f.output()), want.Len())
	}
}

func TestCompareGolden_MissingFileIsFatalWithHint(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.txt")
	f := runFake(func(tb testing.TB) {
		tb.Helper()
		testkit.CompareGolden(tb, []byte("x"), path)
	})
	if !f.fatal {
		t.Fatal("CompareGolden did not stop the test for a missing golden file")
	}
	if !strings.Contains(f.output(), "-update") {
		t.Errorf("failure message = %q, want it to mention -update", f.output())
	}
}

func TestWriteGolden_CreatesFoldersAndFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "new.txt")
	f := runFake(func(tb testing.TB) {
		tb.Helper()
		testkit.WriteGolden(tb, []byte("fresh\n"), path)
	})
	if f.failed {
		t.Fatalf("WriteGolden failed:\n%s", f.output())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written golden file: %v", err)
	}
	if string(got) != "fresh\n" {
		t.Errorf("written content = %q, want %q", got, "fresh\n")
	}
}

func TestGolden_ReadsFromTestdataGolden(t *testing.T) {
	t.Parallel()

	testkit.Golden(t, []byte("hello from testkit\n"), "testkit/hello.txt")
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "golden.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp golden file: %v", err)
	}
	return path
}
