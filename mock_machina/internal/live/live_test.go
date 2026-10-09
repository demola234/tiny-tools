package live_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/live"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func contract(t *testing.T, routes string) *model.Project {
	t.Helper()
	p, probs, err := config.LoadFS(testkit.MapFS(t, map[string]string{"routes/api.yaml": routes}))
	if err != nil || len(probs) > 0 {
		t.Fatalf("contract: %v %v", probs, err)
	}
	return p
}

func backend(t *testing.T, handler http.HandlerFunc) *url.URL {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func reply(w http.ResponseWriter, status int, contentType, body string) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func texts(changes []diff.Change) []string {
	out := make([]string, len(changes))
	for i, c := range changes {
		out[i] = c.Severity.String() + " " + c.Text()
	}
	return out
}

func TestCheck(t *testing.T) {
	t.Parallel()

	p := contract(t, `
		list:
		  route: GET /users
		  states:
		    success:
		      body: { users: [{ id: u_1, name: Ada }], nextPage: 2 }
		get:
		  route: GET /users/{id}
		  examples: { id: "42" }
		  states:
		    found:
		      body: { id: u_1 }
		    not_found:
		      status: 404
		      body: { error: user not found }
		health:
		  route: GET /health
		  states:
		    up: {}
		page:
		  route: GET /page
		  states:
		    ok:
		      headers: { Cache-Control: no-store }
		      body: { title: Home }
		broken:
		  route: GET /broken
		  states:
		    ok:
		      body: { a: 1 }
		create:
		  route: POST /users
		  states:
		    created:
		      status: 201
		orders:
		  route: GET /users/{id}/orders
		  states:
		    ok: {}
	`)
	base := backend(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users":
			reply(w, 200, "application/json", `{"users":[{"id":7}],"extra":true}`)
		case "/users/42":
			reply(w, 404, "application/json", `{"error":"nope"}`)
		case "/health":
			reply(w, 503, "text/plain", "down")
		case "/page":
			reply(w, 200, "text/html", "<html></html>")
		case "/broken":
			reply(w, 200, "application/json", "{not json")
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	got := texts(live.Check(t.Context(), p, live.Options{BaseURL: base}))
	want := []string{
		`breaking api.broken: state "ok": live response isn't valid JSON`,
		`breaking api.health: live API returned 503; no state returns it (states return 200)`,
		`breaking api.list: state "success": field "nextPage" is missing live`,
		`breaking api.list: state "success": field "users[].id" is a number live, a string in the contract`,
		`breaking api.list: state "success": field "users[].name" is missing live`,
		`breaking api.page: state "ok" returns application/json, live API returned text/html`,
		`warning api.page: state "ok" sets header Cache-Control, live API doesn't`,
		`info api.create: not checked: POST isn't sent to a live API without --include-writes`,
		`info api.list: state "success": live response has extra field "extra"`,
		`info api.orders: not checked: no example value for {id} (add examples: { id: ... })`,
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("differences (-want +got):\n%s", d)
	}
}

func TestCheck_WritesHeadersAndParams(t *testing.T) {
	t.Parallel()

	p := contract(t, `
		create:
		  route: POST /users
		  states:
		    created:
		      status: 201
		get:
		  route: GET /users/{id}
		  examples: { id: "42" }
		  states:
		    found: {}
	`)
	var (
		mu   sync.Mutex
		seen []string
	)
	base := backend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			reply(w, 401, "", "")
			return
		}
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Method == http.MethodPost {
			reply(w, 201, "", "")
			return
		}
		reply(w, 200, "", "")
	})
	base.Path = "/api/v1"

	changes := live.Check(t.Context(), p, live.Options{
		BaseURL:       base,
		Headers:       http.Header{"Authorization": {"Bearer secret"}},
		Params:        map[string]string{"id": "a b"},
		IncludeWrites: true,
		Concurrency:   1,
	})
	if len(changes) != 0 {
		t.Errorf("changes = %v, want none", texts(changes))
	}
	want := []string{"GET /api/v1/users/a b", "POST /api/v1/users"}
	slices.Sort(seen)
	if d := cmp.Diff(want, seen); d != "" {
		t.Errorf("requests (-want +got):\n%s", d)
	}
}

func TestCheck_Unreachable(t *testing.T) {
	t.Parallel()

	p := contract(t, "list:\n  route: GET /users\n  states:\n    ok: {}\n")
	base := backend(t, func(http.ResponseWriter, *http.Request) {})
	srv := *base
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL, _ := url.Parse(closed.URL)
	closed.Close()
	srv.Host = closedURL.Host

	got := live.Check(t.Context(), p, live.Options{BaseURL: &srv, Timeout: time.Second})
	if len(got) != 1 || got[0].Severity != diff.Breaking || !strings.HasPrefix(got[0].Message, "request failed: ") {
		t.Errorf("changes = %v, want one breaking request failure", texts(got))
	}
}

func TestCheck_ManyRoutesInParallel(t *testing.T) {
	t.Parallel()

	var routes strings.Builder
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		routes.WriteString(name + ":\n  route: GET /" + name + "\n  states:\n    ok:\n      body: { v: 1 }\n")
	}
	p := contract(t, routes.String())
	base := backend(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, 200, "application/json", `{"v":"x"}`)
	})
	got := live.Check(t.Context(), p, live.Options{BaseURL: base, Concurrency: 4})
	if len(got) != 8 {
		t.Errorf("got %d changes, want one per route: %v", len(got), texts(got))
	}
}

func TestCheck_CRUDRoutesAreNotChecked(t *testing.T) {
	t.Parallel()

	called := false
	target := backend(t, func(w http.ResponseWriter, _ *http.Request) { called = true })
	p := contract(t, "items:\n  route: CRUD /items\n  states:\n    ok: {}\n")
	got := texts(live.Check(t.Context(), p, live.Options{BaseURL: target, IncludeWrites: true}))
	want := []string{"info api.items: not checked: a CRUD route is a whole simulated collection, not one request"}
	if diff := cmp.Diff(want, got); diff != "" || called {
		t.Errorf("changes (-want +got):\n%s\ncalled the API: %v", diff, called)
	}
}
