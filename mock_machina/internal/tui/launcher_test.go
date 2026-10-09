package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func actions() []tui.Action {
	return []tui.Action{
		{Name: "start", Short: "Serve the mock API", Args: []string{"start"}, Fields: []tui.Field{
			{Label: "Folder", Flag: "--dir", Hint: "nearest .mockmachina"},
			{Label: "Port", Flag: "--port", Default: "4001"},
			{Label: "HTTPS", Flag: "--https", Toggle: true},
		}},
		{Name: "state list", Short: "Show routes and their states", Args: []string{"state", "list"}},
		{Name: "state set", Short: "Switch the state a route serves", Args: []string{"state", "set"}, Fields: []tui.Field{
			{Label: "Route and state", Required: true, Split: true, Choices: []string{"users.list empty", "users.list unauthorized", "users.get not_found"}},
		}},
		{Name: "add", Short: "Add a route", Args: []string{"add"}, Fields: []tui.Field{
			{Label: "Method", Choices: []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
			{Label: "Path", Required: true, Hint: "/users/{id}"},
			{Label: "Summary", Flag: "--summary"},
		}},
		{Name: "lint", Short: "Check .mockmachina/ for mistakes", Args: []string{"lint"}},
	}
}

type launch struct {
	t *testing.T
	m tui.Launcher
}

func newLaunch(t *testing.T) *launch {
	t.Helper()
	l := &launch{t: t, m: tui.NewLauncher("v0.7.0", actions())}
	l.send(tui.Tick(now), tea.WindowSizeMsg{Width: 80, Height: 22})
	return l
}

func (l *launch) send(msgs ...tea.Msg) tea.Cmd {
	l.t.Helper()
	var last tea.Cmd
	for _, msg := range msgs {
		next, cmd := l.m.Update(msg)
		m, ok := next.(tui.Launcher)
		if !ok {
			l.t.Fatalf("Update returned %T", next)
		}
		l.m, last = m, cmd
	}
	return last
}

func (l *launch) keys(names ...string) tea.Cmd {
	l.t.Helper()
	return l.send(keys(names...)...)
}

func (l *launch) frame() string { return ansi.Strip(l.m.View().Content) + "\n" }

func quits(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

var (
	right = tea.KeyPressMsg{Code: tea.KeyRight}
	space = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	ctrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
)

func TestLauncher_Menu(t *testing.T) {
	l := newLaunch(t)
	if !l.m.View().AltScreen {
		t.Error("the launcher should use the alternate screen")
	}
	testkit.Golden(t, []byte(l.frame()), "tui/launcher-menu.txt")
}

func TestLauncher_LinesFitTheWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 22}, {60, 16}, {120, 40}} {
		l := newLaunch(t)
		l.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, step := range [][]string{nil, {"enter"}} {
			l.keys(step...)
			lines := strings.Split(strings.TrimSuffix(l.frame(), "\n"), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d after %v: %d lines", size[0], size[1], step, len(lines))
			}
			for i, line := range lines {
				if w := ansi.StringWidth(line); w != size[0] {
					t.Errorf("%dx%d after %v: line %d is %d wide: %q", size[0], size[1], step, i, w, line)
				}
			}
		}
	}
}

func TestLauncher_TypingFilters(t *testing.T) {
	l := newLaunch(t)
	l.keys("state")
	testkit.Golden(t, []byte(l.frame()), "tui/launcher-filter.txt")

	l.keys("down", "enter")
	if !strings.Contains(l.frame(), "Route and state") {
		t.Errorf("down then enter should open state set:\n%s", l.frame())
	}
}

func TestLauncher_FilterMatchingNothing(t *testing.T) {
	l := newLaunch(t)
	if l.keys("zzz", "enter"); l.m.Chosen() != nil {
		t.Errorf("Chosen = %v, want nothing", l.m.Chosen())
	}
	if !strings.Contains(l.frame(), "no command matches") {
		t.Errorf("want an empty-filter hint:\n%s", l.frame())
	}
}

func TestLauncher_ActionWithoutFieldsRunsAtOnce(t *testing.T) {
	l := newLaunch(t)
	if !quits(t, l.keys("lint", "enter")) {
		t.Fatal("enter on lint should quit the launcher")
	}
	if diff := cmp.Diff([]string{"lint"}, l.m.Chosen()); diff != "" {
		t.Errorf("Chosen (-want +got):\n%s", diff)
	}
}

