package control_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const pageCSP = "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func TestUI_Shell(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, "", map[string]string{
		"routes/users.yaml":       usersYAML,
		"routes/cart.yaml":        cartYAML,
		"scenarios/new-user.yaml": "states: { users.list: empty }\n",
	})
	rec := f.raw(t, "GET", "/__mockmachina/ui/", "", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("GET ui/ = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != pageCSP {
		t.Errorf("CSP %q, want %q", got, pageCSP)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy %q, want same-origin so htmx POSTs carry the real Origin", got)
	}
	testkit.Golden(t, rec.Body.Bytes(), "ui/shell.html")
}

func TestUI_Static(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	pinned := map[string]string{
		"htmx.min.js": "60231ae6ba9db3825eb15a261122d5f55921c4d53b66bf637dc18b4ee27c79f9",
		"sse.min.js":  "204a17ec2bf490b7df592f55ebe547e44b099fb53c88b442d0bced0e10327e12",
	}
	for name, sum := range pinned {
		rec := f.raw(t, "GET", "/__mockmachina/ui/static/"+name, "", nil)
		got := sha256.Sum256(rec.Body.Bytes())
		if rec.Code != 200 || hex.EncodeToString(got[:]) != sum {
			t.Errorf("%s = %d, sha256 %x; want the pinned file %s", name, rec.Code, got, sum)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("%s Content-Type %q", name, ct)
		}
	}
	for name, want := range map[string]string{"app.js": "javascript", "style.css": "text/css"} {
		rec := f.raw(t, "GET", "/__mockmachina/ui/static/"+name, "", nil)
		if ct := rec.Header().Get("Content-Type"); rec.Code != 200 || !strings.Contains(ct, want) {
			t.Errorf("%s = %d %q", name, rec.Code, ct)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") != pageCSP {
			t.Errorf("%s headers %v", name, rec.Header())
		}
	}
	if rec := f.raw(t, "GET", "/__mockmachina/ui/static/nope.js", "", nil); rec.Code != 404 {
		t.Errorf("unknown file = %d, want 404", rec.Code)
	}
}

func TestUI_PostsNeedHTMX(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	code, got := f.call(t, "POST", "/__mockmachina/ui/reset", "", nil)
	if code != 403 || got["error"] != "forbidden" {
		t.Errorf("POST without HX-Request = %d %v, want 403", code, got)
	}
	if code, _ := f.call(t, "POST", "/__mockmachina/ui/reset", "", map[string]string{"HX-Request": "true"}); code == 403 {
		t.Error("POST with HX-Request was refused")
	}
}
