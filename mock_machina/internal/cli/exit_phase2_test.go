package cli_test

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func contractFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExitPhase2_RoundTrip(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	first := filepath.Join(work, "first", ".mockmachina")
	second := filepath.Join(work, "second", ".mockmachina")
	spec := testkit.Path(t, "specs", "swagger-petstore-3.0.yaml")
	if code, _, stderr := runCLI(t, "import", spec, "--dir", first); code != cli.ExitOK {
		t.Fatalf("import: %s", stderr)
	}
	if code, stdout, _ := runCLI(t, "lint", "--dir", first); code != cli.ExitOK {
		t.Fatalf("lint after import failed:\n%s", stdout)
	}
	exported := filepath.Join(work, "openapi.yaml")
	if code, _, stderr := runCLI(t, "export", "--dir", first, "-o", exported); code != cli.ExitOK {
		t.Fatalf("export: %s", stderr)
	}
	if code, _, stderr := runCLI(t, "import", exported, "--dir", second); code != cli.ExitOK {
		t.Fatalf("second import: %s", stderr)
	}
	if diff := cmp.Diff(contractFiles(t, first), contractFiles(t, second)); diff != "" {
		t.Errorf("import → export → import changed the contract (-first +second):\n%s", diff)
	}
}

func TestExitPhase2_LintType(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "schemas", "users.yaml"), "User:\n  type: object\n  properties:\n    age: { type: integer }\n")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "get:\n  route: GET /users/{id}\n  responses: { 200: User }\n  states:\n    found:\n      body: { age: forty }\n")
	code, stdout, _ := runCLI(t, "lint", "--dir", dir)
	want := `routes/users.yaml:5: state "found" doesn't match User: age: should be an integer, but is a string`
	if code != cli.ExitFailure || !strings.Contains(stdout, want) {
		t.Errorf("exit %d, stdout:\n%s\nwant a line %q", code, stdout, want)
	}
}

func TestExitPhase2_RequestValidation(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "session.yaml"), `create:
  route: POST /session
  request:
    body: { type: object, required: [email, password], properties: { email: { type: string, format: email } } }
  states:
    ok: { status: 201 }
`)
	lines, stop := startProject(t, dir)
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/session", strings.NewReader(`{"email":"ada@example.com"}`))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	want := `{"error":"invalid_request","route":"session.create","problems":[{"at":"body","message":"missing required field \"password\""}]}`
	if res.StatusCode != http.StatusBadRequest || strings.TrimSpace(string(body)) != want {
		t.Errorf("= %d %s\nwant 400 %s", res.StatusCode, body, want)
	}
}
