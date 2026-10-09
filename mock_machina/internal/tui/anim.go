package tui

import (
	"image/color"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	fastFrame   = 50 * time.Millisecond
	idleFrame   = 200 * time.Millisecond
	flashFor    = 900 * time.Millisecond
	popFor      = 450 * time.Millisecond
	toastFor    = 4 * time.Second
	toastFade   = time.Second
	breathCycle = 1600 * time.Millisecond
	blinkEvery  = 530 * time.Millisecond
	dotsEvery   = 400 * time.Millisecond
	spinEvery   = 80 * time.Millisecond
	rateBuckets = 10
	rateBucket  = 2 * time.Second
	blendSteps  = 12
)

var (
	spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	pops    = []string{"◎", "◉", "●"}
	bars    = []rune("▁▂▃▄▅▆▇█")
)

type Tick time.Time

func (m Model) tick() tea.Cmd {
	every := idleFrame
	if m.animating() {
		every = fastFrame
	}
	return tea.Tick(every, func(t time.Time) tea.Msg { return Tick(t) })
}

func (m Model) animating() bool {
	if m.pending != nil || (m.toast.text != "" && m.since(m.toast.at) < toastFor) {
		return true
	}
	if n := len(m.reqs); n > 0 && m.since(m.reqs[n-1].seen) < flashFor {
		return true
	}
	for _, at := range m.popped {
		if m.since(at) < popFor {
			return true
		}
	}
	return false
}

func (m Model) since(t time.Time) time.Duration { return m.now.Sub(t) }

func phase(d, cycle time.Duration, steps int) int {
	if d < 0 || cycle <= 0 {
		return 0
	}
	return int(d%cycle) * steps / int(cycle)
}

func fade(age, length time.Duration, from, to color.Color) (color.Color, bool) {
	if age < 0 || age >= length {
		return nil, false
	}
	return lipgloss.Blend1D(blendSteps, from, to)[int(age)*blendSteps/int(length)], true
}

func (m Model) breath() color.Color {
	steps := lipgloss.Blend1D(blendSteps, m.th.accent, m.th.faint)
	i := phase(m.clock(), breathCycle, 2*blendSteps)
	if i >= blendSteps {
		i = 2*blendSteps - 1 - i
	}
	return steps[i]
}

func (m Model) spin() string {
	return spinner[phase(m.clock(), spinEvery*time.Duration(len(spinner)), len(spinner))]
}

func (m Model) blink() bool { return phase(m.clock(), 2*blinkEvery, 2) == 0 }

func (m Model) dots() string {
	n := phase(m.clock(), 4*dotsEvery, 4)
	return strings.Repeat("·", n) + strings.Repeat(" ", 3-n)
}

func (m Model) pop(route string) (string, bool) {
	at, ok := m.popped[route]
	if !ok || m.since(at) >= popFor || m.since(at) < 0 {
		return "", false
	}
	return pops[int(m.since(at))*len(pops)/int(popFor)], true
}

func (m Model) rate() (string, int) {
	var counts [rateBuckets]int
	total := 0
	for _, e := range slices.Backward(m.reqs) {
		age := m.now.Sub(e.Time)
		if age >= rateBuckets*rateBucket {
			break
		}
		if age < 0 {
			continue
		}
		counts[rateBuckets-1-int(age/rateBucket)]++
		total++
	}
	peak := 1
	for _, c := range counts {
		peak = max(peak, c)
	}
	var b strings.Builder
	for _, c := range counts {
		b.WriteRune(bars[c*(len(bars)-1)/peak])
	}
	return b.String(), total
}

func (m Model) clock() time.Duration { return time.Duration(m.now.UnixNano()) }
