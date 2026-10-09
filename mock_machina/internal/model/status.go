package model

import "slices"

type Status string

const (
	StatusDraft       Status = "draft"
	StatusAgreed      Status = "agreed"
	StatusImplemented Status = "implemented"
	StatusDeprecated  Status = "deprecated"
)

var statuses = []Status{StatusDraft, StatusAgreed, StatusImplemented, StatusDeprecated}

func Statuses() []Status { return slices.Clone(statuses) }

func (s Status) Valid() bool { return slices.Contains(statuses, s) }

func (s Status) Effective() Status {
	if s == "" {
		return StatusDraft
	}
	return s
}

type Owners struct {
	Backend  []string
	Frontend []string
}

func (o Owners) IsZero() bool { return len(o.Backend) == 0 && len(o.Frontend) == 0 }

type Extension struct {
	Key   string
	Value any
}
