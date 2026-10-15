package control_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestUICoverage_Pages(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, "", map[string]string{"routes/users.yaml": uiRoutesYAML, "routes/cart.yaml": cartYAML})
	f.mockStatus(t)
	f.mem.Pin("users.list", "empty")
	f.mockStatus(t)
	for target, golden := range map[string]string{
		"/__mockmachina/ui/coverage":             "ui/coverage.html",
		"/__mockmachina/ui/coverage?filter=gaps": "ui/coverage-gaps.html",
	} {
		rec := f.raw(t, "GET", target, "", nil)
		if rec.Code != 200 {
			t.Errorf("GET %s = %d", target, rec.Code)
		}
		testkit.Golden(t, rec.Body.Bytes(), golden)
	}
}
