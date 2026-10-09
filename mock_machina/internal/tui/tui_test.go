package tui_test

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func route(id, method, path, active string, states ...string) *model.Route {
	r := &model.Route{ID: id, Method: model.Method(method), Path: path, Active: active}
	for _, s := range states {
		r.States = append(r.States, model.NamedState{Name: s, State: &model.State{Status: 200}})
	}
	return r
}

func project(usersActive string) *model.Project {
	return &model.Project{Routes: []*model.Route{
		route("session.create", "POST", "/session", "success", "success", "invalid"),
		route("users.get", "GET", "/users/{id}", "found", "found", "not_found"),
		route("users.list", "GET", "/users", usersActive, "success", "empty", "unauthorized"),
	}}
}

type recorder struct {
	sets    []string
	reloads int
	err     error
}

func (r *recorder) options() tui.Options {
	return tui.Options{
		Header: tui.Header{Project: "shop", URL: "http://127.0.0.1:4001", Seed: 42},
		SetActive: func(route, state string) error {
			r.sets = append(r.sets, route+":"+state)
			return r.err
		},
		Reload: func() { r.reloads++ },
	}
}

func start(t *testing.T, rec *recorder, width, height int) tui.Model {
	t.Helper()
	m := tui.New(project("success"), rec.options())
	return send(t, m, tui.Tick(now), tea.WindowSizeMsg{Width: width, Height: height})
}

func send(t *testing.T, m tui.Model, msgs ...tea.Msg) tui.Model {
	t.Helper()
	for _, msg := range msgs {
		next, cmd := m.Update(msg)
		got, ok := next.(tui.Model)
		if !ok {
			t.Fatalf("Update returned %T, want tui.Model", next)
		}
		m = got
		if _, key := msg.(tea.KeyPressMsg); key && cmd != nil {
			if out := cmd(); out != nil {
				if _, quit := out.(tea.QuitMsg); !quit {
					m = send(t, m, out)
				}
			}
		}
	}
	return m
}

var special = map[string]tea.KeyPressMsg{
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"enter":     {Code: tea.KeyEnter},
	"esc":       {Code: tea.KeyEscape},
	"tab":       {Code: tea.KeyTab},
	"backspace": {Code: tea.KeyBackspace},
}

func keys(names ...string) []tea.Msg {
	msgs := make([]tea.Msg, 0, len(names))
	for _, n := range names {
		if k, ok := special[n]; ok {
			msgs = append(msgs, k)
			continue
		}
		for _, r := range n {
			msgs = append(msgs, tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
	return msgs
}

func frame(m tui.Model) string { return ansi.Strip(m.View().Content) + "\n" }

func at(sec int) time.Time { return time.Date(2026, 10, 9, 12, 1, sec, 0, time.UTC) }

var now = at(10)

func later(d time.Duration) tui.Tick { return tui.Tick(now.Add(d)) }

func requests() []tea.Msg {
	return []tea.Msg{
		tui.Request{Time: at(3), Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "success"},
		tui.Request{Time: at(5), Method: "GET", Path: "/users/u_404", Status: 404, Route: "users.get", State: "not_found", Note: "rule 1"},
		tui.Request{Time: at(9), Method: "POST", Path: "/session", Status: 400, Route: "session.create", State: "invalid", Note: "request doesn't match the schema"},
	}
}

func TestView_FirstFrame(t *testing.T) {
	m := start(t, &recorder{}, 80, 14)
	if !m.View().AltScreen {
		t.Error("the view should use the alternate screen")
	}
	testkit.Golden(t, []byte(frame(m)), "tui/first.txt")
}

func TestView_EveryLineFitsTheWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 14}, {60, 10}, {120, 30}} {
		m := send(t, start(t, &recorder{}, size[0], size[1]), requests()...)
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		if len(lines) != size[1] {
			t.Errorf("%dx%d: %d lines, want %d", size[0], size[1], len(lines), size[1])
		}
		for i, l := range lines {
			if w := len([]rune(l)); w != size[0] {
				t.Errorf("%dx%d: line %d is %d wide: %q", size[0], size[1], i, w, l)
			}
		}
	}
}

