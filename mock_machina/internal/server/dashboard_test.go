package server_test

import (
	"net/http"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

const dashboard = "/__mockmachina/ui/"

var browser = map[string]string{"Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document", "Accept": "text/html"}

func rootProject(withRoot bool) *model.Project {
	p := coverageProject()
	if withRoot {
		p.Routes = append(p.Routes, &model.Route{
			ID: "api.root", Method: model.MethodGet, Path: "/", Active: "ok",
			States: model.States{{Name: "ok", State: &model.State{Body: jsonBody(`{"api":"v1"}`)}}},
		})
	}
	return p
}

func TestRoot_SendsBrowsersToTheDashboard(t *testing.T) {
	t.Parallel()

	var reported []server.Request
	h, err := server.New(rootProject(false), server.Options{Dashboard: dashboard, Report: func(r server.Request) { reported = append(reported, r) }})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodGet, "/", browser)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != dashboard {
		t.Errorf("browser at / = %d, Location %q; want 302 to the dashboard", rec.Code, rec.Header().Get("Location"))
	}
	if len(reported) != 0 {
		t.Errorf("the redirect was reported as traffic: %+v", reported)
	}

	tests := []struct {
		name   string
		target string
		header map[string]string
	}{
		{"curl at /", "/", nil},
		{"an app's fetch at /", "/", map[string]string{"Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}},
		{"a browser at another path", "/nowhere", browser},
	}
	for _, tt := range tests {
		if rec := do(t, h, http.MethodGet, tt.target, tt.header); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want the mock's 404", tt.name, rec.Code)
		}
	}
	if rec := do(t, h, http.MethodPost, "/", browser); rec.Code != http.StatusNotFound {
		t.Errorf("POST / from a browser = %d, want 404", rec.Code)
	}
}

func TestRoot_ARouteOrAProxyWins(t *testing.T) {
	t.Parallel()

	h, err := server.New(rootProject(true), server.Options{Dashboard: dashboard})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodGet, "/", browser); rec.Code != http.StatusOK || rec.Body.String() != `{"api":"v1"}` {
		t.Errorf("a route at / = %d %q, want the route's answer", rec.Code, rec.Body)
	}

	target, seen := backend(t)
	h, err = server.New(rootProject(false), server.Options{Dashboard: dashboard, Proxy: target})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodGet, "/", browser); rec.Code != http.StatusAccepted || len(seen.requests) != 1 {
		t.Errorf("with a proxy, / = %d and the backend saw %v; want it proxied", rec.Code, seen.requests)
	}

	h, err = server.New(rootProject(false), server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodGet, "/", browser); rec.Code != http.StatusNotFound {
		t.Errorf("with no dashboard, / = %d, want 404", rec.Code)
	}
}
