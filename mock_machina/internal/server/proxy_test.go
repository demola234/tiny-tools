package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

type backendSeen struct {
	mu       sync.Mutex
	states   []string
	hosts    []string
	bodies   []string
	requests []string
}

func backend(t *testing.T) (*url.URL, *backendSeen) {
	t.Helper()
	seen := &backendSeen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.mu.Lock()
		seen.states = append(seen.states, r.Header.Get("X-Mock-State"))
		seen.hosts = append(seen.hosts, r.Host)
		seen.bodies = append(seen.bodies, string(body))
		seen.requests = append(seen.requests, r.Method+" "+r.URL.RequestURI())
		seen.mu.Unlock()
		w.Header().Set("Access-Control-Allow-Origin", "https://backend.example")
		w.Header().Set("X-Backend", "yes")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "backend "+r.Method+" "+r.URL.RequestURI())
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return u, seen
}

func proxyProject() *model.Project {
	p := usersProject()
	p.Routes[0].Serve = model.ServeProxy
	return p
}

func TestProxy_Routing(t *testing.T) {
	t.Parallel()

	target, seen := backend(t)
	var got []server.Request
	h, err := server.New(proxyProject(), server.Options{Proxy: target, Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method, target, state string
		wantStatus            int
		wantBody              string
	}{
		{"GET", "/users", "", 200, `{"users":[{"id":"u_1"}]}`},
		{"GET", "/orders?page=2", "", 202, "backend GET /orders?page=2"},
		{"GET", "/users/u_1", "", 202, "backend GET /users/u_1"},
		{"GET", "/users/u_1", "found", 200, `{"id":"u_1"}`},
		{"GET", "/users/u_1?__state=found", "", 200, `{"id":"u_1"}`},
	}
	for _, tc := range tests {
		header := map[string]string{}
		if tc.state != "" {
			header["X-Mock-State"] = tc.state
		}
		rec := do(t, h, tc.method, tc.target, header)
		if rec.Code != tc.wantStatus || rec.Body.String() != tc.wantBody {
			t.Errorf("%s %s (state %q) = %d %q, want %d %q", tc.method, tc.target, tc.state, rec.Code, rec.Body, tc.wantStatus, tc.wantBody)
		}
	}
	want := []server.Request{
		{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "success", Reason: server.ReasonActive},
		{Method: "GET", Path: "/orders?page=2", Status: 202, Proxied: true},
		{Method: "GET", Path: "/users/u_1", Status: 202, Route: "users.get", Proxied: true},
		{Method: "GET", Path: "/users/u_1", Status: 200, Route: "users.get", State: "found", Reason: server.ReasonHeader},
		{Method: "GET", Path: "/users/u_1", Status: 200, Route: "users.get", State: "found", Reason: server.ReasonQuery},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("reported requests (-want +got):\n%s", diff)
	}
	seen.mu.Lock()
	defer seen.mu.Unlock()
	for i, host := range seen.hosts {
		if host != target.Host {
			t.Errorf("backend request %d had Host %q, want %q", i, host, target.Host)
		}
	}
}

func TestProxy_SendsTheRequestAsIs(t *testing.T) {
	t.Parallel()

	target, seen := backend(t)
	target.Path = "/api"
	h, err := server.New(usersProject(), server.Options{Proxy: target})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/orders", strings.NewReader(`{"qty":2}`))
	req.Header.Set("X-Mock-State", "ignored")
	h.ServeHTTP(httptest.NewRecorder(), req)
	seen.mu.Lock()
	defer seen.mu.Unlock()
	if seen.requests[0] != "POST /api/orders" || seen.bodies[0] != `{"qty":2}` || seen.states[0] != "" {
		t.Errorf("backend saw %q with body %q and X-Mock-State %q", seen.requests[0], seen.bodies[0], seen.states[0])
	}
}

func TestProxy_CORS(t *testing.T) {
	t.Parallel()

	target, _ := backend(t)
	origin := map[string]string{"Origin": "http://localhost:3000"}
	for _, tc := range []struct {
		disable bool
		want    []string
	}{
		{false, []string{"http://localhost:3000"}},
		{true, []string{"https://backend.example"}},
	} {
		h, err := server.New(usersProject(), server.Options{Proxy: target, DisableCORS: tc.disable})
		if err != nil {
			t.Fatal(err)
		}
		rec := do(t, h, "GET", "/orders", origin)
		if got := rec.Header().Values("Access-Control-Allow-Origin"); !cmp.Equal(got, tc.want) {
			t.Errorf("DisableCORS %v: Access-Control-Allow-Origin = %q, want %q", tc.disable, got, tc.want)
		}
		if rec.Header().Get("X-Backend") != "yes" {
			t.Error("the backend's own headers weren't passed on")
		}
	}
}

func TestProxy_BackendDown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	target, _ := url.Parse(srv.URL)
	srv.Close()
	var got []server.Request
	h, err := server.New(usersProject(), server.Options{Proxy: target, Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, "GET", "/orders", nil)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "couldn't reach the backend at "+srv.URL) {
		t.Errorf("= %d %q", rec.Code, rec.Body)
	}
	if len(got) != 1 || got[0].Status != http.StatusBadGateway || got[0].Problem != server.ProblemBackend || !got[0].Proxied {
		t.Errorf("reported %+v", got)
	}
}

func TestProxy_ServeProxyWithoutATargetIsMocked(t *testing.T) {
	t.Parallel()

	h, err := server.New(proxyProject(), server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, "GET", "/users/u_1", nil); rec.Code != http.StatusOK || rec.Body.String() != `{"id":"u_1"}` {
		t.Errorf("= %d %q", rec.Code, rec.Body)
	}
}
