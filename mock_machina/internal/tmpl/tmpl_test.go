package tmpl_test

import (
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/match"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

func env(seed uint64) tmpl.Env {
	return tmpl.Env{
		Facts: match.Facts{
			Path:    map[string]string{"id": "u_1"},
			Query:   url.Values{"page": {"2"}},
			Header:  http.Header{"X-Name": {`Ada "the" Count`}},
			Cookies: map[string]string{"session": "s1"},
			Body:    map[string]any{"qty": 3.0, "tags": []any{"a", "b"}, "ok": true, "user": map[string]any{"name": "Ada"}},
			Vars:    map[string]any{"plan": "pro"},
			Call:    7,
		},
		Rand: rand.New(rand.NewPCG(seed, 1)),
		Now:  time.Date(2026, 10, 7, 9, 30, 0, 0, time.FixedZone("WAT", 3600)),
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"{{ path.id", `template "{{ path.id" has no closing }}`},
		{"{{ }}", `template "{{ }}" is empty`},
		{"{{ pth.id }}", `unknown template "pth.id" (did you mean "path.id"?)`},
		{"{{ hello }}", `unknown template "hello" (templates: path.<name>, query.<name>, header.<name>, cookie.<name>, body.<a.b.0>, var.<name>, call, uuid, now, now.unix, random.int, fake.*)`},
		{"{{ fake.person.nam }}", `unknown template "fake.person.nam" (did you mean "fake.person.name"?)`},
		{"{{ fake.email 2 }}", `fake.email takes no arguments`},
		{"{{ query }}", `template "query" needs a name, like query.page`},
		{"{{ call.x }}", `template "call.x" takes no name; write call`},
		{"{{ uuid 3 }}", `uuid takes no arguments`},
		{"{{ random.int 1 }}", `random.int needs two whole numbers, like {{ random.int 1 100 }}`},
		{"{{ random.int 9 1 }}", `random.int needs two whole numbers, like {{ random.int 1 100 }}`},
	}
	for _, tc := range tests {
		if _, err := tmpl.Parse(tc.in); err == nil || err.Error() != tc.want {
			t.Errorf("Parse(%q) error = %v\nwant %s", tc.in, err, tc.want)
		}
	}
}

func eval(t *testing.T, s string) any {
	t.Helper()
	tx, err := tmpl.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return tx.Eval(env(1))
}

