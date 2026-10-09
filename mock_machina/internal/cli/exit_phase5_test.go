package cli_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const shopFlow = `create:
  route: POST /session
  rules:
    - when: { body.password: { ne: secret } }
      state: wrong_password
  states:
    ok:
      status: 201
      set: { signed_in: true, email: "{{ body.email }}" }
      body: { token: "{{ uuid }}" }
    wrong_password: { status: 401, body: { error: wrong password } }
me:
  route: GET /me
  rules:
    - when: { var.signed_in: { exists: false } }
      state: signed_out
  states:
    ok: { body: { email: "{{ var.email }}", name: "{{ fake.person.name }}", city: "{{ fake.city }}" } }
    signed_out: { status: 401 }
`

const cartFlow = `items:
  route: CRUD /cart/items
  states:
    ok: {}
    down: { status: 503, body: { error: unavailable } }
flaky:
  route: POST /checkout
  states:
    reset: { fault: reset }
    cut: { status: 200, fault: truncated, body: { order: o_1234567890 } }
`

func TestExitPhase5_Flow(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "session.yaml"), shopFlow)
	writeFile(t, filepath.Join(dir, "routes", "cart.yaml"), cartFlow)
	writeFile(t, filepath.Join(dir, "data", "items.json"), `[{"id":"i_1","sku":"tea","qty":1}]`)
	writeFile(t, filepath.Join(dir, "config.yaml"), "locale: en_NG\n")
	lines, stop := startProject(t, dir, "--seed", "42")
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))

	var transcript strings.Builder
	step := func(method, path, body string) {
		req, err := http.NewRequestWithContext(t.Context(), method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		fmt.Fprintf(&transcript, "%s\n  → %s\n", strings.TrimSpace(method+" "+path+" "+body), strings.TrimSpace(fmt.Sprintf("%d %s", res.StatusCode, data)))
	}
	step("GET", "/me", "")
	step("POST", "/session", `{"email":"ada@example.com","password":"nope"}`)
	step("POST", "/session", `{"email":"ada@example.com","password":"secret"}`)
	step("GET", "/me", "")
	step("GET", "/cart/items", "")
	step("POST", "/cart/items", `{"sku":"bread","qty":2}`)
	step("PATCH", "/cart/items/i_1", `{"qty":3}`)
	step("GET", "/cart/items", "")
	step("GET", "/cart/items?__state=down", "")
	testkit.Golden(t, []byte(transcript.String()), "exit/phase5_flow.txt")
}

func TestExitPhase5_Faults(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "cart.yaml"), cartFlow)
	lines, stop := startProject(t, dir)
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/checkout", nil)
	res, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = res.Body.Close()
		t.Error("reset: the request succeeded")
	} else if !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, io.EOF) {
		t.Logf("reset gave %v", err)
	}

	req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/checkout?__state=cut", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if _, err := io.ReadAll(res.Body); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated: read error %v, want unexpected EOF", err)
	}
}
