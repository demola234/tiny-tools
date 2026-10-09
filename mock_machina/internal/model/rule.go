package model

import (
	"regexp"
	"slices"
)

type Mode string

const (
	ModeActive     Mode = "active"
	ModeRules      Mode = "rules"
	ModeSequential Mode = "sequential"
	ModeRandom     Mode = "random"
)

var modes = []Mode{ModeActive, ModeRules, ModeSequential, ModeRandom}

func Modes() []Mode { return slices.Clone(modes) }

type Rule struct {
	When  []Condition
	State string
	Src   Source
}

type Condition struct {
	Scope   Scope
	Key     string
	Op      Op
	Value   any
	Values  []any
	Pattern *regexp.Regexp
}

type Scope string

const (
	ScopePath   Scope = "path"
	ScopeQuery  Scope = "query"
	ScopeHeader Scope = "header"
	ScopeCookie Scope = "cookie"
	ScopeBody   Scope = "body"
	ScopeVar    Scope = "var"
	ScopeCall   Scope = "call"
)

var scopes = []Scope{ScopePath, ScopeQuery, ScopeHeader, ScopeCookie, ScopeBody, ScopeVar, ScopeCall}

func Scopes() []Scope { return slices.Clone(scopes) }

type Op string

const (
	OpEq      Op = "eq"
	OpNe      Op = "ne"
	OpIn      Op = "in"
	OpMatches Op = "matches"
	OpExists  Op = "exists"
	OpGt      Op = "gt"
	OpGte     Op = "gte"
	OpLt      Op = "lt"
	OpLte     Op = "lte"
)

var ops = []Op{OpEq, OpNe, OpIn, OpMatches, OpExists, OpGt, OpGte, OpLt, OpLte}

func Ops() []Op { return slices.Clone(ops) }
