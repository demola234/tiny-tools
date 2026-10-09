package model_test

import (
	"slices"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func sampleStates() model.States {
	return model.States{
		{Name: "success", State: &model.State{Status: 200}},
		{Name: "empty", State: &model.State{}},
		{Name: "unauthorized", State: &model.State{Status: 401}},
	}
}

func TestStates_Get(t *testing.T) {
	t.Parallel()

	states := sampleStates()
	got, ok := states.Get("unauthorized")
	if !ok || got != states[2].State {
		t.Errorf("Get(unauthorized) = %p, %v; want %p, true", got, ok, states[2].State)
	}
	if got, ok := states.Get("missing"); ok || got != nil {
		t.Errorf("Get(missing) = %p, %v; want nil, false", got, ok)
	}
	if got, ok := model.States(nil).Get("success"); ok || got != nil {
		t.Errorf("nil States Get(success) = %p, %v; want nil, false", got, ok)
	}
}

func TestStates_Names_KeepsFileOrder(t *testing.T) {
	t.Parallel()

	want := []string{"success", "empty", "unauthorized"}
	if got := sampleStates().Names(); !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
}

func TestState_EffectiveStatus(t *testing.T) {
	t.Parallel()

	if got := (&model.State{}).EffectiveStatus(); got != 200 {
		t.Errorf("EffectiveStatus() with no status = %d, want 200", got)
	}
	if got := (&model.State{Status: 404}).EffectiveStatus(); got != 404 {
		t.Errorf("EffectiveStatus() with 404 = %d, want 404", got)
	}
}
