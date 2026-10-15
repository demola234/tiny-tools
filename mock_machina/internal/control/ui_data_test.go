package control_test

import (
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func dataFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWith(t, "", map[string]string{
		"routes/users.yaml": uiRoutesYAML,
		"routes/cart.yaml":  cartYAML,
		"data/cart.json":    `[{"id":"c_1","sku":"TEE","qty":1,"price":{"kobo":1250000}},{"id":"c/2","sku":"SCARF","note":"gift"}]`,
	})
}

func TestUIData_Pages(t *testing.T) {
	t.Parallel()

	f := dataFixture(t)
	for target, golden := range map[string]string{
		"/__mockmachina/ui/data":      "ui/data.html",
		"/__mockmachina/ui/data/cart": "ui/data-cart.html",
	} {
		rec := f.raw(t, "GET", target, "", nil)
		if rec.Code != 200 {
			t.Errorf("GET %s = %d", target, rec.Code)
		}
		testkit.Golden(t, rec.Body.Bytes(), golden)
	}
	rec := f.raw(t, "GET", "/__mockmachina/ui/data/carts", "", nil)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "did you mean &#34;cart&#34;?") {
		t.Errorf("unknown collection = %d", rec.Code)
	}
	rec = newFixture(t, "").raw(t, "GET", "/__mockmachina/ui/data", "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "No CRUD collections") {
		t.Errorf("no collections = %d", rec.Code)
	}
}

func TestUIData_Delete(t *testing.T) {
	t.Parallel()

	f := dataFixture(t)
	rec := f.raw(t, "POST", "/__mockmachina/ui/data/cart/c%2F2/delete", "", htmx)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "SCARF") {
		t.Errorf("delete c/2 = %d, item still shown", rec.Code)
	}
	_, got := f.call(t, "GET", "/__mockmachina/collections/cart", "", nil)
	if got["count"] != float64(1) {
		t.Errorf("after delete, count %v", got["count"])
	}
	rec = f.raw(t, "POST", "/__mockmachina/ui/data/cart/c_9/delete", "", htmx)
	if !strings.Contains(rec.Body.String(), "cart has no item &#34;c_9&#34;") {
		t.Errorf("deleting a missing item didn't explain itself:\n%s", rec.Body)
	}
}

func TestUI_Reset(t *testing.T) {
	t.Parallel()

	f := dataFixture(t)
	f.mem.Pin("users.list", "empty")
	f.raw(t, "POST", "/__mockmachina/ui/data/cart/c_1/delete", "", htmx)
	rec := f.raw(t, "POST", "/__mockmachina/ui/reset", "", htmx)
	if rec.Code != 204 || rec.Header().Get("HX-Refresh") != "true" {
		t.Errorf("reset = %d, HX-Refresh %q", rec.Code, rec.Header().Get("HX-Refresh"))
	}
	if _, ok := f.mem.Pinned("users.list"); ok {
		t.Error("reset kept the pin")
	}
	if _, got := f.call(t, "GET", "/__mockmachina/collections/cart", "", nil); got["count"] != float64(2) {
		t.Errorf("reset left %v items, want the 2 seeded", got["count"])
	}
}
