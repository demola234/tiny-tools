package docs_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/docs"
)

func get(t *testing.T, h http.Handler, path string) (int, string, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, rec.Header().Get("Content-Type"), string(body)
}

func TestHandler(t *testing.T) {
	t.Parallel()

	calls := 0
	h := docs.Handler("Shop <API>", func() ([]byte, error) {
		calls++
		return []byte("openapi: 3.1.0\n"), nil
	})
	code, ctype, body := get(t, h, "/")
	if code != 200 || !strings.HasPrefix(ctype, "text/html") || !strings.Contains(body, "<title>Shop &lt;API&gt;</title>") || !strings.Contains(body, `url: "openapi.yaml"`) {
		t.Errorf("/ = %d %s\n%s", code, ctype, body)
	}
	if code, ctype, body := get(t, h, "/swagger-ui-bundle.js"); code != 200 || !strings.Contains(ctype, "javascript") || len(body) < 1_000_000 {
		t.Errorf("bundle = %d %s, %d bytes", code, ctype, len(body))
	}
	if code, ctype, _ := get(t, h, "/swagger-ui.css"); code != 200 || !strings.HasPrefix(ctype, "text/css") {
		t.Errorf("css = %d %s", code, ctype)
	}
	for range 2 {
		if code, ctype, body := get(t, h, "/openapi.yaml"); code != 200 || !strings.Contains(ctype, "yaml") || body != "openapi: 3.1.0\n" {
			t.Errorf("spec = %d %s %q", code, ctype, body)
		}
	}
	if calls != 2 {
		t.Errorf("the spec was made %d times; want it fresh on every request", calls)
	}
	if code, _, _ := get(t, h, "/nope"); code != 404 {
		t.Errorf("/nope = %d", code)
	}
}

func TestHandler_SpecError(t *testing.T) {
	t.Parallel()

	h := docs.Handler("x", func() ([]byte, error) { return nil, errors.New("routes/users.yaml:3: broken") })
	if code, _, body := get(t, h, "/openapi.yaml"); code != 500 || !strings.Contains(body, "routes/users.yaml:3: broken") {
		t.Errorf("= %d %q", code, body)
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "site")
	written, err := docs.Write(dir, "Shop", []byte("openapi: 3.1.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"LICENSE", "NOTICE", "index.html", "openapi.yaml", "swagger-ui-bundle.js", "swagger-ui.css"}
	slices.Sort(written)
	if !slices.Equal(written, want) {
		t.Errorf("written %v, want %v", written, want)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "openapi.yaml")); string(data) != "openapi: 3.1.0\n" {
		t.Errorf("openapi.yaml = %q", data)
	}
}
