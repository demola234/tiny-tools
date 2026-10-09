package release_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/release"
)

func serveRelease(t *testing.T, corrupt bool) *httptest.Server {
	t.Helper()
	dist := t.TempDir()
	if _, err := release.Build(t.Context(), release.Options{
		Version: "v1.2.3", ModuleDir: moduleDir(t), Out: dist, Targets: []release.Target{{OS: runtime.GOOS, Arch: runtime.GOARCH}},
	}); err != nil {
		t.Fatal(err)
	}
	if corrupt {
		sums, _ := os.ReadFile(filepath.Join(dist, "checksums.txt"))
		_ = os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(strings.Repeat("0", 64)+string(sums[64:])), 0o644)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/releases", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"rodinia/v9.0.0"},{"tag_name":"mock_machina/v1.2.3"},{"tag_name":"mock_machina/v1.2.2"}]`))
	})
	mux.Handle("/download/mock_machina/v1.2.3/", http.StripPrefix("/download/mock_machina/v1.2.3/", http.FileServer(http.Dir(dist))))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func install(t *testing.T, srv *httptest.Server, env ...string) (string, string, error) {
	t.Helper()
	dest := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "sh", filepath.Join(moduleDir(t), "install.sh"))
	cmd.Env = append(os.Environ(),
		"MOCKMACHINA_API="+srv.URL+"/api/releases",
		"MOCKMACHINA_DOWNLOAD_BASE="+srv.URL+"/download",
		"MOCKMACHINA_INSTALL_DIR="+dest,
	)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	return dest, string(out), err
}

func TestInstallScript(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is for macOS and Linux")
	}

	srv := serveRelease(t, false)
	dest, out, err := install(t, srv)
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	if !strings.Contains(out, "installed mockmachina v1.2.3 to "+filepath.Join(dest, "mockmachina")) {
		t.Errorf("output %q", out)
	}
	got, err := exec.CommandContext(t.Context(), filepath.Join(dest, "mockmachina"), "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(got), "v1.2.3") {
		t.Errorf("installed binary: %s, %v", got, err)
	}

	if _, out, err := install(t, srv, "MOCKMACHINA_VERSION=v1.2.3"); err != nil {
		t.Errorf("pinned version: %v\n%s", err, out)
	}
}

func TestInstallScript_RefusesABadChecksum(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is for macOS and Linux")
	}

	dest, out, err := install(t, serveRelease(t, true))
	if err == nil || !strings.Contains(out, "checksum doesn't match") {
		t.Errorf("err %v, output %q", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "mockmachina")); statErr == nil {
		t.Error("a binary with a bad checksum was installed")
	}
}
