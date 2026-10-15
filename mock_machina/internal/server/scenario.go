package server

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const scenarioHeader = "X-Mock-Scenario"

type scenarios map[string]map[string]string

func indexScenarios(list []*model.Scenario) scenarios {
	out := make(scenarios, len(list))
	for _, s := range list {
		states := make(map[string]string, len(s.States))
		for _, st := range s.States {
			states[st.Route] = st.State
		}
		out[s.Name] = states
	}
	return out
}

func (s scenarios) state(req *http.Request, route string) (string, bool) {
	state, ok := s[req.Header.Get(scenarioHeader)][route]
	return state, ok
}

func (s scenarios) unknown(req *http.Request) (string, bool) {
	name := req.Header.Get(scenarioHeader)
	if _, ok := s[name]; name == "" || ok {
		return "", false
	}
	return name, true
}

func (s scenarios) names() []string {
	out := make([]string, 0, len(s))
	for name := range s {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func (s scenarios) unknownMessage(name string) string {
	names := s.names()
	if h := hint(name, names); h != "" {
		return fmt.Sprintf("unknown scenario %q (%s)", name, h)
	}
	if len(names) == 0 {
		return fmt.Sprintf("unknown scenario %q (this project has no scenarios)", name)
	}
	return fmt.Sprintf("unknown scenario %q (scenarios: %s)", name, strings.Join(names, ", "))
}

type unknownScenario struct {
	Error    string   `json:"error"`
	Scenario string   `json:"scenario"`
	Valid    []string `json:"valid"`
	Hint     string   `json:"hint,omitempty"`
}

func checkScenario(next http.Handler, opts Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name, unknown := opts.scenarios.unknown(req)
		if !unknown {
			next.ServeHTTP(w, req)
			return
		}
		report := opts.reporter()
		names := opts.scenarios.names()
		writeJSON(w, http.StatusBadRequest, unknownScenario{Error: "unknown_scenario", Scenario: name, Valid: names, Hint: hint(name, names)})
		report(Request{Method: req.Method, Path: req.URL.Path, Status: http.StatusBadRequest, Problem: ProblemUnknownScenario})
	})
}
