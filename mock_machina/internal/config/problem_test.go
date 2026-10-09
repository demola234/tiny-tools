package config_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

func TestProblem_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		problem config.Problem
		want    string
	}{
		{config.Problem{File: "routes/a/route.yaml", Line: 7, Msg: "bad"}, "routes/a/route.yaml:7: bad"},
		{config.Problem{File: "routes/a", Msg: "no route.yaml"}, "routes/a: no route.yaml"},
	}
	for _, tc := range tests {
		if got := tc.problem.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestProblems_Summary(t *testing.T) {
	t.Parallel()

	a := config.Problem{File: "routes/a/route.yaml", Line: 1, Msg: "x"}
	b := config.Problem{File: "routes/b/route.yaml", Line: 2, Msg: "y"}
	tests := []struct {
		probs config.Problems
		want  string
	}{
		{nil, "no problems"},
		{config.Problems{a}, "1 problem in 1 file"},
		{config.Problems{a, a}, "2 problems in 1 file"},
		{config.Problems{a, b, a}, "3 problems in 2 files"},
	}
	for _, tc := range tests {
		if got := tc.probs.Summary(); got != tc.want {
			t.Errorf("Summary() of %d problems = %q, want %q", len(tc.probs), got, tc.want)
		}
	}
}

func TestProblems_Sorted(t *testing.T) {
	t.Parallel()

	probs := config.Problems{
		{File: "routes/b/route.yaml", Line: 2, Msg: "b2"},
		{File: "routes/a/route.yaml", Line: 10, Msg: "a10"},
		{File: "routes/a/route.yaml", Line: 9, Msg: "a9"},
		{File: "routes/a/route.yaml", Line: 9, Msg: "a9 second"},
	}
	got := probs.Sorted()
	want := []string{"a9", "a9 second", "a10", "b2"}
	for i, p := range got {
		if p.Msg != want[i] {
			t.Errorf("Sorted()[%d] = %q, want %q", i, p.Msg, want[i])
		}
	}
	if probs[0].Msg != "b2" {
		t.Error("Sorted() changed the original order")
	}
}
