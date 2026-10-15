package control_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func publishTraffic(f fixture) {
	f.api.Publish(server.Request{Method: "GET", Path: "/users", Status: 200, Route: "users.list", State: "full", Reason: server.ReasonActive, Duration: 3 * time.Millisecond})
	f.api.Publish(server.Request{Method: "GET", Path: "/nope", Status: 404, Problem: server.ProblemNoRoute, Suggestion: "GET /users"})
	f.api.Publish(server.Request{Method: "GET", Path: "/real", Status: 200, Proxied: true, Duration: 140 * time.Millisecond})
	f.api.Publish(server.Request{Method: "POST", Path: "https://hooks.example/paid", Status: 500, Route: "users.list", State: "full", Webhook: true})
}

func TestUITraffic_Page(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	f.api.SetGRPC("127.0.0.1:4002")
	publishTraffic(f)
	for target, golden := range map[string]string{
		"/__mockmachina/ui/traffic":                "ui/traffic.html",
		"/__mockmachina/ui/traffic?filter=errors":  "ui/traffic-errors.html",
		"/__mockmachina/ui/traffic?filter=proxied": "ui/traffic-proxied.html",
	} {
		rec := f.raw(t, "GET", target, "", nil)
		if rec.Code != 200 {
			t.Errorf("GET %s = %d", target, rec.Code)
		}
		testkit.Golden(t, rec.Body.Bytes(), golden)
	}
}

func uiEvents(t *testing.T, f fixture, query string) *bufio.Scanner {
	t.Helper()
	srv := httptest.NewServer(f.api)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/__mockmachina/ui/events"+query, nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); resp.StatusCode != http.StatusOK || ct != "text/event-stream" {
		t.Fatalf("ui/events = %d %q", resp.StatusCode, ct)
	}
	return bufio.NewScanner(resp.Body)
}

func TestUITraffic_Events(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "")
	lines := uiEvents(t, f, "?filter=errors")
	publishTraffic(f)
	event, data := nextEvent(lines)
	if event != "request" || !strings.HasPrefix(data, "<li") || !strings.Contains(data, "/nope") || strings.Contains(data, "\n") {
		t.Errorf("first error event %q %q, want the /nope row on one line", event, data)
	}
	f.api.Swap(&model.Project{})
	for {
		event, data = nextEvent(lines)
		if event != "request" {
			break
		}
	}
	if event != "reload" || data != "{}" {
		t.Errorf("after Swap, event %q %q, want reload", event, data)
	}
}