func TestView_RequestsNewestLast(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), requests()...)
	testkit.Golden(t, []byte(frame(m)), "tui/requests.txt")
}

func TestView_KeepsTheLatestRequestsInView(t *testing.T) {
	m := start(t, &recorder{}, 80, 10)
	for i := range 20 {
		m = send(t, m, tui.Request{Time: at(i), Method: "GET", Path: fmt.Sprintf("/users/%d", i), Status: 200, Route: "users.get", State: "found"})
	}
	view := frame(m)
	if !strings.Contains(view, "/users/19") || strings.Contains(view, "/users/0 ") {
		t.Errorf("want the newest requests in view:\n%s", view)
	}
}

func TestKeys_MoveIntoStatesAndSetOne(t *testing.T) {
	rec := &recorder{}
	m := send(t, start(t, rec, 80, 14), keys("down", "down", "down", "down", "down", "down", "down", "down")...)
	testkit.Golden(t, []byte(frame(m)), "tui/states.txt")

	m = send(t, m, keys("enter")...)
	if diff := cmp.Diff([]string{"users.list:empty"}, rec.sets); diff != "" {
		t.Errorf("SetActive calls (-want +got):\n%s", diff)
	}
	if !strings.Contains(frame(m), "users.list now serves empty") {
		t.Errorf("want a note about the switch:\n%s", frame(m))
	}

	m = send(t, m, tui.Reloaded{Project: project("empty")})
	testkit.Golden(t, []byte(frame(m)), "tui/switched.txt")
}

func TestKeys_EnterOnARouteSetsNothing(t *testing.T) {
	rec := &recorder{}
	send(t, start(t, rec, 80, 14), keys("enter")...)
	if len(rec.sets) != 0 {
		t.Errorf("SetActive called with %v", rec.sets)
	}
}

func TestKeys_SetActiveErrorIsShown(t *testing.T) {
	rec := &recorder{err: errors.New("routes/users.yaml has problems; fix them first")}
	m := send(t, start(t, rec, 80, 14), keys("down", "enter")...)
	if !strings.Contains(frame(m), "routes/users.yaml has problems") {
		t.Errorf("want the error in the status line:\n%s", frame(m))
	}
}

func TestKeys_UpFromARouteLandsOnThePreviousRoutesLastState(t *testing.T) {
	rec := &recorder{}
	send(t, start(t, rec, 80, 14), keys("down", "down", "down", "up", "enter")...)
	if diff := cmp.Diff([]string{"session.create:invalid"}, rec.sets); diff != "" {
		t.Errorf("SetActive calls (-want +got):\n%s", diff)
	}
}

func TestKeys_CursorStopsAtTheEnds(t *testing.T) {
	rec := &recorder{}
	m := send(t, start(t, rec, 80, 14), keys("up", "up")...)
	m = send(t, m, keys(strings.Repeat("j", 30))...)
	send(t, m, keys("enter")...)
	if diff := cmp.Diff([]string{"users.list:unauthorized"}, rec.sets); diff != "" {
		t.Errorf("SetActive calls (-want +got):\n%s", diff)
	}
}

func TestKeys_Filter(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), keys("/", "users", "backspace", "s.l")...)
	testkit.Golden(t, []byte(frame(m)), "tui/filter.txt")

	rec := &recorder{}
	m = send(t, start(t, rec, 80, 14), keys("/", "list", "enter", "down", "down", "enter")...)
	if diff := cmp.Diff([]string{"users.list:empty"}, rec.sets); diff != "" {
		t.Errorf("after filtering, SetActive calls (-want +got):\n%s", diff)
	}

	m = send(t, m, keys("/", "esc")...)
	if !strings.Contains(frame(m), "session.create") {
		t.Errorf("esc should clear the filter:\n%s", frame(m))
	}
}

func TestKeys_FilterMatchingNothing(t *testing.T) {
	rec := &recorder{}
	m := send(t, start(t, rec, 80, 14), keys("/", "zzz", "enter", "down", "enter")...)
	if len(rec.sets) != 0 {
		t.Errorf("SetActive called with %v", rec.sets)
	}
	if !strings.Contains(frame(m), "no routes match") {
		t.Errorf("want an empty-filter hint:\n%s", frame(m))
	}
}

