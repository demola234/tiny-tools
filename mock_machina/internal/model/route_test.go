package model_test

import (
	"slices"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func TestRoute_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		route model.Route
		want  string
	}{
		{"plain path", model.Route{Method: model.MethodGet, Path: "/users"}, "GET /users"},
		{"one parameter", model.Route{Method: model.MethodGet, Path: "/users/{id}"}, "GET /users/{}"},
		{
			"parameter names don't matter",
			model.Route{Method: model.MethodPost, Path: "/users/{userId}/orders/{id}"},
			"POST /users/{}/orders/{}",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.route.Key(); got != tc.want {
				t.Errorf("Key() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMethod_Valid(t *testing.T) {
	t.Parallel()

	for _, m := range []model.Method{
		model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
		model.MethodDelete, model.MethodHead, model.MethodOptions,
	} {
		if !m.Valid() {
			t.Errorf("%q.Valid() = false, want true", m)
		}
	}
	for _, m := range []model.Method{"", "get", "Get", "TRACE", "CONNECT", "CRUD"} {
		if m.Valid() {
			t.Errorf("%q.Valid() = true, want false", m)
		}
	}
}

func TestMethods(t *testing.T) {
	t.Parallel()

	want := []model.Method{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	got := model.Methods()
	if !slices.Equal(got, want) {
		t.Errorf("Methods() = %v, want %v", got, want)
	}
	got[0] = "changed"
	if model.Methods()[0] != model.MethodGet {
		t.Error("changing the slice from Methods() changed the package's list")
	}
}
