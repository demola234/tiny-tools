package model

import (
	"slices"
	"time"
)

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
	Stream    *Stream
	Webhooks  []Webhook
	GRPC      GRPCStatus
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

type Stream struct {
	Send  []Message
	Reply []Reply
	Every time.Duration
	Push  []Message
	Close *Close
}

type Message struct {
	Data []byte
	Text bool
}

type Reply struct {
	When []Condition
	Send []Message
	Src  Source
}

type Close struct {
	After  time.Duration
	Code   int
	Reason string
	Drop   bool
}

type Webhook struct {
	URL     string
	Method  string
	After   time.Duration
	Headers map[string]string
	Body    Body
	Retries int
	Src     Source
}

type GRPCStatus struct {
	Code    string
	Message string
}

var grpcCodes = []string{
	"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED", "NOT_FOUND", "ALREADY_EXISTS",
	"PERMISSION_DENIED", "RESOURCE_EXHAUSTED", "FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE",
	"UNIMPLEMENTED", "INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED",
}

func GRPCCodes() []string { return slices.Clone(grpcCodes) }
