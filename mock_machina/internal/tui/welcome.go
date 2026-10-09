package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

const (
	cardMin      = 60
	cardMax      = 100
	cardLeftMax  = 36
	cardGutters  = 7
	reloadHint   = "Saved changes in .mockmachina/ reload as you go"
	welcomeTitle = "Welcome to MockMachina"
)

const (
	stepEvery  = 400 * time.Millisecond
	blinkCycle = 3 * time.Second
	blinkFor   = 150 * time.Millisecond
	mascotWide = 9
)

var (
	feet = []string{"  ▀▀ ▀▀  ", " ▀▀   ▀▀ "}
	eyes = []string{" █ ███ █ ", " █▄███▄█ "}
)

func mascotRows(at time.Duration) []string {
	blinking := 0
	if at > 0 && at%blinkCycle < blinkFor {
		blinking = 1
	}
	return []string{"    ▄    ", "  ▄███▄  ", eyes[blinking], " ███████ ", feet[phase(at, 2*stepEvery, len(feet))]}
}

func (th theme) mascot(at time.Duration) []line {
	rows := mascotRows(at)
	colors := lipgloss.Blend1D(len(rows), th.accent, th.accent2)
	glow := lipgloss.Blend1D(blendSteps, th.accent2, th.faint)
	i := phase(at, breathCycle, 2*blendSteps)
	if i >= blendSteps {
		i = 2*blendSteps - 1 - i
	}
	colors[0] = glow[i]
	out := make([]line, len(rows))
	for r, row := range rows {
		out[r] = text(row, fg(colors[r]))
	}
	return out
}

type Tip struct{ Command, What string }

type Welcome struct {
	Version string
	Project string
	URL     string
	Dir     string
	Routes  int
	Tips    []Tip
	Notes   []string
}

func (w Welcome) Render(width int, dark bool) string { return w.RenderAt(width, dark, 0) }

func (w Welcome) RenderAt(width int, dark bool, at time.Duration) string {
	if width < cardMin {
		return ""
	}
	width = min(width, cardMax)
	th := newTheme(dark)
	left := min(cardLeftMax, (width-cardGutters)*2/5)
	right := width - cardGutters - left
	lrows, rrows := w.leftRows(th, at), w.rightRows(th, right)
	height := max(len(lrows), len(rrows)) + 1

	border := fg(th.accent)
	out := make([]string, 0, height+2)
	out = append(out, w.top(th, width))
	for i := range height {
		l, r := line{}, line{}
		if i < len(lrows) {
			l = center(lrows[i], left)
		}
		if i < len(rrows) {
			r = rrows[i]
		}
		out = append(out, border.Render("│ ")+l.render(left)+fg(th.faint).Render(" │ ")+r.render(right)+border.Render(" │"))
	}
	out = append(out, border.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(out, "\n")
}

func (w Welcome) top(th theme, width int) string {
	var t line
	t.spans = gradient("MockMachina", th.accent, th.accent2)
	if w.Version != "" {
		t.add(" "+w.Version, fg(th.dim))
	}
	tw := min(t.width(), width-6)
	border := fg(th.accent)
	return border.Render("╭─ ") + t.render(tw) + border.Render(" "+strings.Repeat("─", width-5-tw)+"╮")
}

func center(l line, width int) line {
	if pad := (width - l.width()) / 2; pad > 0 {
		l.spans = append([]span{{text: strings.Repeat(" ", pad)}}, l.spans...)
	}
	return l
}

func text(s string, st lipgloss.Style) line {
	var l line
	l.add(s, st)
	return l
}

func (w Welcome) leftRows(th theme, at time.Duration) []line {
	greeting := welcomeTitle
	if w.Project != "" {
		greeting = "Serving " + w.Project
	}
	rows := []line{{}, text(greeting, fg(th.fg).Bold(true)), {}}
	rows = append(rows, th.mascot(at)...)
	rows = append(rows, line{}, text(routeCount(w.Routes), fg(th.dim)))
	if w.URL != "" {
		rows = append(rows, text(w.URL, fg(th.info).Underline(true)))
	}
	if w.Dir != "" {
		rows = append(rows, text(w.Dir, fg(th.dim)))
	}
	return rows
}

func routeCount(n int) string {
	if n == 1 {
		return "1 route"
	}
	return strconv.Itoa(n) + " routes"
}

func (w Welcome) rightRows(th theme, width int) []line {
	heading := fg(th.accent).Bold(true)
	notes := w.Notes
	if len(notes) == 0 {
		notes = []string{reloadHint}
	}
	rows := make([]line, 0, len(w.Tips)+len(notes)+4)
	rows = append(rows, line{}, text("Try it", heading))
	for _, t := range w.Tips {
		var l line
		l.add(t.Command, fg(th.fg))
		if t.What != "" {
			l.add("  "+t.What, fg(th.dim))
		}
		rows = append(rows, l)
	}
	rows = append(rows, text(strings.Repeat("─", width), fg(th.faint)), text("Notes", heading))
	for _, n := range notes {
		rows = append(rows, text(n, fg(th.dim)))
	}
	return rows
}
