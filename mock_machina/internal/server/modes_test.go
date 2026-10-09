package server_test

import (
	"slices"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func modeProject(mode model.Mode) *model.Project {
	st := func(code int) *model.State { return &model.State{Status: code} }
	return &model.Project{Routes: []*model.Route{{
		ID: "pay.create", Method: model.MethodPost, Path: "/pay", Active: "busy", Mode: mode,
		States: model.States{{Name: "busy", State: st(503)}, {Name: "slow", State: st(504)}, {Name: "ok", State: st(201)}},
	}}}
}

func calls(t *testing.T, p *model.Project, opts server.Options, n int) ([]int, []server.Request) {
	t.Helper()
	var got []server.Request
	opts.Report = func(r server.Request) { got = append(got, r) }
	h, err := server.New(p, opts)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]int, n)
	for i := range codes {
		codes[i] = send(t, h, "POST", "/pay", "", nil)
	}
	return codes, got
}

func TestModes_Sequential(t *testing.T) {
	t.Parallel()

	codes, reports := calls(t, modeProject(model.ModeSequential), server.Options{}, 5)
	if want := []int{503, 504, 201, 201, 201}; !slices.Equal(codes, want) {
		t.Errorf("statuses %v, want %v", codes, want)
	}
	if reports[0].Reason != server.ReasonSequential {
		t.Errorf("reason %q, want sequential", reports[0].Reason)
	}
}

func TestModes_SequentialStillObeysExplicitStates(t *testing.T) {
	t.Parallel()

	h, err := server.New(modeProject(model.ModeSequential), server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if code := send(t, h, "POST", "/pay", "", map[string]string{"X-Mock-State": "ok"}); code != 201 {
		t.Errorf("X-Mock-State: ok = %d, want 201", code)
	}
}

func TestModes_RandomIsSeeded(t *testing.T) {
	t.Parallel()

	a, reports := calls(t, modeProject(model.ModeRandom), server.Options{Seed: seed.New(42)}, 200)
	b, _ := calls(t, modeProject(model.ModeRandom), server.Options{Seed: seed.New(42)}, 200)
	c, _ := calls(t, modeProject(model.ModeRandom), server.Options{Seed: seed.New(43)}, 200)
	if !slices.Equal(a, b) {
		t.Error("the same seed gave different states")
	}
	if slices.Equal(a, c) {
		t.Error("another seed gave the same states")
	}
	for _, code := range []int{503, 504, 201} {
		if !slices.Contains(a, code) {
			t.Errorf("200 random calls never returned %d", code)
		}
	}
	if reports[0].Reason != server.ReasonRandom {
		t.Errorf("reason %q, want random", reports[0].Reason)
	}
}