func TestLauncher_FormBuildsTheCommand(t *testing.T) {
	l := newLaunch(t)
	l.keys("enter")
	testkit.Golden(t, []byte(l.frame()), "tui/launcher-form.txt")

	l.keys("down", "backspace", "backspace", "backspace", "backspace", "5000", "down")
	l.send(space)
	if !strings.Contains(l.frame(), "$ mockmachina start --port 5000 --https") {
		t.Errorf("want the preview to follow the form:\n%s", l.frame())
	}
	if !quits(t, l.keys("enter")) {
		t.Fatal("enter in the form should quit the launcher")
	}
	if diff := cmp.Diff([]string{"start", "--port", "5000", "--https"}, l.m.Chosen()); diff != "" {
		t.Errorf("Chosen (-want +got):\n%s", diff)
	}
}

func TestLauncher_DefaultsAreLeftOut(t *testing.T) {
	l := newLaunch(t)
	l.keys("enter", "enter")
	if diff := cmp.Diff([]string{"start"}, l.m.Chosen()); diff != "" {
		t.Errorf("Chosen (-want +got):\n%s", diff)
	}
}

func TestLauncher_ChoicesCycle(t *testing.T) {
	l := newLaunch(t)
	l.keys("state set", "enter")
	l.send(right)
	l.keys("enter")
	if diff := cmp.Diff([]string{"state", "set", "users.list", "unauthorized"}, l.m.Chosen()); diff != "" {
		t.Errorf("Chosen (-want +got):\n%s", diff)
	}
}

func TestLauncher_RequiredFieldStopsTheRun(t *testing.T) {
	l := newLaunch(t)
	cmd := l.keys("add", "enter", "enter")
	if quits(t, cmd) || l.m.Chosen() != nil {
		t.Fatalf("an empty Path shouldn't run; Chosen = %v", l.m.Chosen())
	}
	if !strings.Contains(l.frame(), "Path is needed") {
		t.Errorf("want the missing field named:\n%s", l.frame())
	}
	l.keys("down", "/orders", "enter")
	if diff := cmp.Diff([]string{"add", "GET", "/orders"}, l.m.Chosen()); diff != "" {
		t.Errorf("Chosen (-want +got):\n%s", diff)
	}
}

func TestLauncher_PreviewQuotesSpaces(t *testing.T) {
	l := newLaunch(t)
	l.keys("add", "enter", "down", "/orders", "down", "List orders")
	if !strings.Contains(l.frame(), `$ mockmachina add GET /orders --summary "List orders"`) {
		t.Errorf("want a quoted summary in the preview:\n%s", l.frame())
	}
}

func TestLauncher_EscGoesBackThenQuits(t *testing.T) {
	l := newLaunch(t)
	l.keys("enter", "esc")
	if !strings.Contains(l.frame(), "state list") {
		t.Errorf("esc in the form should go back to the menu:\n%s", l.frame())
	}
	l.keys("li")
	if quits(t, l.keys("esc")) {
		t.Fatal("the first esc should only clear the filter")
	}
	if !quits(t, l.keys("esc")) || l.m.Chosen() != nil {
		t.Errorf("esc on an empty filter should quit with nothing chosen; Chosen = %v", l.m.Chosen())
	}
}

func TestLauncher_CtrlCQuitsAnywhere(t *testing.T) {
	for _, before := range [][]string{nil, {"enter"}} {
		l := newLaunch(t)
		l.keys(before...)
		if !quits(t, l.send(ctrlC)) || l.m.Chosen() != nil {
			t.Errorf("after %v: ctrl+c should quit with nothing chosen", before)
		}
	}
}

func TestLauncher_TextCursorBlinks(t *testing.T) {
	l := newLaunch(t)
	l.keys("enter")
	on := l.frame()
	l.send(later(530 * time.Millisecond))
	if strings.Count(on, "▏") == strings.Count(l.frame(), "▏") {
		t.Error("the text cursor should blink")
	}
}

func TestLauncher_MascotMoves(t *testing.T) {
	l := newLaunch(t)
	if !strings.Contains(l.frame(), "▄███▄") {
		t.Fatalf("want the mascot in the menu:\n%s", l.frame())
	}
	before := l.frame()
	l.send(later(400 * time.Millisecond))
	if before == l.frame() {
		t.Error("the mascot should move")
	}
}

func TestLauncher_EmptyOptionalPositionalIsLeftOut(t *testing.T) {
	l := tui.NewLauncher("", []tui.Action{{Name: "diff", Args: []string{"diff"}, Fields: []tui.Field{{Label: "Base", Hint: "main"}}}})
	next, _ := l.Update(keys("enter")[0])
	next, _ = next.Update(keys("enter")[0])
	if diff := cmp.Diff([]string{"diff"}, next.(tui.Launcher).Chosen()); diff != "" {
		t.Errorf("Chosen (-want +got):\n%s", diff)
	}
}
