package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	launcherChrome = 5
	menuKeys       = "type to filter · ↑↓ move · ⏎ choose · esc quit"
	formKeys       = "↑↓ field · ←→ choose · space toggle · ⏎ run · esc back"
)

func (l Launcher) View() tea.View {
	w, h := max(l.width, minWidth), max(l.height, launcherChrome+3)
	inner, body := w-4, h-launcherChrome

	lines := make([]string, 0, h)
	lines = append(lines, l.header().pad(w))
	title, rows, keys := l.menuTitle(), l.withMascot(l.menuRows(body), inner), menuKeys
	if l.form != nil {
		title, rows, keys = text(l.form.Name, fg(l.th.accent).Bold(true)), l.formRows(), formKeys
	}
	lines = append(lines, drawBox(l.th, title, true, inner, rows, body)...)
	lines = append(lines, l.status().pad(w), text(keys, fg(l.th.dim)).pad(w))

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (ln line) pad(width int) string {
	edge := ln.style(lipgloss.NewStyle()).Render(" ")
	return edge + ln.render(width-2) + edge
}

func (l Launcher) blink() bool { return phase(time.Duration(l.now.UnixNano()), 2*blinkEvery, 2) == 0 }

func (l Launcher) header() line {
	m := Model{th: l.th, now: l.now}
	var h line
	h.add("●", fg(m.breath())).add(" ", lipgloss.NewStyle())
	h.spans = append(h.spans, gradient("MockMachina", l.th.accent, l.th.accent2)...)
	if l.version != "" {
		h.add(" "+l.version, fg(l.th.dim))
	}
	return *h.add("  pick a command, or run mockmachina <command> --help", fg(l.th.faint))
}

func (l Launcher) menuTitle() line {
	var t line
	return *t.add("Commands", fg(l.th.accent).Bold(true)).add(" "+strconv.Itoa(len(l.visible())), fg(l.th.dim))
}

func (l Launcher) cursor() string {
	if l.blink() {
		return "▏"
	}
	return " "
}

func (l Launcher) menuRows(height int) []line {
	var search line
	search.add("› ", fg(l.th.accent).Bold(true))
	if l.filter == "" {
		search.add(l.cursor(), fg(l.th.accent)).add("type to filter", fg(l.th.faint))
	} else {
		search.add(l.filter, fg(l.th.fg)).add(l.cursor(), fg(l.th.accent))
	}
	rows := []line{search, {}}
	visible := l.visible()
	if len(visible) == 0 {
		return append(rows, text("no command matches "+strconv.Quote(l.filter), fg(l.th.dim).Italic(true)))
	}
	nameWidth := 0
	for _, a := range visible {
		nameWidth = max(nameWidth, len(a.Name))
	}
	first := max(l.cur-(height-len(rows))+1, 0)
	for i := first; i < len(visible); i++ {
		a, on := visible[i], i == l.cur
		var r line
		if on {
			r.bg = l.th.selection
		}
		ptr, name := "  ", fg(l.th.fg)
		if on {
			ptr, name = "▸ ", fg(l.th.accent).Bold(true)
		}
		r.add(ptr, fg(l.th.accent).Bold(true)).add(a.Name+strings.Repeat(" ", nameWidth-len(a.Name)), name)
		r.add("   "+a.Short, fg(l.th.dim))
		if len(a.Fields) > 0 {
			r.add(" …", fg(l.th.faint))
		}
		rows = append(rows, r)
	}
	return rows
}

func (l Launcher) formRows() []line {
	rows := []line{text(l.form.Short, fg(l.th.dim)), {}}
	labelWidth := 0
	for _, f := range l.form.Fields {
		labelWidth = max(labelWidth, len(f.Label))
	}
	for i, f := range l.form.Fields {
		on := i == l.focus
		var r line
		if on {
			r.bg = l.th.selection
		}
		ptr, label := "  ", fg(l.th.dim)
		if on {
			ptr, label = "▸ ", fg(l.th.accent).Bold(true)
		}
		r.add(ptr, fg(l.th.accent).Bold(true)).add(f.Label+strings.Repeat(" ", labelWidth-len(f.Label))+"  ", label)
		l.fieldValue(&r, f, l.inputs[i], on)
		rows = append(rows, r)
	}
	if l.problem != "" {
		rows = append(rows, line{}, text("⚠ "+l.problem, fg(l.th.err).Bold(true)))
	}
	return rows
}

func (l Launcher) fieldValue(r *line, f Field, in input, on bool) {
	switch {
	case f.Toggle:
		if in.on {
			r.add("● on", fg(l.th.ok).Bold(true))
		} else {
			r.add("○ off", fg(l.th.dim))
		}
	case len(f.Choices) > 0:
		arrow := fg(l.th.faint)
		if on {
			arrow = fg(l.th.accent)
		}
		r.add("‹ ", arrow).add(f.Choices[in.choice], fg(l.th.fg).Bold(true)).add(" ›", arrow)
		r.add("  "+strconv.Itoa(in.choice+1)+"/"+strconv.Itoa(len(f.Choices)), fg(l.th.faint))
	default:
		r.add(in.text, fg(l.th.fg))
		if on {
			r.add(l.cursor(), fg(l.th.accent))
		}
		if in.text == "" && f.Hint != "" {
			r.add(f.Hint, fg(l.th.faint).Italic(true))
		}
	}
}

func (l Launcher) status() line {
	var args []string
	switch {
	case l.form != nil:
		args, _ = l.args()
		if args == nil {
			args = l.form.Args
		}
	default:
		if visible := l.visible(); l.cur < len(visible) {
			args = visible[l.cur].Args
		}
	}
	if args == nil {
		return line{}
	}
	var s line
	return *s.add("$ ", fg(l.th.accent).Bold(true)).add("mockmachina "+Quote(args), fg(l.th.fg))
}

func (l Launcher) withMascot(rows []line, inner int) []line {
	const top = 2
	art := l.th.mascot(time.Duration(l.now.UnixNano()))
	for len(rows) < top+len(art) {
		rows = append(rows, line{})
	}
	for i, a := range art {
		r := &rows[top+i]
		gap := inner - mascotWide - 1 - r.width()
		if gap < 2 {
			return rows
		}
		r.add(strings.Repeat(" ", gap), lipgloss.NewStyle())
		r.spans = append(r.spans, a.spans...)
	}
	return rows
}
