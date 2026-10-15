package model

import (
	"slices"
	"strings"
)

type Project struct {
	Config    Config
	Routes    []*Route
	Schemas   []*Schema
	Protos    map[string]string
	Scenarios []*Scenario
}

type Scenario struct {
	Name        string
	Title       string
	Description string
	States      []ScenarioState
	Src         Source
}

type ScenarioState struct {
	Route string
	State string
}

func (p *Project) Scenario(name string) (*Scenario, bool) {
	i, found := slices.BinarySearchFunc(p.Scenarios, name, func(s *Scenario, name string) int {
		return strings.Compare(s.Name, name)
	})
	if !found {
		return nil, false
	}
	return p.Scenarios[i], true
}

func (p *Project) Route(id string) (*Route, bool) {
	i, found := slices.BinarySearchFunc(p.Routes, id, func(r *Route, id string) int {
		return strings.Compare(r.ID, id)
	})
	if !found {
		return nil, false
	}
	return p.Routes[i], true
}
