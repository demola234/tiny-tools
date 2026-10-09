package update_test

import (
	"archive/tar"
	"archive/zip"
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
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/release"
	"github.com/demola234/tiny-tools/mock_machina/internal/update"
)

func TestNewer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.0", "v0.1.1", true},
		{"v0.1.1", "v0.1.1", false},
		{"v0.2.0", "v0.1.9", false},
		{"v0.9.0", "v0.10.0", true},
		{"v1.0.0-rc.1", "v1.0.0", true},
		{"v1.0.0", "v1.0.0-rc.1", false},
		{"(devel)", "v0.1.1", true},
		{"v0.1.1", "nonsense", false},
	}
	for _, tt := range tests {
		if got := update.Newer(tt.current, tt.latest); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestHow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		exe, goos, gobin string
		want             update.Method
	}{
		{"/opt/homebrew/Cellar/mockmachina/0.1.1/bin/mockmachina", "darwin", "", update.Homebrew},
		{"/home/linuxbrew/.linuxbrew/Cellar/mockmachina/0.1.1/bin/mockmachina", "linux", "", update.Homebrew},
		{`C:\Users\ada\scoop\apps\mockmachina\current\mockmachina.exe`, "windows", "", update.Scoop},
		{"/home/ada/go/bin/mockmachina", "linux", "/home/ada/go/bin", update.Go},
		{"/usr/local/bin/mockmachina", "linux", "/home/ada/go/bin", update.Replace},
	}
	for _, tt := range tests {
		if got := update.How(tt.exe, tt.goos, tt.gobin); got != tt.want {
			t.Errorf("How(%q) = %v, want %v", tt.exe, got, tt.want)
		}
	}
}

func archive(t *testing.T, target release.Target, bin []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	if target.OS == "windows" {
		zw := zip.NewWriter(&b)
		f, _ := zw.Create(target.Exe())
		_, _ = f.Write(bin)
		_ = zw.Close()
		return b.Bytes()
	}
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "README.md", Mode: 0o644, Size: 2})
	_, _ = tw.Write([]byte("hi"))
	_ = tw.WriteHeader(&tar.Header{Name: target.Exe(), Mode: 0o755, Size: int64(len(bin))})
	_, _ = tw.Write(bin)
	_ = tw.Close()
	_ = gz.Close()
	return b.Bytes()
}

func serve(t *testing.T, target release.Target, bin []byte, corrupt bool) update.Source {
	t.Helper()
	data := archive(t, target, bin)
	sum := sha256.Sum256(data)
	if corrupt {
		sum[0] ^= 0xff
	}
	name := target.Archive("v0.2.0")
	mux := http.NewServeMux()
	mux.HandleFunc("/releases", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"rodinia/v9.0.0","draft":false},{"tag_name":"mock_machina/v0.3.0","draft":true},{"tag_name":"mock_machina/v0.2.0","draft":false},{"tag_name":"mock_machina/v0.1.1","draft":false}]`))
	})
	mux.HandleFunc("/download/mock_machina/v0.2.0/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) })
	mux.HandleFunc("/download/mock_machina/v0.2.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("0000  other.tar.gz\n" + hex.EncodeToString(sum[:]) + "  " + name + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return update.Source{Client: srv.Client(), API: srv.URL + "/releases", Download: srv.URL + "/download"}
}

func TestLatest_SkipsOtherToolsAndDrafts(t *testing.T) {
	t.Parallel()

	src := serve(t, release.Target{OS: "linux", Arch: "amd64"}, []byte("new"), false)
	got, err := src.Latest(t.Context())
	if err != nil || got != "v0.2.0" {
		t.Errorf("Latest = %q, %v; want v0.2.0", got, err)
	}
}

func TestReplace_SwapsTheBinary(t *testing.T) {
	t.Parallel()

	for _, target := range []release.Target{{OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		src := serve(t, target, []byte("new binary"), false)
		exe := filepath.Join(t.TempDir(), target.Exe())
		if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := src.Replace(t.Context(), "v0.2.0", target, exe); err != nil {
			t.Fatalf("%s: %v", target.OS, err)
		}
		got, _ := os.ReadFile(exe)
		if string(got) != "new binary" {
			t.Errorf("%s: binary = %q", target.OS, got)
		}
		if info, _ := os.Stat(exe); runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
			t.Errorf("%s: not executable: %v", target.OS, info.Mode())
		}
		leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), ".mockmachina-*"))
		if len(leftovers) > 0 {
			t.Errorf("%s: temp files left: %v", target.OS, leftovers)
		}
	}
}

func TestReplace_RefusesABadChecksum(t *testing.T) {
	t.Parallel()

	target := release.Target{OS: "linux", Arch: "amd64"}
	src := serve(t, target, []byte("tampered"), true)
	exe := filepath.Join(t.TempDir(), "mockmachina")
	_ = os.WriteFile(exe, []byte("old binary"), 0o755)
	err := src.Replace(t.Context(), "v0.2.0", target, exe)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("err = %v; want a checksum error", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Errorf("binary changed to %q after a bad checksum", got)
	}
}

func TestReplace_ReportsDownloadProgress(t *testing.T) {
	t.Parallel()

	target := release.Target{OS: "linux", Arch: "amd64"}
	src := serve(t, target, bytes.Repeat([]byte("x"), 200_000), false)
	var calls int
	var last, total int64
	src.Progress = func(done, size int64) { calls, last, total = calls+1, done, size }
	exe := filepath.Join(t.TempDir(), "mockmachina")
	_ = os.WriteFile(exe, []byte("old"), 0o755)
	if err := src.Replace(t.Context(), "v0.2.0", target, exe); err != nil {
		t.Fatal(err)
	}
	if calls == 0 || last != total || total <= 0 {
		t.Errorf("progress: %d calls, last %d of %d; want calls ending at the full size", calls, last, total)
	}
}

func TestReplace_GivesUpWhenTheDownloadStalls(t *testing.T) {
	t.Parallel()

	stop := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		<-stop
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { close(stop); srv.Close() })
	src := update.Source{Client: srv.Client(), API: srv.URL, Download: srv.URL + "/dl", Idle: 50 * time.Millisecond}
	exe := filepath.Join(t.TempDir(), "mockmachina")
	_ = os.WriteFile(exe, []byte("old"), 0o755)
	err := src.Replace(t.Context(), "v0.2.0", release.Target{OS: "linux", Arch: "amd64"}, exe)
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Errorf("err = %v; want a stalled download", err)
	}
}
