package control

import (
	"sync"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

const listenerBuffer = 64

type event struct {
	Time       time.Time `json:"time"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status,omitempty"`
	GRPCCode   string    `json:"grpc_code,omitempty"`
	Route      string    `json:"route,omitempty"`
	State      string    `json:"state,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Rule       int       `json:"rule,omitempty"`
	Problem    string    `json:"problem,omitempty"`
	Suggestion string    `json:"suggestion,omitempty"`
	Fault      string    `json:"fault,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	Proxied    bool      `json:"proxied,omitempty"`
	Webhook    bool      `json:"webhook,omitempty"`
	DurationMS int64     `json:"duration_ms"`
}

type message struct {
	name string
	data any
}

type hub struct {
	clock     clock.Clock
	mu        sync.Mutex
	listeners map[chan message]struct{}
	closed    bool
	history   [historySize]event
	next      int
	kept      int
	total     int
	errs      int
}

func newHub(c clock.Clock) *hub {
	return &hub{clock: c, listeners: map[chan message]struct{}{}}
}

func (h *hub) subscribe() (<-chan message, func()) {
	ch := make(chan message, listenerBuffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.listeners[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.listeners[ch]; ok {
			delete(h.listeners, ch)
			close(ch)
		}
	}
}

func (h *hub) publish(r server.Request) {
	e := event{
		Time: h.clock.Now(), Method: r.Method, Path: r.Path, Status: r.Status, GRPCCode: r.GRPCCode,
		Route: r.Route, State: r.State, Reason: string(r.Reason), Rule: r.Rule, Problem: string(r.Problem),
		Suggestion: r.Suggestion, Fault: string(r.Fault), Detail: r.Detail, Proxied: r.Proxied, Webhook: r.Webhook,
		DurationMS: r.Duration.Milliseconds(),
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history[h.next] = e
	h.next = (h.next + 1) % historySize
	h.kept = min(h.kept+1, historySize)
	h.total++
	if e.failed() {
		h.errs++
	}
	h.send(message{name: "request", data: e})
}

func (e event) failed() bool {
	return e.Status >= 400 || e.GRPCCode != "" && e.GRPCCode != "OK" || e.Fault != "" || e.Problem != ""
}

func (h *hub) reload() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.send(message{name: "reload", data: struct{}{}})
}

func (h *hub) send(m message) {
	for ch := range h.listeners {
		select {
		case ch <- m:
		default:
		}
	}
}

func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.listeners {
		delete(h.listeners, ch)
		close(ch)
	}
}

const historySize = 500

func (h *hub) recent() []event {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]event, h.kept)
	for i := range out {
		out[i] = h.history[(h.next-1-i+historySize)%historySize]
	}
	return out
}

func (h *hub) counts() (total, errs int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.total, h.errs
}

func (h *hub) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next, h.kept, h.total, h.errs = 0, 0, 0, 0
}
