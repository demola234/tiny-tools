package server_test

import (
	"net/http"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

const appOrigin = "http://localhost:5173"

func TestCORS_ReflectsTheOrigin(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"/users", "/nowhere"} {
		rec := do(t, newHandler(t), http.MethodGet, target, map[string]string{"Origin": appOrigin})
		want := map[string]string{
			"Access-Control-Allow-Origin":      appOrigin,
			"Access-Control-Allow-Credentials": "true",
			"Access-Control-Expose-Headers":    "X-Mock-State, X-Mock-Route",
			"Vary":                             "Origin",
		}
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("GET %s: %s = %q, want %q", target, k, got, v)
			}
		}
	}
}

func TestCORS_NoOriginNoHeaders(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodGet, "/users", nil)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q without an Origin, want none", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Parallel()

	var reported []server.Request
	h, err := server.New(usersProject(), server.Options{Report: func(r server.Request) { reported = append(reported, r) }})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodOptions, "/users", map[string]string{
		"Origin":                         appOrigin,
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "authorization, content-type",
	})

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	want := map[string]string{
		"Access-Control-Allow-Origin":      appOrigin,
		"Access-Control-Allow-Methods":     "POST",
		"Access-Control-Allow-Headers":     "authorization, content-type",
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Max-Age":           "600",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if len(reported) != 0 {
		t.Errorf("preflight was reported as a request: %+v", reported)
	}
}

func TestCORS_PlainOptionsIsNotAPreflight(t *testing.T) {
	t.Parallel()

	rec := do(t, newHandler(t), http.MethodOptions, "/users", map[string]string{"Origin": appOrigin})
	if rec.Code != http.StatusNotFound {
		t.Errorf("OPTIONS without Access-Control-Request-Method: status = %d, want 404 from the routes", rec.Code)
	}
}

func TestCORS_Disabled(t *testing.T) {
	t.Parallel()

	h, err := server.New(usersProject(), server.Options{DisableCORS: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodGet, "/users", map[string]string{"Origin": appOrigin})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q with CORS disabled, want none", got)
	}
	pre := do(t, h, http.MethodOptions, "/users", map[string]string{"Origin": appOrigin, "Access-Control-Request-Method": "GET"})
	if pre.Code != http.StatusNotFound {
		t.Errorf("preflight with CORS disabled: status = %d, want 404", pre.Code)
	}
}
