package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const graphqlProject = `user:
  route: GRAPHQL /graphql
  operation: GetUser
  rules:
    - when: { body.variables.id: u_404 }
      state: not_found
  states:
    found:
      body: { data: { user: { id: "{{ body.variables.id }}" } } }
    not_found:
      body: { data: { user: null }, errors: [{ message: user not found }] }
login:
  route: GRAPHQL /graphql
  operation: Login
  states:
    ok:
      body: { data: { login: { ok: true } } }
`

func graphqlServer(t *testing.T) (*httptest.Server, <-chan server.Request) {
	t.Helper()
	proj, probs, err := config.LoadFS(testkit.MapFS(t, map[string]string{"routes/api.yaml": graphqlProject}))
	if err != nil || len(probs) > 0 {
		t.Fatalf("load: %v %v", probs, err)
	}
	got := make(chan server.Request, 8)
	h, err := server.New(proj, server.Options{Report: func(r server.Request) { got <- r }})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, got
}

func gql(t *testing.T, srv *httptest.Server, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(data)
}

func TestGraphQL_DispatchesByOperation(t *testing.T) {
	t.Parallel()

	srv, got := graphqlServer(t)
	tests := []struct{ name, body, want string }{
		{"operationName", `{"operationName":"GetUser","query":"query GetUser($id: ID!) { user(id: $id) { id } }","variables":{"id":"u_1"}}`, `{"data":{"user":{"id":"u_1"}}}`},
		{"name from the query text", `{"query":"query GetUser { user(id: \"x\") { id } }","variables":{"id":"u_2"}}`, `{"data":{"user":{"id":"u_2"}}}`},
		{"rule on variables", `{"operationName":"GetUser","query":"query GetUser { x }","variables":{"id":"u_404"}}`, `{"data":{"user":null},"errors":[{"message":"user not found"}]}`},
		{"mutation", `{"query":"mutation Login($e: String) { login(email: $e) { ok } }"}`, `{"data":{"login":{"ok":true}}}`},
	}
	for _, tt := range tests {
		status, body := gql(t, srv, tt.body)
		if status != http.StatusOK || body != tt.want {
			t.Errorf("%s: %d %s, want %s", tt.name, status, body, tt.want)
		}
	}
	if r := <-got; r.Method != "GRAPHQL" || r.Path != "GetUser" || r.Route != "api.user" {
		t.Errorf("report = %+v", r)
	}
}

func TestGraphQL_GetRequest(t *testing.T) {
	t.Parallel()

	srv, _ := graphqlServer(t)
	q := url.Values{"query": {"query GetUser { user { id } }"}, "variables": {`{"id":"u_9"}`}}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/graphql?"+q.Encode(), nil)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if string(data) != `{"data":{"user":{"id":"u_9"}}}` {
		t.Errorf("body = %s", data)
	}
}

func TestGraphQL_UnknownOperation(t *testing.T) {
	t.Parallel()

	srv, got := graphqlServer(t)
	status, body := gql(t, srv, `{"operationName":"GetUsr","query":"query GetUsr { x }"}`)
	if status != http.StatusBadRequest || !strings.Contains(body, `no mock for operation GetUsr (did you mean GetUser?)`) {
		t.Errorf("%d %s", status, body)
	}
	if r := <-got; r.Problem != server.ProblemNoRoute || r.Path != "GetUsr" {
		t.Errorf("report = %+v", r)
	}
}
