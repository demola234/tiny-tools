package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/release"
	"github.com/demola234/tiny-tools/mock_machina/internal/update"
)

func releaseServer(t *testing.T) update.Source {
	t.Helper()
	target := release.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: target.Exe(), Mode: 0o755, Size: 3})
	_, _ = tw.Write([]byte("new"))
	_ = tw.Close()
	_ = gz.Close()
	sum := sha256.Sum256(b.Bytes())
	name := target.Archive("v0.2.0")
	mux := http.NewServeMux()
	mux.HandleFunc("/releases", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"mock_machina/v0.2.0"}]`))
	})
	mux.HandleFunc("/dl/mock_machina/v0.2.0/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(b.Bytes()) })
	mux.HandleFunc("/dl/mock_machina/v0.2.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return update.Source{Client: srv.Client(), API: srv.URL + "/releases", Download: srv.URL + "/dl"}
}

func runUpdateFor(t *testing.T, current string, check bool) (string, string, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake release is a tar.gz")
	}
	exe := filepath.Join(t.TempDir(), "mockmachina")
	_ = os.WriteFile(exe, []byte("old"), 0o755)
	var out strings.Builder
	err := runUpdate(t.Context(), &out, updateOptions{current: current, check: check, exe: exe, src: releaseServer(t)})
	data, _ := os.ReadFile(exe)
	return out.String(), string(data), err
}

func TestUpdate_ReplacesAnOlderBinary(t *testing.T) {
	t.Parallel()

	out, bin, err := runUpdateFor(t, "v0.1.1", false)
	if err != nil || bin != "new" || !strings.Contains(out, "updated mockmachina v0.1.1 → v0.2.0") {
		t.Errorf("out %q, binary %q, err %v", out, bin, err)
	}
}

func TestUpdate_CheckOnlyReports(t *testing.T) {
	t.Parallel()

	out, bin, err := runUpdateFor(t, "v0.1.1", true)
	if err != nil || bin != "old" || !strings.Contains(out, "v0.2.0 is out (you have v0.1.1); run mockmachina update") {
		t.Errorf("out %q, binary %q, err %v", out, bin, err)
	}
}

func TestUpdate_AlreadyLatest(t *testing.T) {
	t.Parallel()

	out, bin, err := runUpdateFor(t, "v0.2.0", false)
	if err != nil || bin != "old" || !strings.Contains(out, "mockmachina v0.2.0 is the latest") {
		t.Errorf("out %q, binary %q, err %v", out, bin, err)
	}
}

func TestUpdate_DevelopmentBuildIsLeftAlone(t *testing.T) {
	t.Parallel()

	out, bin, err := runUpdateFor(t, "(devel)", false)
	if err != nil || bin != "old" || !strings.Contains(out, "development build") {
		t.Errorf("out %q, binary %q, err %v", out, bin, err)
	}
}

func TestUpdateCommand_ByInstallMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		how  update.Method
		want []string
	}{
		{update.Homebrew, []string{"brew", "upgrade", "mockmachina"}},
		{update.Scoop, []string{"scoop", "update", "mockmachina"}},
		{update.Go, []string{"go", "install", "github.com/demola234/tiny-tools/mock_machina/cmd/mockmachina@v0.2.0"}},
		{update.Replace, nil},
	}
	for _, tt := range tests {
		got := managerCommand(tt.how, "v0.2.0")
		if strings.Join(got, " ") != strings.Join(tt.want, " ") {
			t.Errorf("%v: %v, want %v", tt.how, got, tt.want)
		}
	}
}
