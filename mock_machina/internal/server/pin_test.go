package server_test

import (
	"slices"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func TestPin_WinsOverTheMode(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	mem.Pin("pay.create", "ok")
	codes, reports := calls(t, modeProject(model.ModeSequential), server.Options{Memory: mem}, 3)
	if want := []int{201, 201, 201}; !slices.Equal(codes, want) {
		t.Errorf("statuses %v, want %v", codes, want)
	}
	if reports[0].Reason != server.ReasonControl {
		t.Errorf("reason %q, want control", reports[0].Reason)
	}
	if got, ok := mem.Pinned("pay.create"); !ok || got != "ok" {
		t.Errorf("Pinned = %q, %v", got, ok)
	}
}

func TestPin_HeaderStillWins(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	mem.Pin("pay.create", "ok")
	h, err := server.New(modeProject(model.ModeActive), server.Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	if code := send(t, h, "POST", "/pay", "", map[string]string{"X-Mock-State": "slow"}); code != 504 {
		t.Errorf("X-Mock-State: slow = %d, want 504", code)
	}
	mem.Unpin("pay.create")
	if code := send(t, h, "POST", "/pay", "", nil); code != 503 {
		t.Errorf("after Unpin = %d, want the active 503", code)
	}
}

func TestReset_StartsAgain(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	h, err := server.New(modeProject(model.ModeSequential), server.Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	send(t, h, "POST", "/pay", "", nil)
	send(t, h, "POST", "/pay", "", nil)
	mem.Pin("pay.create", "ok")
	mem.Reset()
	if _, ok := mem.Pinned("pay.create"); ok {
		t.Error("Reset kept the pin")
	}
	if code := send(t, h, "POST", "/pay", "", nil); code != 503 {
		t.Errorf("first call after Reset = %d, want 503", code)
	}

	cart, err := server.New(crudProject(cartData), server.Options{Memory: mem, Seed: seed.New(1)})
	if err != nil {
		t.Fatal(err)
	}
	runSteps(t, cart, []crudStep{{method: "DELETE", target: "/cart/items/i_1", code: 204}})
	mem.Reset()
	runSteps(t, cart, []crudStep{{method: "GET", target: "/cart/items/i_1", code: 200, want: `{"id":"i_1","name":"Tea","qty":1}`}})
}

func TestApply_ReplacesPins(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	mem.Pin("orders.list", "down")
	mem.Apply("new-user", []model.ScenarioState{{Route: "users.list", State: "empty"}})
	if _, ok := mem.Pinned("orders.list"); ok {
		t.Error("an old pin survived Apply")
	}
	if got, ok := mem.Pinned("users.list"); !ok || got != "empty" {
		t.Errorf("Pinned(users.list) = %q, %v; want empty", got, ok)
	}
	if got := mem.Applied(); got != "new-user" {
		t.Errorf("Applied = %q, want new-user", got)
	}
	mem.Reset()
	if got := mem.Applied(); got != "" {
		t.Errorf("after reset, Applied = %q", got)
	}
}
