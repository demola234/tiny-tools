package control_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const uiRoutesYAML = `list:
  route: GET /users
  summary: List users
  active: full
  states:
    full:
      body: [{ id: 1 }]
    empty:
      status: 204
get:
  route: GET /users/{id}
  mode: rules
  rules:
    - when: { path.id: u_404 }
      state: missing
  states:
    found:
      headers: { Cache-Control: no-store }
      latency: 120ms
      body: { id: u_1 }
    missing:
      status: 404
`

func uiFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWith(t, "", map[string]string{"routes/users.yaml": uiRoutesYAML, "routes/cart.yaml": cartYAML})
}

var htmx = map[string]string{"HX-Request": "true", "Content-Type": "application/x-www-form-urlencoded"}

func TestUIRoutes_Pages(t *testing.T) {
	t.Parallel()

	f := uiFixture(t)
	for target, golden := range map[string]string{
		"/__mockmachina/ui/routes":               "ui/routes.html",
		"/__mockmachina/ui/routes?q=cart":        "ui/routes-filtered.html",
		"/__mockmachina/ui/routes?protocol=CRUD": "ui/routes-crud.html",
		"/__mockmachina/ui/routes/users.get":     "ui/route-pane.html",
	} {
		rec := f.raw(t, "GET", target, "", nil)
		if rec.Code != 200 {
			t.Errorf("GET %s = %d", target, rec.Code)
		}
		testkit.Golden(t, rec.Body.Bytes(), golden)
	}
}

func TestUIRoutes_UnknownRoute(t *testing.T) {
	t.Parallel()

	rec := uiFixture(t).raw(t, "GET", "/__mockmachina/ui/routes/users.lst", "", nil)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "did you mean &#34;users.list&#34;?") {
		t.Errorf("unknown route = %d\n%s", rec.Code, rec.Body)
	}
}

func TestUIRoutes_Pin(t *testing.T) {
	t.Parallel()

	f := uiFixture(t)
	rec := f.raw(t, "POST", "/__mockmachina/ui/routes/users.list/pin", "state=empty", htmx)
	if got, ok := f.mem.Pinned("users.list"); rec.Code != 200 || !ok || got != "empty" {
		t.Fatalf("pin = %d, pinned %q %v", rec.Code, got, ok)
	}
	testkit.Golden(t, rec.Body.Bytes(), "ui/route-pinned.html")

	rec = f.raw(t, "POST", "/__mockmachina/ui/routes/users.list/pin", "state=emty", htmx)
	if !strings.Contains(rec.Body.String(), "users.list has no state &#34;emty&#34; (did you mean &#34;empty&#34;?)") {
		t.Errorf("bad state didn't explain itself:\n%s", rec.Body)
	}

	f.raw(t, "POST", "/__mockmachina/ui/routes/users.list/unpin", "", htmx)
	if _, ok := f.mem.Pinned("users.list"); ok {
		t.Error("unpin left the pin")
	}
}

func TestUIRoutes_MakeDefault(t *testing.T) {
	t.Parallel()

	f := uiFixture(t)
	f.mem.Pin("users.list", "full")
	rec := f.raw(t, "POST", "/__mockmachina/ui/routes/users.list/default", "state=empty", htmx)
	data, err := os.ReadFile(filepath.Join(f.dir, "routes", "users.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || !strings.Contains(string(data), "active: empty") {
		t.Errorf("default = %d, file:\n%s", rec.Code, data)
	}
	if _, ok := f.mem.Pinned("users.list"); ok {
		t.Error("make default left the pin")
	}
}
