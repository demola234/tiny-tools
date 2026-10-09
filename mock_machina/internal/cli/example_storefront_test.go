package cli_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func storefront(t *testing.T) string {
	t.Helper()
	src := filepath.Join(filepath.Dir(testkit.Path(t)), "examples", "storefront", ".mockmachina")
	dst := filepath.Join(t.TempDir(), ".mockmachina")
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestExample_StorefrontLintsClean(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	code := cli.Run(t.Context(), cli.Env{Args: []string{"lint", "--strict", "--dir", storefront(t)}, Stdout: &out, Stderr: &out, Info: testInfo})
	if code != cli.ExitOK || strings.TrimSpace(out.String()) != "no problems" {
		t.Errorf("exit %d:\n%s", code, out.String())
	}
}

type walk struct {
	t          *testing.T
	base       string
	transcript strings.Builder
}

func (w *walk) step(method, path, body, header string, wholeBody bool) {
	w.t.Helper()
	req, err := http.NewRequestWithContext(w.t.Context(), method, w.base+path, strings.NewReader(body))
	if err != nil {
		w.t.Fatal(err)
	}
	if name, value, ok := strings.Cut(header, ": "); ok {
		req.Header.Set(name, value)
	}
	line := strings.TrimSpace(strings.Join([]string{method, path, header, body}, " "))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(&w.transcript, "%s\n  → connection dropped\n", line)
		return
	}
	data, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	got := fmt.Sprintf("%d %s", res.StatusCode, res.Header.Get("X-Mock-State"))
	if wholeBody {
		got += " " + strings.TrimSpace(string(data))
	}
	fmt.Fprintf(&w.transcript, "%s\n  → %s\n", line, strings.TrimSpace(got))
}

func TestExample_StorefrontWalkthrough(t *testing.T) {
	t.Parallel()

	lines, stop := startProject(t, storefront(t))
	defer stop()
	w := &walk{t: t, base: servingURL.FindString(nextLine(t, lines))}
	go func() {
		for range lines {
		}
	}()
	const auth = "Authorization: Bearer t_1"

	w.step("GET", "/health", "", "", true)
	w.step("GET", "/health", "", "", true)

	w.step("GET", "/me", "", "", true)
	w.step("POST", "/session", `{"email":"ada@example.com","password":"nope"}`, "", true)
	w.step("POST", "/session", `{"email":"ada@","password":"secret"}`, "", true)
	w.step("POST", "/session", `{"email":"ada@example.com","password":"secret"}`, "", true)
	w.step("GET", "/me", "", "", true)

	w.step("GET", "/products", "", "", true)
	w.step("GET", "/products?page=3", "", "", true)
	w.step("GET", "/products?page=two", "", "", true)
	w.step("GET", "/products/p_404", "", "", true)
	w.step("GET", "/products/p_zobo", "", "", true)
	w.step("GET", "/products/p_zobo?__state=out_of_stock", "", "", true)
	w.step("GET", "/deals/today", "", "", true)
	w.step("GET", "/deals/today", "", "", true)

	w.step("GET", "/cart/items", "", "", true)
	w.step("POST", "/cart/items", `{"sku":"p_garri","qty":1}`, "", false)
	w.step("PATCH", "/cart/items/c_1", `{"qty":3}`, "", true)
	w.step("DELETE", "/cart/items/c_2", "", "", true)
	w.step("GET", "/cart/items?__state=down", "", "", true)

	order := `{"items":[{"sku":"p_zobo","qty":3}]}`
	w.step("POST", "/checkout", order, "", true)
	w.step("POST", "/checkout", order, "", true)
	w.step("POST", "/checkout", order, "", false)
	w.step("POST", "/checkout", `{"items":[]}`, "", true)
	w.step("POST", "/checkout", "", "X-Mock-State: empty_cart", true)

	w.step("GET", "/orders", "", "", true)
	w.step("GET", "/orders", "", auth, true)
	w.step("GET", "/orders/o_1001", "", auth, true)
	w.step("GET", "/orders/o_404", "", auth, true)
	for range 4 {
		w.step("GET", "/orders/o_1001/tracking", "", auth, false)
	}

	w.step("DELETE", "/session", "", "", true)
	w.step("DELETE", "/session", "", "", true)
	w.step("GET", "/me", "", "", true)

	testkit.Golden(t, []byte(w.transcript.String()), "examples/storefront.txt")
}
