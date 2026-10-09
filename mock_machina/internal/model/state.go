package model

import "time"

type States []NamedState

type NamedState struct {
	Name  string
	State *State
}

func (s States) Get(name string) (*State, bool) {
	for _, ns := range s {
		if ns.Name == name {
			return ns.State, true
		}
	}
	return nil, false
}

func (s States) Names() []string {
	names := make([]string, len(s))
	for i, ns := range s {
		names[i] = ns.Name
	}
	return names
}

type State struct {
	Status    int
	Headers   map[string]string
	Body      Body
	Latency   time.Duration
	Jitter    time.Duration
	Fault     *Fault
	Set       []Assignment
	Generated bool
	Verbatim  bool

	SkipRequestValidation bool
	Extensions            []Extension
	Src                   Source
}

const defaultStatus = 200

func (s *State) EffectiveStatus() int {
	if s.Status == 0 {
		return defaultStatus
	}
	return s.Status
}

type Body struct {
	File        string
	Data        []byte
	ContentType string
	Generate    bool
}

type Fault struct {
	Type  FaultType
	Rate  float64
	After time.Duration
}

type FaultType string

const (
	FaultTimeout   FaultType = "timeout"
	FaultReset     FaultType = "reset"
	FaultTruncated FaultType = "truncated"
)

type Assignment struct {
	Name  string
	Value any
}
