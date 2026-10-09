package config

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Severity int

const (
	Error Severity = iota
	Warning
)

type Problem struct {
	File     string
	Line     int
	Severity Severity
	Msg      string
}

func (p Problem) String() string {
	msg := p.Msg
	if p.Severity == Warning {
		msg = "warning: " + msg
	}
	if p.Line == 0 {
		return p.File + ": " + msg
	}
	return p.File + ":" + strconv.Itoa(p.Line) + ": " + msg
}

type Problems []Problem

func (ps Problems) String() string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n")
}

func (ps Problems) Summary() string {
	if len(ps) == 0 {
		return "no problems"
	}
	files := make(map[string]struct{}, len(ps))
	for _, p := range ps {
		files[p.File] = struct{}{}
	}
	return plural(len(ps), "problem") + " in " + plural(len(files), "file")
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (ps Problems) sort() {
	slices.SortStableFunc(ps, func(a, b Problem) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
}

func (ps Problems) HasErrors() bool {
	return slices.ContainsFunc(ps, func(p Problem) bool { return p.Severity == Error })
}

func (ps Problems) Sorted() Problems {
	sorted := slices.Clone(ps)
	sorted.sort()
	return sorted
}
