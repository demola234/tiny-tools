package certs_test

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/certs"
)

var now = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func TestLoadOrCreate(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "ca")
	ca, created, err := certs.LoadOrCreate(dir, now)
	if err != nil || !created {
		t.Fatalf("first LoadOrCreate = %v, %v", created, err)
	}
	if !ca.Cert.IsCA || ca.Cert.NotAfter.Before(now.AddDate(9, 11, 0)) {
		t.Errorf("CA: IsCA %v, expires %v", ca.Cert.IsCA, ca.Cert.NotAfter)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "rootCA-key.pem"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("key file: %v, %v; want mode 0600", info, err)
		}
	}
	again, created, err := certs.LoadOrCreate(dir, now.AddDate(0, 1, 0))
	if err != nil || created || !again.Cert.Equal(ca.Cert) {
		t.Errorf("second LoadOrCreate made a new CA: created %v, %v", created, err)
	}
	if !slices.Equal(again.PEM(), ca.PEM()) {
		t.Error("PEM changed")
	}
}

func TestLoadOrCreate_CorruptFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rootCA.pem"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rootCA-key.pem"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := certs.LoadOrCreate(dir, now); err == nil {
		t.Error("a corrupt CA loaded")
	}
}

func TestLeaf(t *testing.T) {
	t.Parallel()

	ca, _, err := certs.LoadOrCreate(t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := ca.Leaf([]string{"localhost", "127.0.0.1", "192.168.1.20", "mac.local"}, now)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if !cert.NotBefore.Before(now) || cert.NotAfter.After(now.AddDate(0, 0, 398)) || cert.NotAfter.Before(now.AddDate(0, 0, 396)) {
		t.Errorf("leaf valid %v to %v", cert.NotBefore, cert.NotAfter)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.PEM())
	for _, name := range []string{"localhost", "127.0.0.1", "192.168.1.20", "mac.local"} {
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: name, Roots: roots, CurrentTime: now}); err != nil {
			t.Errorf("leaf doesn't verify for %s: %v", name, err)
		}
	}
	if _, err := cert.Verify(x509.VerifyOptions{DNSName: "example.com", Roots: roots, CurrentTime: now}); err == nil {
		t.Error("leaf verified for a name it wasn't made for")
	}
}

func TestLeaf_ServesTLS(t *testing.T) {
	t.Parallel()

	ca, _, err := certs.LoadOrCreate(t.TempDir(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := ca.Leaf([]string{"127.0.0.1"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "secure") }))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	defer srv.Close()
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.PEM())
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if body, _ := io.ReadAll(res.Body); string(body) != "secure" {
		t.Errorf("body %q", body)
	}
}

func TestNames(t *testing.T) {
	t.Parallel()

	names := certs.Names([]net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.1.20"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
	}, "dev.example")
	want := []string{"localhost", "127.0.0.1", "::1", "10.0.2.2", "192.168.1.20", "dev.example"}
	if !slices.Equal(names, want) {
		t.Errorf("Names = %v, want %v", names, want)
	}
}
