package suggest_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

func TestDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"/usrs", "/users", 1},
		{"café", "cafe", 1},
		{"", "abc", 3},
		{"/a/very/long/path", "/b", 16},
	}
	for _, tc := range tests {
		if got := suggest.Distance(tc.a, tc.b); got != tc.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestClosest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		got        string
		candidates []string
		want       string
		ok         bool
	}{
		{"empyt", []string{"success", "empty"}, "empty", true},
		{"mehtod", []string{"id", "method", "path"}, "method", true},
		{"xyz", []string{"success", "empty"}, "", false},
		{"empty", []string{"empty"}, "empty", true},
		{"a", nil, "", false},
		{"ab", []string{"aa", "bb"}, "aa", true},
		{"Empty", []string{"empty"}, "empty", true},
		{"x", []string{"a-very-long-candidate-name"}, "", false},
		{"abcdef", []string{"uvwxyz"}, "", false},
	}
	for _, tc := range tests {
		got, ok := suggest.Closest(tc.got, tc.candidates)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Closest(%q, %v) = %q, %v; want %q, %v", tc.got, tc.candidates, got, ok, tc.want, tc.ok)
		}
	}
}

func BenchmarkClosest(b *testing.B) {
	candidates := []string{"route", "summary", "status", "owners", "active", "states", "headers", "body", "latency"}
	long := string(make([]byte, 10_000))
	for b.Loop() {
		suggest.Closest("statse", candidates)
		suggest.Closest("statse", []string{long})
	}
}