func TestKeys_SelectARequest(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), requests()...)
	m = send(t, m, keys("tab", "up")...)
	testkit.Golden(t, []byte(frame(m)), "tui/request-selected.txt")
}

func TestKeys_ReloadAndQuit(t *testing.T) {
	rec := &recorder{}
	m := send(t, start(t, rec, 80, 14), keys("r")...)
	if rec.reloads != 1 {
		t.Errorf("Reload called %d times, want 1", rec.reloads)
	}
	for _, k := range []tea.Msg{keys("q")[0], tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}} {
		_, cmd := m.Update(k)
		if cmd == nil {
			t.Fatalf("%v: no command, want quit", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v: command gave %T, want tea.QuitMsg", k, cmd())
		}
	}
}

func TestProblems_BarAndList(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), tui.Problems{Lines: []string{
		"routes/users.yaml:4: unknown field \"stauts\" (did you mean \"status\"?)",
		"routes/users.yaml:9: state \"empty\" has no status",
	}})
	testkit.Golden(t, []byte(frame(m)), "tui/problems-bar.txt")

	m = send(t, m, keys("l")...)
	testkit.Golden(t, []byte(frame(m)), "tui/problems-list.txt")

	m = send(t, m, keys("l")...)
	m = send(t, m, tui.Problems{})
	if strings.Contains(frame(m), "problem") {
		t.Errorf("empty Problems should clear the bar:\n%s", frame(m))
	}
}

func TestReloaded_KeepsTheCursorOnTheSameRoute(t *testing.T) {
	rec := &recorder{}
	m := send(t, start(t, rec, 80, 14), keys("down", "down", "down")...)
	smaller := &model.Project{Routes: project("success").Routes[1:]}
	m = send(t, m, tui.Reloaded{Project: smaller})
	send(t, m, keys("down", "enter")...)
	if diff := cmp.Diff([]string{"users.get:found"}, rec.sets); diff != "" {
		t.Errorf("SetActive calls (-want +got):\n%s", diff)
	}
}

func TestNote_ShowsInTheStatusLine(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), tui.Note("reloaded after changes to routes/users.yaml (3 routes)"))
	if !strings.Contains(frame(m), "reloaded after changes to routes/users.yaml") {
		t.Errorf("want the note:\n%s", frame(m))
	}
}

func TestView_StyledFrame(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), requests()...)
	testkit.Golden(t, []byte(m.View().Content+"\n"), "tui/styled.ansi")
}

func TestView_LightTerminalGetsTheLightPalette(t *testing.T) {
	dark := start(t, &recorder{}, 80, 14)
	light := send(t, dark, tea.BackgroundColorMsg{Color: color.White})
	if dark.View().Content == light.View().Content {
		t.Error("a light background should change the colors")
	}
	if frame(dark) != frame(light) {
		t.Error("a light background should only change colors, not the layout")
	}
}

func lineWith(t *testing.T, m tui.Model, text string) string {
	t.Helper()
	for l := range strings.SplitSeq(m.View().Content, "\n") {
		for c := range strings.SplitSeq(l, "│") {
			if strings.Contains(ansi.Strip(c), text) {
				return c
			}
		}
	}
	t.Fatalf("no line contains %q:\n%s", text, frame(m))
	return ""
}

const background = "\x1b[48;"

func TestAnimation_NewRequestFlashesThenSettles(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), requests()[0])
	if !strings.Contains(lineWith(t, m, "/users"), background) {
		t.Error("a new request should be highlighted")
	}
	mid := lineWith(t, send(t, m, later(450*time.Millisecond)), "/users")
	if !strings.Contains(mid, background) || mid == lineWith(t, m, "/users") {
		t.Error("halfway through, the highlight should be fading")
	}
	if strings.Contains(lineWith(t, send(t, m, later(time.Second)), "/users"), background) {
		t.Error("after a second, the highlight should be gone")
	}
}

