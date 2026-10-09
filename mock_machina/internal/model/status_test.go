package model_test

import (
	"slices"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func TestStatus_Valid(t *testing.T) {
	t.Parallel()

	for _, s := range []model.Status{model.StatusDraft, model.StatusAgreed, model.StatusImplemented, model.StatusDeprecated} {
		if !s.Valid() {
			t.Errorf("%q.Valid() = false, want true", s)
		}
	}
	for _, s := range []model.Status{"", "Draft", "approved", "200"} {
		if s.Valid() {
			t.Errorf("%q.Valid() = true, want false", s)
		}
	}
}

func TestStatus_Effective(t *testing.T) {
	t.Parallel()

	if got := model.Status("").Effective(); got != model.StatusDraft {
		t.Errorf(`Status("").Effective() = %q, want draft`, got)
	}
	if got := model.StatusAgreed.Effective(); got != model.StatusAgreed {
		t.Errorf("StatusAgreed.Effective() = %q, want agreed", got)
	}
}

func TestStatuses(t *testing.T) {
	t.Parallel()

	want := []model.Status{"draft", "agreed", "implemented", "deprecated"}
	got := model.Statuses()
	if !slices.Equal(got, want) {
		t.Errorf("Statuses() = %v, want %v", got, want)
	}
	got[0] = "changed"
	if model.Statuses()[0] != model.StatusDraft {
		t.Error("changing the slice from Statuses() changed the package's list")
	}
}

func TestOwners_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		owners model.Owners
		want   bool
	}{
		{model.Owners{}, true},
		{model.Owners{Backend: []string{}}, true},
		{model.Owners{Backend: []string{"ademola"}}, false},
		{model.Owners{Frontend: []string{"ada"}}, false},
	}
	for _, tc := range tests {
		if got := tc.owners.IsZero(); got != tc.want {
			t.Errorf("%+v.IsZero() = %v, want %v", tc.owners, got, tc.want)
		}
	}
}
