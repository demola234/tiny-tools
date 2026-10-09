package model_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func TestProject_Route(t *testing.T) {
	t.Parallel()

	p := &model.Project{Routes: []*model.Route{
		{ID: "cart"}, {ID: "health.get"}, {ID: "orders.get"}, {ID: "users.list"},
	}}
	for _, id := range []string{"cart", "orders.get", "users.list"} {
		got, ok := p.Route(id)
		if !ok || got.ID != id {
			t.Errorf("Route(%q) = %v, %v; want the route, true", id, got, ok)
		}
	}
	for _, id := range []string{"a", "health", "zzz", ""} {
		if got, ok := p.Route(id); ok || got != nil {
			t.Errorf("Route(%q) = %v, %v; want nil, false", id, got, ok)
		}
	}
	if got, ok := (&model.Project{}).Route("cart"); ok || got != nil {
		t.Errorf("empty project Route(cart) = %v, %v; want nil, false", got, ok)
	}
}
