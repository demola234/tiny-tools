package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/demola234/tiny-tools/mock_machina/internal/update"
)

func TestProgressBar(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	b := &progressBar{out: &out, label: "downloading v0.2.0"}
	b.update(0, 4<<20)
	b.update(2<<20, 4<<20)
	b.update(4<<20, 4<<20)
	b.finish("updated mockmachina v0.1.1 → v0.2.0")
	got := ansi.Strip(out.String())
	for _, want := range []string{"downloading v0.2.0", "2.0 / 4.0 MB", "4.0 / 4.0 MB", "✓ updated mockmachina v0.1.1 → v0.2.0\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Count(out.String(), "\r") < 3 {
		t.Errorf("each update should redraw the line in place: %q", out.String())
	}
}

type promptRun struct {
	asked   bool
	applied []string
	out     string
}

func offer(t *testing.T, state, answer string, now time.Time, src update.Source) promptRun {
	t.Helper()
	var r promptRun
	var out strings.Builder
	c := updateCheck{
		current: "v0.1.1", state: state, now: now, in: strings.NewReader(answer), out: &out, src: src,
		apply: func(_ context.Context, latest string) error { r.applied = append(r.applied, latest); return nil },
	}
	c.run(t.Context())
	r.out = out.String()
	r.asked = strings.Contains(r.out, "[Y/n]")
	return r
}

func TestUpdatePrompt(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)
	state := filepath.Join(t.TempDir(), "update.json")
	src := releaseServer(t)
	unreachable := update.Source{Client: src.Client, API: "http://127.0.0.1:1/releases", Download: "http://127.0.0.1:1"}

	r := offer(t, state, "\n", day, src)
	if !r.asked || len(r.applied) != 1 || r.applied[0] != "v0.2.0" || !strings.Contains(r.out, "mockmachina v0.2.0 is out (you have v0.1.1)") {
		t.Fatalf("enter should mean yes: %+v", r)
	}

	r = offer(t, state, "n\n", day.Add(time.Hour), unreachable)
	if !r.asked || len(r.applied) != 0 {
		t.Fatalf("an answer of n should not update, and a check within a day should use the saved result: %+v", r)
	}

	r = offer(t, state, "y\n", day.Add(2*time.Hour), unreachable)
	if r.asked {
		t.Errorf("a version the user said no to shouldn't be offered again: %+v", r)
	}

	fresh := filepath.Join(t.TempDir(), "update.json")
	if r := offer(t, fresh, "y\n", day, unreachable); r.asked || r.out != "" {
		t.Errorf("an unreachable GitHub should stay silent: %+v", r)
	}
}

func TestUpdatePrompt_SkipsUpToDateAndDevBuilds(t *testing.T) {
	t.Parallel()

	src := releaseServer(t)
	for _, current := range []string{"v0.2.0", "(devel)"} {
		var out strings.Builder
		c := updateCheck{
			current: current, state: filepath.Join(t.TempDir(), "u.json"), now: time.Now(), in: strings.NewReader("y\n"), out: &out, src: src,
			apply: func(context.Context, string) error { t.Errorf("%s: applied", current); return nil },
		}
		c.run(t.Context())
		if out.Len() != 0 {
			t.Errorf("%s: printed %q", current, out.String())
		}
	}
}

func TestWantsUpdateCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		env  map[string]string
		want bool
	}{
		{[]string{"start"}, nil, true},
		{nil, nil, true},
		{[]string{"update"}, nil, false},
		{[]string{"mcp"}, nil, false},
		{[]string{"--version"}, nil, false},
		{[]string{"lint", "--help"}, nil, false},
		{[]string{"start"}, map[string]string{"CI": "true"}, false},
		{[]string{"start"}, map[string]string{"MOCKMACHINA_NO_UPDATE_CHECK": "1"}, false},
	}
	for _, tt := range tests {
		if got := wantsUpdateCheck(tt.args, func(k string) string { return tt.env[k] }); got != tt.want {
			t.Errorf("%v %v: %v, want %v", tt.args, tt.env, got, tt.want)
		}
	}
}
