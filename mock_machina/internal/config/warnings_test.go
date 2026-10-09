package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

func TestWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		route string
		want  []string
	}{
		{
			name:  "complete route",
			route: "get:\n  route: GET /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok: {}\n    denied:\n      status: 403\n",
			want:  nil,
		},
		{
			name:  "owners from the top of the file count",
			route: "owners: { frontend: [a] }\nget:\n  route: GET /r\n  summary: R\n  states:\n    ok: {}\n    down:\n      status: 503\n",
			want:  nil,
		},
		{
			name:  "generated route",
			route: "get:\n  route: GET /r\n  summary: R\n  owners: { backend: [a] }\n  generated: true\n  states:\n    ok: {}\n    denied:\n      status: 403\n",
			want: []string{
				`routes/r.yaml:1: warning: r.get was written by an AI assistant; review it, then delete "generated: true"`,
			},
		},
		{
			name:  "generated state",
			route: "get:\n  route: GET /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok: {}\n    denied:\n      status: 403\n      generated: true\n",
			want: []string{
				`routes/r.yaml:7: warning: r.get state "denied" was written by an AI assistant; review it, then delete "generated: true"`,
			},
		},
		{
			name:  "list without an empty state",
			route: "get:\n  route: GET /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok:\n      body: { items: [{ id: 1 }], next: null }\n    denied:\n      status: 403\n",
			want:  []string{"routes/r.yaml:1: warning: r.get has no state with an empty list; apps can't test their empty screen"},
		},
		{
			name:  "list with an empty state",
			route: "get:\n  route: GET /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok:\n      body: { items: [{ id: 1 }] }\n    none:\n      body: { items: [] }\n    denied:\n      status: 403\n",
			want:  nil,
		},
		{
			name:  "list as the whole body",
			route: "get:\n  route: GET /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok:\n      body: [1, 2]\n    denied:\n      status: 403\n",
			want:  []string{"routes/r.yaml:1: warning: r.get has no state with an empty list; apps can't test their empty screen"},
		},
		{
			name:  "one item is not a list",
			route: "get:\n  route: GET /r/{id}\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok:\n      body: { tags: [a] }\n    missing:\n      status: 404\n",
			want:  nil,
		},
		{
			name:  "path parameter without a 404",
			route: "get:\n  route: GET /r/{id}\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok: {}\n    denied:\n      status: 403\n",
			want:  []string{"routes/r.yaml:1: warning: r.get has no 404 state; apps can't test an unknown {id}"},
		},
		{
			name:  "write without a 4xx",
			route: "get:\n  route: POST /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    created:\n      status: 201\n    down:\n      status: 503\n",
			want:  []string{"routes/r.yaml:1: warning: r.get has no 4xx state; apps can't test invalid input"},
		},
		{
			name:  "no error states gives only the general warning",
			route: "get:\n  route: PATCH /r/{id}\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok: {}\n",
			want:  []string{"routes/r.yaml:1: warning: r.get has no 4xx or 5xx state; apps can't test errors"},
		},
		{
			name:  "CRUD routes answer their own errors",
			route: "get:\n  route: CRUD /r\n  summary: R\n  owners: { backend: [a] }\n  states:\n    ok: {}\n",
			want:  nil,
		},
		{
			name:  "nothing optional",
			route: okRoute,
			want: []string{
				"routes/r.yaml:1: warning: r.get has no summary; add one line saying what it returns",
				"routes/r.yaml:1: warning: r.get has no 4xx or 5xx state; apps can't test errors",
				"routes/r.yaml:1: warning: r.get has no owners; reviews can't be routed",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := loadClean(t, map[string]string{"routes/r.yaml": tc.route})
			var got []string
			for _, w := range config.Warnings(p) {
				got = append(got, w.String())
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("warnings (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProblems_HasErrors(t *testing.T) {
	t.Parallel()

	warning := config.Problem{File: "f", Severity: config.Warning, Msg: "w"}
	failure := config.Problem{File: "f", Msg: "e"}
	tests := []struct {
		probs config.Problems
		want  bool
	}{
		{nil, false},
		{config.Problems{warning}, false},
		{config.Problems{warning, failure}, true},
	}
	for _, tc := range tests {
		if got := tc.probs.HasErrors(); got != tc.want {
			t.Errorf("HasErrors(%v) = %v, want %v", tc.probs, got, tc.want)
		}
	}
}
