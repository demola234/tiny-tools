package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

var operationName = regexp.MustCompile(`\b(?:query|mutation|subscription)\s+([_A-Za-z][_0-9A-Za-z]*)`)

type graphqlRequest struct {
	Query         string          `json:"query"`
	OperationName string          `json:"operationName,omitempty"`
	Variables     json.RawMessage `json:"variables,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
}

type graphqlErrors struct {
	Errors []graphqlError `json:"errors"`
}

type graphqlEndpoint struct {
	path     string
	routes   map[string]http.Handler
	reporter func() func(Request)
}

func (g *graphqlEndpoint) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	report := g.reporter()
	gr, raw := readGraphQL(req)
	req.Body = io.NopCloser(bytes.NewReader(raw))
	op := gr.OperationName
	if op == "" {
		if m := operationName.FindStringSubmatch(gr.Query); m != nil {
			op = m[1]
		}
	}
	if h, ok := g.routes[op]; ok {
		h.ServeHTTP(w, req)
		return
	}
	msg := "no mock for operation " + op
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range g.routes {
			if !yield(name) {
				return
			}
		}
	})
	if s, ok := suggest.Closest(op, names); ok {
		msg += " (did you mean " + s + "?)"
	} else if op == "" {
		msg = "the request names no operation; mocked operations: " + strings.Join(names, ", ")
	}
	writeJSON(w, http.StatusBadRequest, graphqlErrors{Errors: []graphqlError{{Message: msg}}})
	report(Request{Method: "GRAPHQL", Path: op, Status: http.StatusBadRequest, Problem: ProblemNoRoute})
}

func readGraphQL(req *http.Request) (graphqlRequest, []byte) {
	var gr graphqlRequest
	if req.Method == http.MethodGet {
		q := req.URL.Query()
		gr = graphqlRequest{Query: q.Get("query"), OperationName: q.Get("operationName")}
		if v := q.Get("variables"); json.Valid([]byte(v)) {
			gr.Variables = json.RawMessage(v)
		}
		raw, _ := json.Marshal(gr)
		return gr, raw
	}
	raw, _ := io.ReadAll(io.LimitReader(req.Body, maxBody))
	_ = json.Unmarshal(raw, &gr)
	return gr, raw
}

func graphqlEndpoints(routes []*model.Route, handlers map[*model.Route]http.Handler, reporter func() func(Request)) []*graphqlEndpoint {
	byPath := map[string]*graphqlEndpoint{}
	var out []*graphqlEndpoint
	for _, rt := range routes {
		if rt.Method != model.MethodGraphQL {
			continue
		}
		e, ok := byPath[rt.Path]
		if !ok {
			e = &graphqlEndpoint{path: rt.Path, routes: map[string]http.Handler{}, reporter: reporter}
			byPath[rt.Path] = e
			out = append(out, e)
		}
		e.routes[rt.Operation] = handlers[rt]
	}
	return out
}
