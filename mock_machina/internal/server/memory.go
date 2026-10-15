package server

import (
	"maps"
	"sync"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type Memory struct {
	mu          sync.Mutex
	calls       map[string]int
	vars        map[string]any
	collections map[string]*collection
	pins        map[string]string
	served      map[servedState]struct{}
	applied     string
}

type servedState struct {
	route, state string
}

func NewMemory() *Memory {
	return &Memory{
		calls: map[string]int{}, vars: map[string]any{}, collections: map[string]*collection{},
		pins: map[string]string{}, served: map[servedState]struct{}{},
	}
}

func (m *Memory) call(route string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[route]++
	return m.calls[route]
}

func (m *Memory) variables() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.vars)
}

func (m *Memory) assign(name string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if value == nil {
		delete(m.vars, name)
		return
	}
	m.vars[name] = value
}

func (m *Memory) Pin(route, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pins[route] = state
}

func (m *Memory) Unpin(route string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pins, route)
}

func (m *Memory) Pinned(route string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.pins[route]
	return state, ok
}

func (m *Memory) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.calls)
	clear(m.vars)
	clear(m.collections)
	clear(m.pins)
	clear(m.served)
	m.applied = ""
}

func (m *Memory) serve(route, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.served[servedState{route, state}] = struct{}{}
}

func (m *Memory) Served(route, state string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.served[servedState{route, state}]
	return ok
}

func (m *Memory) Apply(name string, states []model.ScenarioState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.pins)
	for _, st := range states {
		m.pins[st.Route] = st.State
	}
	m.applied = name
}

func (m *Memory) Applied() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applied
}
