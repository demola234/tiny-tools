package cli

import (
	"cmp"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

type infoKey struct{}

func withInfo(ctx context.Context, info buildinfo.Info) context.Context {
	return context.WithValue(ctx, infoKey{}, info)
}

func infoFrom(ctx context.Context) buildinfo.Info {
	info, _ := ctx.Value(infoKey{}).(buildinfo.Info)
	return info
}

func terminal(w io.Writer) (int, bool) {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(f.Fd()) {
		return 0, false
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0, false
	}
	return width, lipgloss.HasDarkBackground(os.Stdin, f)
}

func projectName(dir string) string { return filepath.Base(filepath.Dir(dir)) }

func (o startOptions) card(s *session, info buildinfo.Info, home string) (tui.Welcome, bool) {
	w := o.welcome(s, info, home)
	return w, o.width > 0 && w.Render(o.width, o.dark) != ""
}

const introStep = 150 * time.Millisecond

var introFrames = []time.Duration{
	400 * time.Millisecond, 800 * time.Millisecond, 1200 * time.Millisecond,
	1600 * time.Millisecond, 3 * time.Second, 3200 * time.Millisecond,
}

func playIntro(ctx context.Context, w io.Writer, frame func(time.Duration) string, clk clock.Clock) {
	card := frame(0)
	_, _ = lipgloss.Fprintln(w, card)
	for _, at := range introFrames {
		if clk.Sleep(ctx, introStep) != nil {
			return
		}
		up := strings.Count(card, "\n") + 1
		card = frame(at)
		_, _ = io.WriteString(w, "\x1b["+strconv.Itoa(up)+"A\r")
		_, _ = lipgloss.Fprintln(w, card)
	}
}

func (o startOptions) welcome(s *session, info buildinfo.Info, home string) tui.Welcome {
	dir := o.dir
	if rel, err := filepath.Rel(home, dir); home != "" && err == nil && !strings.HasPrefix(rel, "..") {
		dir = filepath.ToSlash(filepath.Join("~", rel))
	}
	return tui.Welcome{
		Version: info.Version,
		Project: projectName(o.dir),
		URL:     s.url,
		Dir:     dir,
		Routes:  len(s.proj.Routes),
		Tips:    tips(s),
		Notes:   append(append([]string(nil), s.notes...), "Ctrl+C stops the server"),
	}
}

func tips(s *session) []tui.Tip {
	var out []tui.Tip
	for _, r := range s.proj.Routes {
		if r.Method == model.Method("GET") && !strings.Contains(r.Path, "{") {
			out = append(out, tui.Tip{Command: "curl " + s.url + r.Path, What: "call a route"})
			break
		}
	}
	for _, r := range s.proj.Routes {
		if state, ok := otherState(r); ok {
			out = append(out, tui.Tip{Command: "mockmachina state set " + r.ID + " " + state, What: "switch state"})
			break
		}
	}
	return append(out, tui.Tip{Command: "mockmachina start", What: "live screen"})
}

func otherState(r *model.Route) (string, bool) {
	if len(r.States) < 2 {
		return "", false
	}
	active := cmp.Or(r.Active, r.States[0].Name)
	for _, s := range r.States {
		if s.Name != active {
			return s.Name, true
		}
	}
	return "", false
}

func useScreen(plain, inTerminal, outTerminal bool) bool {
	return !plain && inTerminal && outTerminal
}

func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}
