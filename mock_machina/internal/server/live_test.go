package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func say(s string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, s) })
}

func body(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	return rec.Body.String()
}

func TestLive_ServesTheCurrentHandler(t *testing.T) {
	t.Parallel()

	live := server.NewLive(say("first"))
	if got := body(t, live); got != "first" {
		t.Errorf("before swap = %q, want first", got)
	}
	live.Swap(say("second"))
	if got := body(t, live); got != "second" {
		t.Errorf("after swap = %q, want second", got)
	}
}

func TestLive_SwapsWhileServing(t *testing.T) {
	t.Parallel()

	live := server.NewLive(say("a"))
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 50 {
				if got := body(t, live); got != "a" && got != "b" {
					t.Errorf("body = %q, want a or b", got)
				}
			}
		})
	}
	for i := range 100 {
		if i%2 == 0 {
			live.Swap(say("b"))
		} else {
			live.Swap(say("a"))
		}
	}
	wg.Wait()
}
