package live

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func render(fs []finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.severity.String() + " " + f.message
	}
	return out
}

func TestCompareShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		expected, actual string
		want             []string
	}{
		{"identical", `{"a":1,"b":"x"}`, `{"a":2,"b":"y"}`, []string{}},
		{
			"missing field", `{"users":[],"nextPage":2}`, `{"users":[]}`,
			[]string{`breaking field "nextPage" is missing live`},
		},
		{
			"extra field", `{"a":1}`, `{"a":1,"avatar":"x"}`,
			[]string{`info live response has extra field "avatar"`},
		},
		{
			"type changed", `{"id":"u_1"}`, `{"id":1}`,
			[]string{`breaking field "id" is a number live, a string in the contract`},
		},
		{
			"null live", `{"name":"Ada"}`, `{"name":null}`,
			[]string{`warning field "name" is null live, a string in the contract`},
		},
		{"null in the contract allows anything", `{"next":null}`, `{"next":"abc"}`, []string{}},
		{
			"nested and in lists", `{"users":[{"id":"u_1","name":"Ada"}]}`, `{"users":[{"id":7}]}`,
			[]string{
				`breaking field "users[].id" is a number live, a string in the contract`,
				`breaking field "users[].name" is missing live`,
			},
		},
		{"empty example list checks nothing inside", `{"users":[]}`, `{"users":[{"id":1}]}`, []string{}},
		{"empty live list checks nothing inside", `{"users":[{"id":"u_1"}]}`, `{"users":[]}`, []string{}},
		{
			"root type", `{"a":1}`, `[1]`,
			[]string{`breaking the response is a list live, an object in the contract`},
		},
		{
			"booleans and numbers", `{"ok":true,"n":1.5}`, `{"ok":"yes","n":2}`,
			[]string{`breaking field "ok" is a string live, a boolean in the contract`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := render(compareShape(decode(t, tc.expected), decode(t, tc.actual)))
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("findings (-want +got):\n%s", d)
			}
		})
	}
}

func TestCompareShape_CapsFindings(t *testing.T) {
	t.Parallel()

	fields := make([]string, 0, 50)
	for i := range 50 {
		fields = append(fields, `"f`+strconv.Itoa(i)+`":1`)
	}
	expected := decode(t, "{"+strings.Join(fields, ",")+"}")
	got := compareShape(expected, decode(t, `{}`))
	if len(got) != maxFindings+1 {
		t.Fatalf("got %d findings, want %d plus a summary", len(got), maxFindings)
	}
	if last := got[len(got)-1].message; last != "and 30 more differences" {
		t.Errorf("last finding = %q", last)
	}
}

func TestCompareShape_StopsAtTheDepthLimit(t *testing.T) {
	t.Parallel()

	deep := strings.Repeat(`{"a":`, maxDepth+5) + `"x"` + strings.Repeat(`}`, maxDepth+5)
	deeper := strings.Repeat(`{"a":`, maxDepth+5) + `1` + strings.Repeat(`}`, maxDepth+5)
	if got := compareShape(decode(t, deep), decode(t, deeper)); len(got) != 0 {
		t.Errorf("findings beyond the depth limit: %v", render(got))
	}
}
