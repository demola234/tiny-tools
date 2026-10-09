package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func welcomeSession() *session {
	states := func(names ...string) model.States {
		s := make(model.States, len(names))
		for i, n := range names {
			s[i] = model.NamedState{Name: n, State: &model.State{Status: 200}}
		}
		return s
	}
	return &session{
		proj: &model.Project{Routes: []*model.Route{
			{ID: "session.create", Method: "POST", Path: "/session", States: states("success")},
			{ID: "users.get", Method: "GET", Path: "/users/{id}", States: states("found")},
			{ID: "users.list", Method: "GET", Path: "/users", States: states("success", "empty")},
		}},
		url:    "http://127.0.0.1:4001",
		banner: "serving 3 routes from /home/ada/shop/.mockmachina on http://127.0.0.1:4001 (Ctrl+C to stop)",
		notes:  []string{"seed 42 (start with --seed 42 to get the same responses again)"},
	}
}

func TestWelcomeCard(t *testing.T) {
	o := startOptions{dir: filepath.FromSlash("/home/ada/shop/.mockmachina")}
	got := o.welcome(welcomeSession(), buildinfo.Info{Version: "v0.7.0"}, filepath.FromSlash("/home/ada"))
	want := tui.Welcome{
		Version: "v0.7.0",
		Project: "shop",
		URL:     "http://127.0.0.1:4001",
		Dir:     "~/shop/.mockmachina",
		Routes:  3,
		Tips: []tui.Tip{
			{Command: "curl http://127.0.0.1:4001/users", What: "call a route"},
			{Command: "mockmachina state set users.list empty", What: "switch state"},
			{Command: "mockmachina start", What: "live screen"},
		},
		Notes: []string{"seed 42 (start with --seed 42 to get the same responses again)", "Ctrl+C stops the server"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestWelcomeCard_NothingToTry(t *testing.T) {
	s := welcomeSession()
	s.proj.Routes = s.proj.Routes[:1]
	got := startOptions{dir: "/elsewhere/.mockmachina"}.welcome(s, buildinfo.Info{}, "/home/ada")
	if diff := cmp.Diff([]tui.Tip{{Command: "mockmachina start", What: "live screen"}}, got.Tips); diff != "" {
		t.Errorf("tips (-want +got):\n%s", diff)
	}
	if got.Dir != "/elsewhere/.mockmachina" {
		t.Errorf("Dir = %q, want the path unchanged outside home", got.Dir)
	}
}

func TestCard_OnAWideTerminal(t *testing.T) {
	o := startOptions{dir: "/home/ada/shop/.mockmachina", width: 100, dark: true}
	if _, ok := o.card(welcomeSession(), buildinfo.Info{Version: "v0.7.0"}, "/home/ada"); !ok {
		t.Error("want a card at 100 columns")
	}
	for _, width := range []int{0, 40} {
		o.width = width
		if _, ok := o.card(welcomeSession(), buildinfo.Info{}, "/home/ada"); ok {
			t.Errorf("width %d: want plain lines, not a card", width)
		}
	}
}

type fakeClock struct{ slept []time.Duration }

func (*fakeClock) Now() time.Time { return time.Time{} }

func (f *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	f.slept = append(f.slept, d)
	return nil
}

func TestPlayIntro_RedrawsInPlaceThenSettles(t *testing.T) {
	var out strings.Builder
	var at []time.Duration
	clk := &fakeClock{}
	playIntro(t.Context(), &out, func(d time.Duration) string {
		at = append(at, d)
		return "line 1\nline 2 " + d.String()
	}, clk)
	if len(at) < 4 {
		t.Fatalf("want several frames, got %v", at)
	}
	if at[0] != 0 {
		t.Errorf("first frame at %v, want 0", at[0])
	}
	if got, want := strings.Count(out.String(), "\x1b[2A\r"), len(at)-1; got != want {
		t.Errorf("%d cursor moves, want %d (one per redraw)", got, want)
	}
	if len(clk.slept) != len(at)-1 {
		t.Errorf("slept %d times for %d frames", len(clk.slept), len(at))
	}
	if !strings.HasSuffix(out.String(), "line 2 "+at[len(at)-1].String()+"\n") {
		t.Errorf("output should end on the last frame: %q", out.String())
	}
}

func TestPlayIntro_StopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var out strings.Builder
	frames := 0
	playIntro(ctx, &out, func(time.Duration) string { frames++; return "card" }, clock.Real{})
	if frames != 1 {
		t.Errorf("drew %d frames after cancel, want just the first", frames)
	}
}

func TestUseScreen(t *testing.T) {
	tests := []struct {
		plain, in, out, want bool
	}{
		{false, true, true, true},
		{true, true, true, false},
		{false, false, true, false},
		{false, true, false, false},
		{false, false, false, false},
	}
	for _, tt := range tests {
		if got := useScreen(tt.plain, tt.in, tt.out); got != tt.want {
			t.Errorf("useScreen(plain=%v, in=%v, out=%v) = %v, want %v", tt.plain, tt.in, tt.out, got, tt.want)
		}
	}
}
