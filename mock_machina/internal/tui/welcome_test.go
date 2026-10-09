package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func welcome() tui.Welcome {
	return tui.Welcome{
		Version: "v0.7.0",
		Project: "shop",
		URL:     "http://127.0.0.1:4001",
		Dir:     "~/code/shop/.mockmachina",
		Routes:  3,
		Tips: []tui.Tip{
			{Command: "curl http://127.0.0.1:4001/users", What: "call a route"},
			{Command: "mockmachina state set users.list empty", What: "serve another state"},
			{Command: "mockmachina tui", What: "live routes and requests"},
		},
		Notes: []string{"seed 42 (start with --seed 42 to get the same responses again)"},
	}
}

func TestWelcome_Card(t *testing.T) {
	testkit.Golden(t, []byte(ansi.Strip(welcome().Render(100, true))+"\n"), "tui/welcome.txt")
}

func TestWelcome_FitsTheWidth(t *testing.T) {
	for _, tt := range []struct{ width, want int }{{64, 64}, {80, 80}, {100, 100}, {160, 100}} {
		card := welcome().Render(tt.width, true)
		lines := strings.Split(ansi.Strip(card), "\n")
		if len(lines) < 8 {
			t.Errorf("width %d: only %d lines", tt.width, len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w != tt.want {
				t.Errorf("width %d: line %d is %d wide, want %d: %q", tt.width, i, w, tt.want, l)
			}
		}
	}
}

func TestWelcome_TooNarrowGivesNothing(t *testing.T) {
	if got := welcome().Render(50, true); got != "" {
		t.Errorf("want no card on a narrow terminal, got:\n%s", got)
	}
}

func TestWelcome_NoNotesSaysFilesReload(t *testing.T) {
	w := welcome()
	w.Notes = nil
	if got := ansi.Strip(w.Render(100, true)); !strings.Contains(got, "Saved changes in .mockmachina/ reload") {
		t.Errorf("want the reload hint when there are no notes:\n%s", got)
	}
}

func TestWelcome_LightPaletteOnlyChangesColors(t *testing.T) {
	dark, light := welcome().Render(100, true), welcome().Render(100, false)
	if dark == light {
		t.Error("light and dark cards should differ in color")
	}
	if ansi.Strip(dark) != ansi.Strip(light) {
		t.Error("light and dark cards should have the same text")
	}
}

func TestWelcome_MascotMoves(t *testing.T) {
	w := welcome()
	still := ansi.Strip(w.RenderAt(100, true, 0))
	if still != ansi.Strip(w.Render(100, true)) {
		t.Error("Render should be the first frame")
	}
	seen := map[string]bool{still: true}
	for _, at := range []time.Duration{400 * time.Millisecond, 3 * time.Second} {
		frame := ansi.Strip(w.RenderAt(100, true, at))
		if len(strings.Split(frame, "\n")) != len(strings.Split(still, "\n")) {
			t.Fatalf("at %v the card changed height", at)
		}
		seen[frame] = true
	}
	if len(seen) != 3 {
		t.Errorf("want feet and eyes to move: %d distinct frames, want 3", len(seen))
	}
	if !strings.Contains(ansi.Strip(w.RenderAt(100, true, 3*time.Second)), "█▄███▄█") {
		t.Error("at 3s the mascot should blink")
	}
}
