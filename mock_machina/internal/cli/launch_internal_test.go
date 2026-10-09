package cli

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func names(actions []tui.Action) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = a.Name
	}
	return out
}

func find(t *testing.T, actions []tui.Action, name string) tui.Action {
	t.Helper()
	i := slices.IndexFunc(actions, func(a tui.Action) bool { return a.Name == name })
	if i < 0 {
		t.Fatalf("no action %q in %v", name, names(actions))
	}
	return actions[i]
}

func TestCatalog_WithAProject(t *testing.T) {
	proj := welcomeSession().proj
	proj.Routes[2].Active = "success"
	proj.Config.Ports.Mock = 4100
	actions := catalog(proj)

	want := []string{
		"start", "start --plain", "tui", "state set", "state list", "add", "lint", "diff", "diff --live",
		"import", "export", "docs", "docs -o", "mcp", "cert", "cert --install", "init",
	}
	if diff := cmp.Diff(want, names(actions)); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
	if got := find(t, actions, "start").Fields[0]; got.Flag != "--port" || got.Default != "4100" {
		t.Errorf("start's port field = %+v, want --port defaulting to config.yaml's 4100", got)
	}
	set := find(t, actions, "state set").Fields[0]
	if diff := cmp.Diff([]string{"users.list empty"}, set.Choices); diff != "" {
		t.Errorf("state set choices (-want +got):\n%s", diff)
	}
	if !set.Required || !set.Split {
		t.Errorf("state set's field should be required and split: %+v", set)
	}
	for _, a := range actions {
		if a.Short == "" || len(a.Args) == 0 {
			t.Errorf("%s needs a description and arguments: %+v", a.Name, a)
		}
	}
}

func TestCatalog_WithoutAProjectStartsWithInit(t *testing.T) {
	actions := catalog(nil)
	if actions[0].Name != "init" {
		t.Errorf("first action = %s, want init", actions[0].Name)
	}
	if set := find(t, actions, "state set").Fields[0]; set.Choices != nil || set.Hint == "" {
		t.Errorf("without a project, state set should explain why there's nothing to pick: %+v", set)
	}
}

func TestCatalog_CoversEveryCommand(t *testing.T) {
	covered := map[string]bool{}
	for _, a := range catalog(&model.Project{}) {
		covered[a.Args[0]] = true
	}
	for _, c := range NewRootCmd().Commands() {
		if c.IsAvailableCommand() && !covered[c.Name()] {
			t.Errorf("the launcher has no entry for %q", c.Name())
		}
	}
}
