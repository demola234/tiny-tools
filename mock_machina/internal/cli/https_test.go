package cli_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/certs"
	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

var httpsURL = regexp.MustCompile(`https://\S+`)

func trusting(t *testing.T, caPEM []byte) *http.Client {
	t.Helper()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("bad CA PEM")
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
}

func getWith(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

func TestStart_HTTPSWithTheLocalCA(t *testing.T) {
	caDir := filepath.Join(t.TempDir(), "ca")
	t.Setenv("MOCKMACHINA_CA_DIR", caDir)

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: GET /users\n  states:\n    ok: { body: { users: [] } }\n")
	lines, stop := startProject(t, dir, "--https")
	defer stop()
	first := nextLine(t, lines)
	base := httpsURL.FindString(first)
	if !strings.HasPrefix(first, "serving 1 route from ") || base == "" {
		t.Fatalf("first line %q", first)
	}
	if got := nextLine(t, lines); got != "created a local certificate authority in "+caDir+"; run mockmachina cert --install to trust it" {
		t.Errorf("second line %q", got)
	}
	caPEM, err := os.ReadFile(filepath.Join(caDir, "rootCA.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if code, body := getWith(t, trusting(t, caPEM), base+"/users"); code != 200 || body != `{"users":[]}` {
		t.Errorf("GET /users over HTTPS = %d %q", code, body)
	}
}

func TestStart_HTTPSWithOwnCertificate(t *testing.T) {
	t.Parallel()

	ca, _, err := certs.LoadOrCreate(t.TempDir(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := ca.Leaf([]string{"127.0.0.1"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(leaf.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	files := t.TempDir()
	certPath, keyPath := filepath.Join(files, "cert.pem"), filepath.Join(files, "key.pem")
	writeFile(t, certPath, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Certificate[0]})))
	writeFile(t, keyPath, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})))

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "users.yaml"), "list:\n  route: GET /users\n  states:\n    ok: {}\n")
	lines, stop := startProject(t, dir, "--tls-cert", certPath, "--tls-key", keyPath)
	defer stop()
	base := httpsURL.FindString(nextLine(t, lines))
	if code, _ := getWith(t, trusting(t, ca.PEM()), base+"/users"); code != 200 {
		t.Errorf("GET /users = %d", code)
	}
}

func TestStart_TLSFlagsGoTogether(t *testing.T) {
	t.Parallel()

	code, _, stderr := runCLI(t, "start", "--dir", t.TempDir(), "--tls-cert", "cert.pem")
	if code != cli.ExitUsage || !strings.Contains(stderr, "--tls-key") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestCert_Prints(t *testing.T) {
	caDir := filepath.Join(t.TempDir(), "ca")
	t.Setenv("MOCKMACHINA_CA_DIR", caDir)

	code, stdout, stderr := runCLI(t, "cert")
	if code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{
		"local certificate authority: " + filepath.Join(caDir, "rootCA.pem"),
		"xcrun simctl keychain booted add-root-cert " + filepath.Join(caDir, "rootCA.pem"),
		"Android",
		"mockmachina cert --install",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
	code, stdout, _ = runCLI(t, "cert", "--pem")
	if code != cli.ExitOK || !strings.HasPrefix(stdout, "-----BEGIN CERTIFICATE-----") {
		t.Errorf("--pem: exit %d, %.40q", code, stdout)
	}
}