func TestAnimation_SpinnerWhileSettingThenToast(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), keys("down")...)
	next, cmd := m.Update(keys("enter")[0])
	m = next.(tui.Model)
	if cmd == nil {
		t.Fatal("enter on a state should return a command")
	}
	first := ansi.Strip(lineWith(t, m, "success"))
	if !strings.ContainsAny(first, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
		t.Errorf("want a spinner next to the state being set: %q", first)
	}
	spun := ansi.Strip(lineWith(t, send(t, m, later(80*time.Millisecond)), "success"))
	if spun == first {
		t.Errorf("the spinner should move between frames: %q", spun)
	}

	m = send(t, m, cmd())
	if got := ansi.Strip(lineWith(t, m, "now serves")); !strings.Contains(got, "✓ session.create now serves success") {
		t.Errorf("want a success toast, got %q", got)
	}
	if strings.ContainsAny(frame(m), "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
		t.Error("the spinner should stop when the switch is done")
	}
	if strings.Contains(frame(send(t, m, later(5*time.Second))), "now serves") {
		t.Error("the toast should go away after a few seconds")
	}
}

func TestAnimation_NewActiveStatePops(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), keys("down", "down", "down", "down", "down", "down", "down")...)
	m = send(t, m, tui.Reloaded{Project: project("empty")})
	if got := ansi.Strip(lineWith(t, m, "empty")); !strings.Contains(got, "◎ empty") {
		t.Errorf("the new active state should start its pop: %q", got)
	}
	if got := ansi.Strip(lineWith(t, send(t, m, later(time.Second)), "empty")); !strings.Contains(got, "● empty") {
		t.Errorf("after the pop, the marker should settle: %q", got)
	}
}

func TestAnimation_FilterCursorBlinks(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), keys("/", "us")...)
	on, off := frame(m), frame(send(t, m, later(530*time.Millisecond)))
	if strings.Count(on, "▏") == strings.Count(off, "▏") {
		t.Errorf("the filter cursor should blink:\n%s\n%s", on, off)
	}
}

func TestAnimation_WaitingDots(t *testing.T) {
	a := lineWith(t, start(t, &recorder{}, 80, 14), "waiting for requests")
	b := lineWith(t, send(t, start(t, &recorder{}, 80, 14), later(400*time.Millisecond)), "waiting for requests")
	if a == b {
		t.Error("the waiting dots should move")
	}
}

func TestAnimation_RateSparkline(t *testing.T) {
	m := send(t, start(t, &recorder{}, 80, 14), requests()...)
	if got := ansi.Strip(strings.SplitN(m.View().Content, "\n", 2)[0]); !strings.Contains(got, "3 req · 20s") {
		t.Errorf("want the request rate in the header: %q", got)
	}
	m = send(t, m, later(time.Minute))
	if got := ansi.Strip(strings.SplitN(m.View().Content, "\n", 2)[0]); !strings.Contains(got, "▁▁▁▁▁▁▁▁▁▁ 0 req · 20s") {
		t.Errorf("old requests should drop out of the rate: %q", got)
	}
}

func TestTicks_FastWhileAnimatingIdleOtherwise(t *testing.T) {
	m := tui.New(project("success"), (&recorder{}).options())
	if m.Init() == nil {
		t.Fatal("Init should start the clock")
	}
	idle := tickEvery(t, m)
	busy := tickEvery(t, send(t, m, tui.Tick(now), requests()[0]))
	if busy >= idle {
		t.Errorf("animating tick %v should be faster than idle tick %v", busy, idle)
	}
}

func tickEvery(t *testing.T, m tui.Model) time.Duration {
	t.Helper()
	_, cmd := m.Update(tui.Tick(now))
	if cmd == nil {
		t.Fatal("a tick should schedule the next one")
	}
	begin := time.Now()
	if _, ok := cmd().(tui.Tick); !ok {
		t.Fatal("the next tick should be a tui.Tick")
	}
	return time.Since(begin)
}

func TestAnimation_MascotWaitsForRequests(t *testing.T) {
	m := start(t, &recorder{}, 80, 14)
	if !strings.Contains(frame(m), "▄███▄") {
		t.Fatalf("want the mascot in the empty requests pane:\n%s", frame(m))
	}
	if frame(m) == frame(send(t, m, later(400*time.Millisecond))) {
		t.Error("the mascot should move")
	}
	if strings.Contains(frame(send(t, m, requests()[0])), "▄███▄") {
		t.Error("the mascot should make way for requests")
	}
}
