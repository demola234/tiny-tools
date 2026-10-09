package model

import (
	"regexp"
	"slices"
)

type Route struct {
	ID         string
	Method     Method
	Path       string
	Summary    string
	Status     Status
	Owners     Owners
	Group      string
	Examples   map[string]string
	Active     string
	Mode       Mode
	Rules      []Rule
	Serve      Serve
	CRUD       *CRUD
	Request    *Request
	Responses  []Response
	Generated  bool
	States     States
	Extensions []Extension
	Dir        string
	Src        Source
}

var pathParam = regexp.MustCompile(`\{[^/}]*\}`)

func (r *Route) Key() string {
	return string(r.Method) + " " + pathParam.ReplaceAllString(r.Path, "{}")
}

type Method string

const (
	MethodGet     Method = "GET"
	MethodPost    Method = "POST"
	MethodPut     Method = "PUT"
	MethodPatch   Method = "PATCH"
	MethodDelete  Method = "DELETE"
	MethodHead    Method = "HEAD"
	MethodOptions Method = "OPTIONS"
	MethodCRUD    Method = "CRUD"
)

var methods = []Method{
	MethodGet, MethodPost, MethodPut, MethodPatch,
	MethodDelete, MethodHead, MethodOptions,
}

func Methods() []Method { return slices.Clone(methods) }

func (m Method) Valid() bool { return slices.Contains(methods, m) }

type Serve string

const (
	ServeMock  Serve = "mock"
	ServeProxy Serve = "proxy"
)

type CRUD struct {
	Collection string
	IDField    string
	Data       []byte
}
