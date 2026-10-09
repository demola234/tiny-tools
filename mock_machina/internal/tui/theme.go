package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type theme struct {
	accent, accent2, fg, dim, faint color.Color
	ok, warn, err, info             color.Color
	selection, flash, bg            color.Color
}

func newTheme(dark bool) theme {
	pick := lipgloss.LightDark(dark)
	c := func(light, darkHex string) color.Color {
		return pick(lipgloss.Color(light), lipgloss.Color(darkHex))
	}
	return theme{
		accent:    c("#7C3AED", "#A78BFA"),
		accent2:   c("#DB2777", "#F472B6"),
		fg:        c("#1F2937", "#E5E7EB"),
		dim:       c("#6B7280", "#8B8FA3"),
		faint:     c("#D1D5DB", "#3F3F55"),
		ok:        c("#059669", "#34D399"),
		warn:      c("#B45309", "#FBBF24"),
		err:       c("#DC2626", "#F87171"),
		info:      c("#2563EB", "#60A5FA"),
		selection: c("#EDE9FE", "#2E2A4A"),
		flash:     c("#C4B5FD", "#5B4BA8"),
		bg:        c("#FFFFFF", "#16161E"),
	}
}

func fg(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func (t theme) method(m string) lipgloss.Style {
	switch m {
	case "GET":
		return fg(t.ok).Bold(true)
	case "POST":
		return fg(t.info).Bold(true)
	case "PUT", "PATCH":
		return fg(t.warn).Bold(true)
	case "DELETE":
		return fg(t.err).Bold(true)
	}
	return fg(t.dim).Bold(true)
}

func (t theme) status(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return fg(t.err)
	case code >= 400:
		return fg(t.warn)
	case code >= 300:
		return fg(t.info)
	case code >= 200:
		return fg(t.ok)
	}
	return fg(t.dim)
}

type span struct {
	text string
	st   lipgloss.Style
}

type line struct {
	spans []span
	bg    color.Color
}

func (l *line) add(text string, st lipgloss.Style) *line {
	l.spans = append(l.spans, span{text: text, st: st})
	return l
}

func (l line) width() int {
	w := 0
	for _, s := range l.spans {
		w += ansi.StringWidth(s.text)
	}
	return w
}

func (l line) style(st lipgloss.Style) lipgloss.Style {
	if l.bg != nil {
		return st.Background(l.bg)
	}
	return st
}

func (l line) render(width int) string {
	over := l.width() > width
	budget := width
	if over {
		budget = width - 1
	}
	var b strings.Builder
	last := lipgloss.NewStyle()
	for _, s := range l.spans {
		if budget <= 0 {
			break
		}
		text := s.text
		if ansi.StringWidth(text) > budget {
			text = ansi.Truncate(text, budget, "")
		}
		budget -= ansi.StringWidth(text)
		last = s.st
		b.WriteString(l.style(s.st).Render(text))
	}
	used := min(l.width(), width)
	if over {
		b.WriteString(l.style(last).Render("…"))
	}
	if pad := width - used; pad > 0 {
		b.WriteString(l.style(lipgloss.NewStyle()).Render(strings.Repeat(" ", pad)))
	}
	return b.String()
}

func gradient(text string, from, to color.Color) []span {
	runes := []rune(text)
	colors := lipgloss.Blend1D(len(runes), from, to)
	spans := make([]span, len(runes))
	for i, r := range runes {
		spans[i] = span{text: string(r), st: lipgloss.NewStyle().Foreground(colors[i]).Bold(true)}
	}
	return spans
}
