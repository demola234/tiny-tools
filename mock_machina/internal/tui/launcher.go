package tui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type Field struct {
	Label    string
	Flag     string
	Default  string
	Hint     string
	Choices  []string
	Toggle   bool
	Required bool
	Split    bool
}

type Action struct {
	Name   string
	Short  string
	Args   []string
	Fields []Field
}

type input struct {
	text   string
	choice int
	on     bool
}

type Launcher struct {
	version       string
	actions       []Action
	width, height int
	th            theme
	now           time.Time
	filter        string
	cur           int
	form          *Action
	inputs        []input
	focus         int
	problem       string
	chosen        []string
}

func NewLauncher(version string, actions []Action) Launcher {
	return Launcher{version: version, actions: actions, width: defaultWidth, height: defaultHeight, th: newTheme(true)}
}

func (l Launcher) Chosen() []string { return l.chosen }

func (Launcher) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, launcherTick())
}

func launcherTick() tea.Cmd {
	return tea.Tick(idleFrame, func(t time.Time) tea.Msg { return Tick(t) })
}

func (l Launcher) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case Tick:
		l.now = time.Time(msg)
		return l, launcherTick()
	case tea.BackgroundColorMsg:
		l.th = newTheme(msg.IsDark())
	case tea.WindowSizeMsg:
		l.width, l.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			l.chosen = nil
			return l, tea.Quit
		}
		if l.form != nil {
			return l.formKey(msg)
		}
		return l.menuKey(msg)
	}
	return l, nil
}

func (l Launcher) visible() []Action {
	if l.filter == "" {
		return l.actions
	}
	want := strings.ToLower(l.filter)
	var out []Action
	for _, a := range l.actions {
		if strings.Contains(strings.ToLower(a.Name+" "+a.Short), want) {
			out = append(out, a)
		}
	}
	return out
}

func (l Launcher) menuKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	visible := l.visible()
	switch msg.String() {
	case "esc":
		if l.filter == "" {
			return l, tea.Quit
		}
		l.filter, l.cur = "", 0
	case "up":
		l.cur = max(l.cur-1, 0)
	case "down":
		l.cur = min(l.cur+1, max(len(visible)-1, 0))
	case "backspace":
		if r := []rune(l.filter); len(r) > 0 {
			l.filter, l.cur = string(r[:len(r)-1]), 0
		}
	case "enter":
		if l.cur >= len(visible) {
			return l, nil
		}
		return l.open(visible[l.cur])
	default:
		if msg.Text != "" {
			l.filter, l.cur = l.filter+msg.Text, 0
		}
	}
	return l, nil
}

func (l Launcher) open(a Action) (tea.Model, tea.Cmd) {
	l.form, l.focus, l.problem = &a, 0, ""
	l.inputs = make([]input, len(a.Fields))
	for i, f := range a.Fields {
		l.inputs[i] = input{text: f.Default, choice: max(slices.Index(f.Choices, f.Default), 0), on: f.Default == "true"}
	}
	if len(a.Fields) == 0 {
		return l.run()
	}
	return l, nil
}

func (l Launcher) formKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	f := l.form.Fields[l.focus]
	in := &l.inputs[l.focus]
	switch msg.String() {
	case "esc":
		l.form = nil
	case "up", "shift+tab":
		l.focus = max(l.focus-1, 0)
	case "down", "tab":
		l.focus = min(l.focus+1, len(l.form.Fields)-1)
	case "enter":
		return l.run()
	case "left", "right":
		step := 1
		if msg.String() == "left" {
			step = -1
		}
		switch {
		case f.Toggle:
			in.on = !in.on
		case len(f.Choices) > 0:
			in.choice = (in.choice + step + len(f.Choices)) % len(f.Choices)
		}
	case "backspace":
		if r := []rune(in.text); len(r) > 0 && !f.Toggle && len(f.Choices) == 0 {
			in.text = string(r[:len(r)-1])
		}
	default:
		switch {
		case f.Toggle && msg.String() == "space":
			in.on = !in.on
		case !f.Toggle && len(f.Choices) == 0:
			in.text += msg.Text
		}
	}
	return l, nil
}

func (l Launcher) value(i int) string {
	f, in := l.form.Fields[i], l.inputs[i]
	switch {
	case f.Toggle:
		if in.on {
			return "true"
		}
		return ""
	case len(f.Choices) > 0:
		return f.Choices[in.choice]
	}
	return strings.TrimSpace(in.text)
}

func (l Launcher) args() ([]string, string) {
	args := slices.Clone(l.form.Args)
	var flags []string
	for i, f := range l.form.Fields {
		v := l.value(i)
		switch {
		case f.Required && v == "":
			return nil, f.Label + " is needed"
		case f.Flag == "" && v == "":
		case f.Flag == "" && f.Split:
			args = append(args, strings.Fields(v)...)
		case f.Flag == "":
			args = append(args, v)
		case f.Toggle && v != "":
			flags = append(flags, f.Flag)
		case !f.Toggle && v != "" && v != f.Default:
			flags = append(flags, f.Flag, v)
		}
	}
	return append(args, flags...), ""
}

func (l Launcher) run() (tea.Model, tea.Cmd) {
	args, problem := l.args()
	if problem != "" {
		l.problem = problem
		return l, nil
	}
	l.chosen = args
	return l, tea.Quit
}

func Quote(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if a == "" || strings.ContainsAny(a, " \t\"'$\\") {
			out[i] = strconv.Quote(a)
		}
	}
	return strings.Join(out, " ")
}
