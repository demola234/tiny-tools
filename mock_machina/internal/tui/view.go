package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	chrome    = 5
	paneGaps  = 9
	minWidth  = 30
	timeShape = "15:04:05"
)

var keyHints = [][2]string{
	{"↑↓", "move"},
	{"⏎", "set state"},
	{"/", "filter"},
	{"tab", "requests"},
	{"r", "reload"},
	{"l", "lint"},
	{"q", "quit"},
}

func (m Model) View() tea.View {
	w, h := max(m.width, minWidth), max(m.height, chrome+1)
	left := (w - paneGaps) * 2 / 5
	right := w - paneGaps - left
	body := h - chrome

	lines := make([]string, 0, h)
	lines = append(lines, m.header(w))
	lb := m.box(m.routesTitle(), m.focus == routesPane && !m.showProblems, left, m.routeRows(body), body)
	rb := m.box(m.rightTitle(), m.focus == requestsPane || m.showProblems, right, m.rightRows(body, right), body)
	for i := range lb {
		lines = append(lines, lb[i]+" "+rb[i])
	}
	lines = append(lines, " "+m.status().render(w-2)+" ", " "+m.keys().render(w-2)+" ")

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (m Model) header(w int) string {
	var l line
	l.add(" ", lipgloss.NewStyle()).add("●", fg(m.breath())).add(" ", lipgloss.NewStyle())
	l.spans = append(l.spans, gradient("mockmachina", m.th.accent, m.th.accent2)...)
	h := m.opts.Header
	if h.Project != "" {
		l.add("  "+h.Project, fg(m.th.fg).Bold(true))
	}
	if h.URL != "" {
		l.add("  "+h.URL, fg(m.th.info).Underline(true))
	}
	if h.Seed != 0 {
		l.add("  seed "+strconv.FormatUint(h.Seed, 10), fg(m.th.dim))
	}
	if h.Proxy != "" {
		l.add("  → "+h.Proxy, fg(m.th.dim))
	}

	for _, r := range m.headerRights() {
		if rw := r.width(); l.width()+rw+3 <= w {
			return l.render(w-rw-1) + r.render(rw) + " "
		}
	}
	return l.render(w)
}

func (m Model) headerRights() []line {
	var badge line
	if n := len(m.problems); n > 0 {
		badge.add("  ⚠ "+strconv.Itoa(n), fg(m.th.err).Bold(true))
	}
	spark, total := m.rate()
	var full line
	full.add(spark, fg(m.th.accent)).add(fmt.Sprintf(" %d req · 20s", total), fg(m.th.dim))
	full.spans = append(full.spans, badge.spans...)
	return []line{full, badge}
}

func (m Model) box(title line, focused bool, inner int, rows []line, height int) []string {
	return drawBox(m.th, title, focused, inner, rows, height)
}

func drawBox(th theme, title line, focused bool, inner int, rows []line, height int) []string {
	border := fg(th.faint)
	if focused {
		border = fg(th.accent)
	}
	tw := min(title.width(), inner-1)
	out := make([]string, 0, height+2)
	out = append(out, border.Render("╭─ ")+title.render(tw)+border.Render(" "+strings.Repeat("─", inner-tw-1)+"╮"))
	for i := range height {
		var l line
		if i < len(rows) {
			l = rows[i]
		}
		pad := l.style(lipgloss.NewStyle()).Render(" ")
		out = append(out, border.Render("│")+pad+l.render(inner)+pad+border.Render("│"))
	}
	return append(out, border.Render("╰"+strings.Repeat("─", inner+2)+"╯"))
}

func (m Model) paneTitle(name string, count int, focused bool) line {
	var l line
	st := fg(m.th.dim).Bold(true)
	if focused {
		st = fg(m.th.accent).Bold(true)
	}
	l.add(name, st).add(" "+strconv.Itoa(count), fg(m.th.dim))
	return l
}

func (m Model) routesTitle() line {
	l := m.paneTitle("Routes", len(m.visible()), m.focus == routesPane && !m.showProblems)
	if m.filtering || m.filter != "" {
		l.add("  /"+m.filter, fg(m.th.warn))
		if m.filtering && m.blink() {
			l.add("▏", fg(m.th.warn))
		}
	}
	return l
}

func (m Model) rightTitle() line {
	if m.showProblems {
		var l line
		return *l.add("Problems", fg(m.th.err).Bold(true)).add(" "+strconv.Itoa(len(m.problems)), fg(m.th.dim))
	}
	return m.paneTitle("Requests", len(m.reqs), m.focus == requestsPane)
}

func (m Model) pointer(l *line, on bool) {
	if on {
		l.add("▸ ", fg(m.th.accent).Bold(true))
		return
	}
	l.add("  ", lipgloss.NewStyle())
}

func (m Model) routeRows(height int) []line {
	visible := m.visible()
	if len(visible) == 0 {
		var l line
		if m.filter != "" {
			return []line{*l.add("no routes match /"+m.filter, fg(m.th.dim).Italic(true))}
		}
		return []line{*l.add("no routes", fg(m.th.dim).Italic(true))}
	}
	var rows []line
	at := 0
	for _, r := range visible {
		here := r.ID == m.cur.route
		if here && m.cur.state < 0 {
			at = len(rows)
		}
		rows = append(rows, m.routeRow(r, here && m.cur.state < 0))
		if !here {
			continue
		}
		for i, s := range r.States {
			if i == m.cur.state {
				at = len(rows)
			}
			rows = append(rows, m.stateRow(r, s, i == m.cur.state))
		}
	}
	return rows[max(at-height+1, 0):]
}

func (m Model) selected(on bool) line {
	if on && m.focus == routesPane {
		return line{bg: m.th.selection}
	}
	return line{}
}

func (m Model) routeRow(r *model.Route, on bool) line {
	l := m.selected(on)
	m.pointer(&l, on)
	id := fg(m.th.fg)
	if on {
		id = id.Bold(true)
	}
	l.add(r.ID, id).add("  ", lipgloss.NewStyle()).add(string(r.Method), m.th.method(string(r.Method))).add(" "+r.Path, fg(m.th.dim))
	return l
}

func (m Model) stateRow(r *model.Route, s model.NamedState, on bool) line {
	l := m.selected(on)
	m.pointer(&l, on)
	l.add("  ", lipgloss.NewStyle())
	name := fg(m.th.fg)
	switch {
	case m.pending != nil && m.pending.route == r.ID && m.pending.state == s.Name:
		l.add(m.spin(), fg(m.th.accent).Bold(true))
	case r.Active == s.Name:
		frame, popping := m.pop(r.ID)
		if !popping {
			frame = "●"
		}
		l.add(frame, fg(m.th.ok).Bold(true))
		name = name.Bold(true)
	default:
		l.add("○", fg(m.th.faint))
		name = fg(m.th.dim)
	}
	l.add(" "+s.Name, name)
	if s.State != nil && s.State.Status != 0 {
		l.add("  "+strconv.Itoa(s.State.Status), m.th.status(s.State.Status))
	}
	return l
}

func (m Model) rightRows(height, width int) []line {
	if m.showProblems {
		return m.problemRows(height)
	}
	if len(m.reqs) == 0 {
		return m.waiting(height, width)
	}
	first := max(len(m.reqs)-height, 0)
	if m.sel >= 0 && m.sel < first {
		first = m.sel
	}
	last := min(first+height, len(m.reqs))
	rows := make([]line, 0, last-first)
	for i := first; i < last; i++ {
		rows = append(rows, m.requestRow(m.reqs[i], i == m.sel))
	}
	return rows
}

func (m Model) problemRows(height int) []line {
	if len(m.problems) == 0 {
		var l line
		return []line{*l.add("no problems", fg(m.th.ok))}
	}
	shown := m.problems[max(len(m.problems)-height, 0):]
	rows := make([]line, 0, len(shown))
	for _, p := range shown {
		var l line
		rows = append(rows, *l.add("⚠ ", fg(m.th.warn)).add(p, fg(m.th.fg)))
	}
	return rows
}

func (m Model) requestRow(e entry, on bool) line {
	var l line
	if bg, ok := fade(m.since(e.seen), flashFor, m.th.flash, m.th.bg); ok {
		l.bg = bg
	}
	if on {
		l.bg = m.th.selection
	}
	m.pointer(&l, on)
	l.add(e.Time.Format(timeShape), fg(m.th.dim)).add("  ", lipgloss.NewStyle())
	l.add(fmt.Sprintf("%-6s", e.Method), m.th.method(e.Method)).add(e.Path, fg(m.th.fg))
	l.add("  "+strconv.Itoa(e.Status), m.th.status(e.Status).Bold(true))
	if e.State != "" {
		l.add("  "+e.State, fg(m.th.dim))
	}
	if e.Note != "" {
		l.add("  "+e.Note, fg(m.th.warn))
	}
	return l
}

func (m Model) status() line {
	var l line
	switch {
	case m.filtering:
		l.add("/", fg(m.th.accent).Bold(true)).add(m.filter, fg(m.th.fg))
		if m.blink() {
			l.add("▏", fg(m.th.accent))
		}
		l.add("  enter keeps it · esc clears it", fg(m.th.dim))
	case m.focus == requestsPane && m.sel >= 0 && m.sel < len(m.reqs):
		return m.detail(m.reqs[m.sel])
	case m.toast.text != "" && m.since(m.toast.at) < toastFor:
		text := m.th.fg
		if c, ok := fade(m.since(m.toast.at)-(toastFor-toastFade), toastFade, m.th.fg, m.th.faint); ok {
			text = c
		}
		l.add(m.toast.icon+" ", fg(m.toast.kind).Bold(true)).add(firstLine(m.toast.text), fg(text))
	case len(m.problems) > 0:
		l.add("⚠ "+problemCount(len(m.problems))+"; still serving the last good version", fg(m.th.warn))
		l.add(" · l lists them", fg(m.th.dim))
	}
	return l
}

func problemCount(n int) string {
	if n == 1 {
		return "1 problem"
	}
	return strconv.Itoa(n) + " problems"
}

func (m Model) detail(e entry) line {
	var l line
	l.add(e.Method, m.th.method(e.Method)).add(" "+e.Path, fg(m.th.fg)).add(" · ", fg(m.th.faint))
	l.add(strconv.Itoa(e.Status), m.th.status(e.Status).Bold(true))
	if e.Route != "" {
		l.add(" · ", fg(m.th.faint)).add(e.Route+":"+e.State, fg(m.th.accent))
	}
	if e.Note != "" {
		l.add(" · ", fg(m.th.faint)).add(e.Note, fg(m.th.warn))
	}
	return l
}

func (m Model) keys() line {
	var l line
	for i, k := range keyHints {
		if i > 0 {
			l.add("  ", lipgloss.NewStyle())
		}
		l.add(k[0], fg(m.th.accent).Bold(true)).add(" "+k[1], fg(m.th.dim))
	}
	return l
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

func (m Model) waiting(height, width int) []line {
	note := text("waiting for requests"+m.dots(), fg(m.th.dim).Italic(true))
	art := m.th.mascot(m.clock())
	if height < len(art)+3 {
		return []line{note}
	}
	rows := make([]line, 0, height)
	for range (height - len(art) - 2) / 2 {
		rows = append(rows, line{})
	}
	for _, r := range art {
		rows = append(rows, center(r, width))
	}
	return append(rows, line{}, center(note, width))
}
