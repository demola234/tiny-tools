package config_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const graphqlRoutes = `user:
  route: GRAPHQL /graphql
  operation: GetUser
  rules:
    - when: { body.variables.id: u_404 }
      state: not_found
  states:
    found:
      body: { data: { user: { id: "{{ body.variables.id }}", name: Ada } } }
    not_found:
      body: { data: { user: null }, errors: [{ message: user not found }] }
login:
  route: GRAPHQL /graphql
  operation: Login
  states:
    ok:
      body: { data: { login: { token: "{{ uuid }}" } } }
`

func TestLoadFS_GraphQL(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/api.yaml": graphqlRoutes})
	r, _ := p.Route("api.user")
	if r.Method != model.MethodGraphQL || r.Operation != "GetUser" || r.Path != "/graphql" {
		t.Errorf("route = %s %s %q", r.Method, r.Path, r.Operation)
	}
}

func TestLoadFS_GraphQLProblems(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, file, want string }{
		{"no operation", "q:\n  route: GRAPHQL /graphql\n  states:\n    s: {}\n", `routes/api.yaml:1: route "q" needs "operation": the GraphQL operation name it answers, like operation: GetUser`},
		{"bad name", "q:\n  route: GRAPHQL /graphql\n  operation: get-user\n  states:\n    s: {}\n", `routes/api.yaml:3: operation "get-user" isn't a GraphQL name; use letters, digits and underscores, like GetUser`},
		{"operation on rest", "q:\n  route: GET /users\n  operation: GetUser\n  states:\n    s: {}\n", `routes/api.yaml:3: operation only works on GRAPHQL routes`},
		{"same operation twice", "a:\n  route: GRAPHQL /graphql\n  operation: GetUser\n  states:\n    s: {}\nb:\n  route: GRAPHQL /graphql\n  operation: GetUser\n  states:\n    s: {}\n", `routes/api.yaml:6: GRAPHQL /graphql GetUser is already defined by route "api.a"`},
		{"post on the same path", "a:\n  route: GRAPHQL /graphql\n  operation: GetUser\n  states:\n    s: {}\nb:\n  route: POST /graphql\n  states:\n    s: {}\n", `routes/api.yaml:6: POST /graphql is already served by GRAPHQL route "api.a"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, map[string]string{"routes/api.yaml": tc.file})
			if got := probs.String(); got != tc.want {
				t.Errorf("problems:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
