package seed_test

import (
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
)

func first(s seed.Source, parts ...string) [3]uint64 {
	r := s.Stream(parts...)
	return [3]uint64{r.Uint64(), r.Uint64(), r.Uint64()}
}

func TestStream_SameInputsGiveTheSameNumbers(t *testing.T) {
	t.Parallel()

	if a, b := first(seed.New(42), "users.get", "1"), first(seed.New(42), "users.get", "1"); a != b {
		t.Errorf("%v != %v", a, b)
	}
}

func TestStream_EachInputChangesTheNumbers(t *testing.T) {
	t.Parallel()

	base := first(seed.New(42), "users.get", "1")
	for name, got := range map[string][3]uint64{
		"another seed":      first(seed.New(43), "users.get", "1"),
		"another route":     first(seed.New(42), "users.list", "1"),
		"another call":      first(seed.New(42), "users.get", "2"),
		"parts split apart": first(seed.New(42), "users.get1"),
		"parts regrouped":   first(seed.New(42), "users.ge", "t1"),
	} {
		if got == base {
			t.Errorf("%s gave the same numbers", name)
		}
	}
}

func TestPick_IsNeverZero(t *testing.T) {
	t.Parallel()

	for range 1000 {
		if seed.Pick() == 0 {
			t.Fatal("Pick returned 0, which means pick a seed")
		}
	}
}

func TestSource_Value(t *testing.T) {
	t.Parallel()

	if got := seed.New(42).Value(); got != 42 {
		t.Errorf("Value() = %d, want 42", got)
	}
}

func TestUUID(t *testing.T) {
	t.Parallel()

	a, b := seed.UUID(seed.New(1).Stream("x")), seed.UUID(seed.New(1).Stream("x"))
	if a != b || len(a) != 36 || a[14] != '4' || !strings.ContainsRune("89ab", rune(a[19])) {
		t.Errorf("UUID = %q, %q; want the same v4 UUID twice", a, b)
	}
}
