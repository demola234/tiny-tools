package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	maxRequests   = 1000
	defaultWidth  = 80
	defaultHeight = 24
)

type Header struct {
	Project string
	URL     string
	Seed    uint64
	Proxy   string
}

type Options struct {
	Header    Header
	SetActive func(route, state string) error
	Reload    func()
}

type Request struct {
	Time   time.Time
	Method string
	Path   string
	Status int
	Route  string
	State  string
	Note   string
}

type Reloaded struct{ Project *model.Project }

type Problems struct{ Lines []string }

type Note string

type setDone struct {
	route, state string
	err          error
}

type pane int

const (
	routesPane pane = iota
	requestsPane
)

type cursor struct {
	route string
	state int
}

type entry struct {
	Request
	seen time.Time
}

type pendingSet struct{ route, state string }

type toast struct {
	icon, text string
	kind       color.Color
	at         time.Time
}

type Model struct {
	opts          Options
	routes        []*model.Route
	width, height int
	cur           cursor
	filter        string
	filtering     bool
	focus         pane
	reqs          []entry
	sel           int
	problems      []string
	showProblems  bool
	th            theme
	now           time.Time
	pending       *pendingSet
	toast         toast
	popped        map[string]time.Time
}

func New(proj *model.Project, o Options) Model {
	m := Model{
		opts: o, routes: proj.Routes, width: defaultWidth, height: defaultHeight, sel: -1,
		th: newTheme(true), popped: map[string]time.Time{},
	}
	m.cur = m.fixCursor(cursor{state: -1})
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case Tick:
		m.now = time.Time(msg)
		return m, m.tick()
	case tea.BackgroundColorMsg:
		m.th = newTheme(msg.IsDark())
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		if m.filtering {
			return m.filterKey(msg)
		}
		return m.key(msg)
	case Request:
		m.addRequest(msg)
	case Reloaded:
		m.reloaded(msg.Project)
	case Problems:
		m.problems = msg.Lines
		m.showProblems = m.showProblems && len(msg.Lines) > 0
	case Note:
		m.toast = toast{icon: "↻", text: string(msg), kind: m.th.info, at: m.now}
	case setDone:
		m.pending = nil
		m.toast = toast{icon: "✓", text: fmt.Sprintf("%s now serves %s", msg.route, msg.state), kind: m.th.ok, at: m.now}
		if msg.err != nil {
			m.toast = toast{icon: "✗", text: msg.err.Error(), kind: m.th.err, at: m.now}
		}
	}
	return m, nil
}

func (m *Model) reloaded(p *model.Project) {
	before := map[string]string{}
	for _, r := range m.routes {
		before[r.ID] = r.Active
	}
	for _, r := range p.Routes {
		if was, ok := before[r.ID]; ok && was != r.Active {
			m.popped[r.ID] = m.now
		}
	}
	m.routes = p.Routes
	m.cur = m.fixCursor(m.cur)
}

func (m Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.toggleFocus()
	case "/":
		m.filtering, m.filter = true, ""
		m.cur = m.fixCursor(m.cur)
	case "r":
		return m, m.reload()
	case "l":
		m.showProblems = !m.showProblems
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter":
		cmd := m.setActive()
		return m, cmd
	}
	return m, nil
}

func (m Model) filterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.filtering, m.filter = false, ""
	case "enter":
		m.filtering = false
	case "backspace":
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
		}
	default:
		m.filter += msg.Text
	}
	m.cur = m.fixCursor(m.cur)
	return m, nil
}

func (m *Model) toggleFocus() {
	if m.focus == requestsPane {
		m.focus, m.sel = routesPane, -1
		return
	}
	m.focus, m.sel = requestsPane, len(m.reqs)-1
}

func (m Model) reload() tea.Cmd {
	if m.opts.Reload == nil {
		return nil
	}
	return func() tea.Msg {
		m.opts.Reload()
		return nil
	}
}

func (m *Model) setActive() tea.Cmd {
	r, ok := m.current()
	if !ok || m.focus != routesPane || m.cur.state < 0 || m.opts.SetActive == nil {
		return nil
	}
	state := r.States[m.cur.state].Name
	set := m.opts.SetActive
	m.pending = &pendingSet{route: r.ID, state: state}
	return func() tea.Msg {
		return setDone{route: r.ID, state: state, err: set(r.ID, state)}
	}
}

func (m *Model) addRequest(r Request) {
	m.reqs = append(m.reqs, entry{Request: r, seen: m.now})
	if extra := len(m.reqs) - maxRequests; extra > 0 {
		m.reqs = m.reqs[extra:]
		if m.sel >= 0 {
			m.sel = max(m.sel-extra, 0)
		}
	}
}

func (m *Model) move(by int) {
	if m.focus == requestsPane {
		if len(m.reqs) > 0 {
			m.sel = min(max(m.sel+by, 0), len(m.reqs)-1)
		}
		return
	}
	visible := m.visible()
	i := m.index(visible)
	if i < 0 {
		return
	}
	states := len(visible[i].States)
	switch {
	case by > 0 && m.cur.state < states-1:
		m.cur.state++
	case by > 0 && i < len(visible)-1:
		m.cur = cursor{route: visible[i+1].ID, state: -1}
	case by < 0 && m.cur.state >= 0:
		m.cur.state--
	case by < 0 && i > 0:
		m.cur = cursor{route: visible[i-1].ID, state: len(visible[i-1].States) - 1}
	}
}

func (m Model) visible() []*model.Route {
	if m.filter == "" {
		return m.routes
	}
	want := strings.ToLower(m.filter)
	var out []*model.Route
	for _, r := range m.routes {
		if strings.Contains(strings.ToLower(r.ID), want) || strings.Contains(strings.ToLower(r.Path), want) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) index(visible []*model.Route) int {
	for i, r := range visible {
		if r.ID == m.cur.route {
			return i
		}
	}
	return -1
}

func (m Model) current() (*model.Route, bool) {
	visible := m.visible()
	if i := m.index(visible); i >= 0 {
		return visible[i], true
	}
	return nil, false
}

func (m Model) fixCursor(c cursor) cursor {
	m.cur = c
	r, ok := m.current()
	if !ok {
		visible := m.visible()
		if len(visible) == 0 {
			return cursor{state: -1}
		}
		return cursor{route: visible[0].ID, state: -1}
	}
	c.state = min(c.state, len(r.States)-1)
	return c
}
