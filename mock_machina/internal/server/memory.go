package server

import (
	"maps"
	"sync"
)

type Memory struct {
	mu          sync.Mutex
	calls       map[string]int
	vars        map[string]any
	collections map[string]*collection
}

func NewMemory() *Memory {
	return &Memory{calls: map[string]int{}, vars: map[string]any{}, collections: map[string]*collection{}}
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
