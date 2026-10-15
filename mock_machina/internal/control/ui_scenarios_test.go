package control_test

import (
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func scenarioFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWith(t, "", map[string]string{
		"routes/users.yaml":        uiRoutesYAML,
		"scenarios/new-user.yaml":  "title: New user\ndescription: Just signed up, nothing yet.\nstates:\n  users.list: empty\n",
		"scenarios/lost-user.yaml": "states:\n  users.get: missing\n  users.list: full\n",
	})
}

func TestUIScenarios_Page(t *testing.T) {
	t.Parallel()

	f := scenarioFixture(t)
	rec := f.raw(t, "POST", "/__mockmachina/ui/scenarios/new-user/apply", "", htmx)
	if rec.Code != 200 || f.mem.Applied() != "new-user" {
		t.Fatalf("apply = %d, applied %q: %s", rec.Code, f.mem.Applied(), rec.Body)
	}
	if got, _ := f.mem.Pinned("users.list"); got != "empty" {
		t.Errorf("users.list pinned to %q, want empty", got)
	}
	testkit.Golden(t, rec.Body.Bytes(), "ui/scenarios.html")

	rec = f.raw(t, "POST", "/__mockmachina/ui/scenarios/new-usr/apply", "", htmx)
	if !strings.Contains(rec.Body.String(), "no scenario &#34;new-usr&#34; (did you mean &#34;new-user&#34;?)") {
		t.Errorf("unknown scenario didn't explain itself:\n%s", rec.Body)
	}
}

func TestUIScenarios_None(t *testing.T) {
	t.Parallel()

	rec := newFixture(t, "").raw(t, "GET", "/__mockmachina/ui/scenarios", "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "No scenarios yet") {
		t.Errorf("no scenarios = %d", rec.Code)
	}
}
