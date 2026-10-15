package control_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestUIOverview_Busy(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, "", map[string]string{
		"routes/users.yaml":       uiRoutesYAML,
		"routes/cart.yaml":        cartYAML,
		"scenarios/new-user.yaml": "title: New user\nstates: { users.list: empty }\n",
	})
	if code, _ := f.call(t, "POST", "/__mockmachina/scenarios/new-user/apply", "", nil); code != 200 {
		t.Fatalf("apply = %d", code)
	}
	f.mem.Pin("users.get", "missing")
	f.mockStatus(t)
	publishTraffic(f)
	rec := f.raw(t, "GET", "/__mockmachina/ui/", "", nil)
	if rec.Code != 200 {
		t.Fatalf("GET ui/ = %d", rec.Code)
	}
	testkit.Golden(t, rec.Body.Bytes(), "ui/overview-busy.html")
}
