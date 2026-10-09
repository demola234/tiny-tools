package config_test

import (
	"errors"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

func TestFindRoute(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/users.yaml":  "list:\n  route: GET /users\n  states:\n    ok: {}\nget:\n  route: GET /users/{id}\n  states:\n    ok: {}\n",
		"routes/health.yaml": "get:\n  route: GET /health\n  states:\n    ok: {}\n",
	})

	r, err := config.FindRoute(p, "users.get")
	if err != nil || r.ID != "users.get" {
		t.Errorf("FindRoute(users.get) = %v, %v; want the route", r, err)
	}

	tests := []struct{ id, want string }{
		{"users.lst", `no route "users.lst" (did you mean "users.list"?)`},
		{"orders", `no route "orders" (routes: health.get, users.get, users.list)`},
	}
	for _, tc := range tests {
		_, err := config.FindRoute(p, tc.id)
		if !errors.Is(err, config.ErrUnknownRoute) || err.Error() != tc.want {
			t.Errorf("FindRoute(%s) error = %v, want %q", tc.id, err, tc.want)
		}
	}
}
