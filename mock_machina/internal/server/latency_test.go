package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func slowProject() *model.Project {
	return &model.Project{Routes: []*model.Route{{
		ID:     "users.list",
		Method: model.MethodGet,
		Path:   "/users",
		Active: "slow",
		States: model.States{
			{Name: "slow", State: &model.State{Latency: 800 * time.Millisecond, Body: jsonBody(`{"users":[]}`)}},
			{Name: "fast", State: &model.State{Body: jsonBody(`{"users":[]}`)}},
		},
	}}}
}

func TestServer_WaitsForTheStatesLatency(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h, err := server.New(slowProject(), server.Options{Clock: clock.Real{}})
		if err != nil {
			t.Fatal(err)
		}
		for state, want := range map[string]time.Duration{"slow": 800 * time.Millisecond, "fast": 0} {
			start := time.Now()
			rec := do(t, h, http.MethodGet, "/users", map[string]string{"X-Mock-State": state})
			if got := time.Since(start); got != want {
				t.Errorf("%s: responded after %v, want %v", state, got, want)
			}
			if rec.Body.String() != `{"users":[]}` {
				t.Errorf("%s: body = %q", state, rec.Body.String())
			}
		}
	})
}

func TestServer_ClientLeavesDuringLatency(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var got []server.Request
		h, err := server.New(slowProject(), server.Options{
			Clock:  clock.Real{},
			Report: func(r server.Request) { got = append(got, r) },
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(100*time.Millisecond, cancel)
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users", nil)
		rec := httptest.NewRecorder()
		start := time.Now()
		h.ServeHTTP(rec, req)

		if elapsed := time.Since(start); elapsed != 100*time.Millisecond {
			t.Errorf("returned after %v, want 100ms (when the client left)", elapsed)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("body = %q, want nothing written", rec.Body.String())
		}
		want := server.Request{
			Method: "GET", Path: "/users", Status: 499, Route: "users.list", State: "slow",
			Reason: server.ReasonActive, Problem: server.ProblemClientLeft,
		}
		if len(got) != 1 || got[0] != want {
			t.Errorf("reported %+v, want [%+v]", got, want)
		}
	})
}

func TestServer_Jitter(t *testing.T) {
	t.Parallel()

	waits := func(s uint64) []time.Duration {
		var got []time.Duration
		synctest.Test(t, func(t *testing.T) {
			p := slowProject()
			p.Routes[0].States[0].State.Jitter = 200 * time.Millisecond
			h, err := server.New(p, server.Options{Clock: clock.Real{}, Seed: seed.New(s)})
			if err != nil {
				t.Fatal(err)
			}
			for range 50 {
				start := time.Now()
				do(t, h, http.MethodGet, "/users", nil)
				got = append(got, time.Since(start))
			}
		})
		return got
	}
	a := waits(1)
	for _, d := range a {
		if d < 600*time.Millisecond || d > time.Second {
			t.Errorf("waited %v, want 800ms ± 200ms", d)
		}
	}
	if slices.Equal(a, slices.Repeat([]time.Duration{a[0]}, len(a))) {
		t.Error("every wait was the same")
	}
	if !slices.Equal(a, waits(1)) || slices.Equal(a, waits(2)) {
		t.Error("jitter isn't seeded")
	}
}
