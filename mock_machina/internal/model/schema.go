package model

type Schema struct {
	Name  string
	JSON  []byte
	Lines map[string]int
	Src   Source
}

type SchemaRef struct {
	Name   string
	Inline *Schema
}

type Request struct {
	Params  []Param
	Query   []Param
	Headers []Param
	Body    *SchemaRef
}

type Param struct {
	Name     string
	Required bool
	Schema   SchemaRef
}

type Response struct {
	Status string
	Schema SchemaRef
}
