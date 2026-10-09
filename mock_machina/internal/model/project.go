package model

import (
	"slices"
	"strings"
)

type Project struct {
	Config  Config
	Routes  []*Route
	Schemas []*Schema
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