func TestText_Eval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want any
	}{
		{"plain", "plain"},
		{"{{ path.id }}", "u_1"},
		{"{{path.id}}", "u_1"},
		{"user {{ path.id }} on page {{ query.page }}", "user u_1 on page 2"},
		{"{{ query.page }}", "2"},
		{"{{ body.qty }}", 3.0},
		{"{{ body.ok }}", true},
		{"{{ body.tags.1 }}", "b"},
		{"{{ body.user }}", map[string]any{"name": "Ada"}},
		{"n={{ body.qty }}", "n=3"},
		{"{{ header.x-name }}", `Ada "the" Count`},
		{"{{ cookie.session }}", "s1"},
		{"{{ var.plan }}", "pro"},
		{"{{ call }}", 7.0},
		{"{{ query.missing }}", ""},
		{"[{{ body.nope }}]", "[]"},
		{"{{ now }}", "2026-10-07T08:30:00Z"},
		{"{{ now.unix }}", 1791361800.0},
	}
	for _, tc := range tests {
		if got := eval(t, tc.in); !equal(got, tc.want) {
			t.Errorf("%q = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func equal(a, b any) bool {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		return len(am) == len(bm) && am["name"] == bm["name"]
	}
	return a == b
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestText_Generators(t *testing.T) {
	t.Parallel()

	tx, _ := tmpl.Parse("{{ uuid }}")
	a, b, c := tx.Eval(env(1)), tx.Eval(env(1)), tx.Eval(env(2))
	if s, _ := a.(string); !uuidPattern.MatchString(s) {
		t.Errorf("uuid %q isn't a v4 UUID", a)
	}
	if a != b || a == c {
		t.Errorf("uuid isn't seeded: %v %v %v", a, b, c)
	}
	n, _ := tmpl.Parse("{{ random.int 1 6 }}")
	seen := map[float64]bool{}
	e := env(3)
	for range 600 {
		v, _ := n.Eval(e).(float64)
		if v < 1 || v > 6 || v != float64(int(v)) {
			t.Fatalf("random.int 1 6 gave %v", v)
		}
		seen[v] = true
	}
	if len(seen) != 6 {
		t.Errorf("random.int 1 6 only gave %v", seen)
	}
}

func TestText_UsesBody(t *testing.T) {
	t.Parallel()

	for s, want := range map[string]bool{"{{ body.a }}": true, "x {{ path.id }}": false, "plain": false} {
		tx, _ := tmpl.Parse(s)
		if tx.UsesBody() != want {
			t.Errorf("%q UsesBody = %v", s, !want)
		}
	}
}

func TestJSON_Render(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{`{"z":1,"a":"{{ path.id }}","m":[{"q":"{{ body.qty }}"},"{{ body.ok }}"]}`, `{"z":1,"a":"u_1","m":[{"q":3},true]}`},
		{`{"name":"{{ header.x-name }}","n":"line\n{{ call }}"}`, `{"name":"Ada \"the\" Count","n":"line\n7"}`},
		{`{ "spaced" : [ 1 , 2 ] , "t" : "{{ query.page }}" }`, `{"spaced":[1,2],"t":"2"}`},
		{`{"{{ path.id }}":"keys stay as they are","u":"{{ body.user }}","x":null,"f":1.5e3}`, `{"{{ path.id }}":"keys stay as they are","u":{"name":"Ada"},"x":null,"f":1.5e3}`},
		{`["{{ var.plan }}", "{{ query.none }}"]`, `["pro",""]`},
		{`{"html":"<b>&</b>"}`, `{"html":"<b>&</b>"}`},
	}
	for _, tc := range tests {
		j, err := tmpl.ParseJSON([]byte(tc.in))
		if err != nil {
			t.Fatalf("ParseJSON(%s): %v", tc.in, err)
		}
		if got := string(j.Render(env(1))); got != tc.want {
			t.Errorf("Render(%s)\n = %s\nwant %s", tc.in, got, tc.want)
		}
	}
}

func TestJSON_Errors(t *testing.T) {
	t.Parallel()

	if _, err := tmpl.ParseJSON([]byte(`{"a":"{{ nope }}"}`)); err == nil || err.Error() != `unknown template "nope" (did you mean "now"?)` {
		t.Errorf("error = %v", err)
	}
	if _, err := tmpl.ParseJSON([]byte(`{"a":`)); err == nil {
		t.Error("invalid JSON gave no error")
	}
}

func TestHas(t *testing.T) {
	t.Parallel()

	if !tmpl.Has([]byte(`{"a":"{{ x }}"}`)) || tmpl.Has([]byte(`{"a":"x"}`)) {
		t.Error("Has is wrong")
	}
}

func FuzzParse(f *testing.F) {
	f.Add("{{ path.id }} and {{ random.int 1 2 }}")
	f.Add("{{{{}}}}")
	f.Fuzz(func(_ *testing.T, s string) {
		if tx, err := tmpl.Parse(s); err == nil {
			tx.Eval(env(1))
		}
		if j, err := tmpl.ParseJSON([]byte(s)); err == nil {
			j.Render(env(1))
		}
	})
}

func TestUsesRandom(t *testing.T) {
	t.Parallel()

	for s, want := range map[string]bool{"{{ uuid }}": true, "n {{ random.int 1 2 }}": true, "{{ now }}": false, "{{ path.id }}": false} {
		tx, _ := tmpl.Parse(s)
		if tx.UsesRandom() != want {
			t.Errorf("%q UsesRandom = %v", s, !want)
		}
	}
	j, _ := tmpl.ParseJSON([]byte(`{"a":["{{ uuid }}"]}`))
	k, _ := tmpl.ParseJSON([]byte(`{"a":["{{ path.id }}"]}`))
	if !j.UsesRandom() || k.UsesRandom() {
		t.Error("JSON UsesRandom is wrong")
	}
}

func TestText_Fake(t *testing.T) {
	t.Parallel()

	name, _ := tmpl.Parse("{{ fake.person.name }}")
	e := env(4)
	e.Locale = "en_NG"
	v, ok := name.Eval(e).(string)
	if !ok || v == "" {
		t.Errorf("fake.person.name = %#v", v)
	}
	price, _ := tmpl.Parse("{{ fake.price }}")
	if _, ok := price.Eval(env(1)).(float64); !ok {
		t.Error("fake.price isn't a number")
	}
	if !name.UsesRandom() {
		t.Error("fake.* should count as random")
	}
}
