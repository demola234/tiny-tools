package control

import (
	"fmt"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func TestHub_SlowListenerNeverBlocks(t *testing.T) {
	t.Parallel()

	h := newHub(clock.Real{})
	slow, stop := h.subscribe()
	defer stop()
	for range 1000 {
		h.publish(server.Request{Path: "/users"})
	}
	if n := len(slow); n != listenerBuffer {
		t.Errorf("buffered %d events, want %d", n, listenerBuffer)
	}
}

func TestHub_CloseEndsListeners(t *testing.T) {
	t.Parallel()

	h := newHub(clock.Real{})
	ch, stop := h.subscribe()
	h.close()
	if _, open := <-ch; open {
		t.Error("listener still open after close")
	}
	stop()
	late, _ := h.subscribe()
	if _, open := <-late; open {
		t.Error("a listener joining after close stays open")
	}
	h.publish(server.Request{})
}

func TestHub_KeepsTheNewestRequests(t *testing.T) {
	t.Parallel()

	h := newHub(clock.Real{})
	for i := range historySize + 1 {
		h.publish(server.Request{Path: fmt.Sprintf("/r%d", i)})
	}
	got := h.recent()
	if len(got) != historySize {
		t.Fatalf("kept %d requests, want %d", len(got), historySize)
	}
	if got[0].Path != fmt.Sprintf("/r%d", historySize) || got[len(got)-1].Path != "/r1" {
		t.Errorf("kept %s to %s, want /r%d to /r1", got[0].Path, got[len(got)-1].Path, historySize)
	}
	if total, _ := h.counts(); total != historySize+1 {
		t.Errorf("total %d, want %d", total, historySize+1)
	}
}

func TestHub_CountsErrors(t *testing.T) {
	t.Parallel()

	h := newHub(clock.Real{})
	for _, r := range []server.Request{
		{Status: 200},
		{Status: 101},
		{Status: 404},
		{Status: 500},
		{GRPCCode: "OK"},
		{GRPCCode: "NOT_FOUND"},
		{Status: 200, Fault: model.FaultReset},
		{Status: 499, Problem: server.ProblemClientLeft},
	} {
		h.publish(r)
	}
	if total, errs := h.counts(); total != 8 || errs != 5 {
		t.Errorf("counts %d total, %d errors; want 8, 5", total, errs)
	}
	h.clear()
	if total, errs := h.counts(); total != 0 || errs != 0 || len(h.recent()) != 0 {
		t.Errorf("after clear: %d total, %d errors, %d kept", total, errs, len(h.recent()))
	}
}
