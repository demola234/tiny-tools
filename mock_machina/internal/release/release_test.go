package release_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/release"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	return filepath.Dir(testkit.Path(t))
}

func tarFiles(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(tr)
		out[h.Name] = data
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	targets := []release.Target{{OS: runtime.GOOS, Arch: runtime.GOARCH}, {OS: "windows", Arch: "amd64"}}
	arts, err := release.Build(t.Context(), release.Options{Version: "v1.2.3", ModuleDir: moduleDir(t), Out: out, Targets: targets})
	if err != nil {
		t.Fatal(err)
	}
	native := fmt.Sprintf("mockmachina_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		native = strings.TrimSuffix(native, ".tar.gz") + ".zip"
	}
	names := []string{arts[0].Name, arts[1].Name}
	if !slices.Equal(names, []string{native, "mockmachina_1.2.3_windows_amd64.zip"}) {
		t.Fatalf("artifacts = %v", names)
	}
	sums, err := os.ReadFile(filepath.Join(out, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range arts {
		data, _ := os.ReadFile(filepath.Join(out, a.Name))
		want := fmt.Sprintf("%x  %s", sha256.Sum256(data), a.Name)
		if !strings.Contains(string(sums), want+"\n") || a.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
			t.Errorf("checksums.txt lacks %q", want)
		}
	}

	zr, err := zip.OpenReader(filepath.Join(out, "mockmachina_1.2.3_windows_amd64.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	inZip := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		inZip = append(inZip, f.Name)
	}
	slices.Sort(inZip)
	if !slices.Equal(inZip, []string{"CHANGELOG.md", "LICENSE", "README.md", "mockmachina.exe"}) {
		t.Errorf("zip holds %v", inZip)
	}

	if runtime.GOOS == "windows" {
		return
	}
	files := tarFiles(t, filepath.Join(out, native))
	bin := filepath.Join(t.TempDir(), "mockmachina")
	if err := os.WriteFile(bin, files["mockmachina"], 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := exec.CommandContext(t.Context(), bin, "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(got), "mockmachina version v1.2.3") {
		t.Errorf("--version = %s, %v", got, err)
	}
}

var fakeArtifacts = []release.Artifact{
	{Name: "mockmachina_1.2.3_darwin_amd64.tar.gz", Target: release.Target{OS: "darwin", Arch: "amd64"}, SHA256: strings.Repeat("a", 64)},
	{Name: "mockmachina_1.2.3_darwin_arm64.tar.gz", Target: release.Target{OS: "darwin", Arch: "arm64"}, SHA256: strings.Repeat("b", 64)},
	{Name: "mockmachina_1.2.3_linux_amd64.tar.gz", Target: release.Target{OS: "linux", Arch: "amd64"}, SHA256: strings.Repeat("c", 64)},
	{Name: "mockmachina_1.2.3_linux_arm64.tar.gz", Target: release.Target{OS: "linux", Arch: "arm64"}, SHA256: strings.Repeat("d", 64)},
	{Name: "mockmachina_1.2.3_windows_amd64.zip", Target: release.Target{OS: "windows", Arch: "amd64"}, SHA256: strings.Repeat("e", 64)},
	{Name: "mockmachina_1.2.3_windows_arm64.zip", Target: release.Target{OS: "windows", Arch: "arm64"}, SHA256: strings.Repeat("f", 64)},
}

func TestFormula(t *testing.T) {
	t.Parallel()

	testkit.Golden(t, release.Formula("v1.2.3", "demola234/tiny-tools", fakeArtifacts), "release/mockmachina.rb")
}

func TestScoop(t *testing.T) {
	t.Parallel()

	testkit.Golden(t, release.Scoop("v1.2.3", "demola234/tiny-tools", fakeArtifacts), "release/mockmachina.json")
}

func TestNotes(t *testing.T) {
	t.Parallel()

	changelog := "# Changelog\n\nIntro.\n\n## [Unreleased]\n\n- next thing\n\n## [1.2.3] - 2026-10-08\n\n### Added\n\n- a thing\n- another\n\n## [1.2.2]\n\n- old\n"
	notes, err := release.Notes([]byte(changelog), "v1.2.3")
	if err != nil || notes != "### Added\n\n- a thing\n- another\n" {
		t.Errorf("Notes = %q, %v", notes, err)
	}
	unreleased, err := release.Notes([]byte(changelog), "v9.9.9")
	if err != nil || unreleased != "- next thing\n" {
		t.Errorf("a version without a section should use Unreleased: %q, %v", unreleased, err)
	}
	if _, err := release.Notes([]byte("# Changelog\n"), "v1.0.0"); err == nil {
		t.Error("no section and no Unreleased gave no error")
	}
}

func TestLatest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, changelog, want string
	}{
		{"newest version under Unreleased", "# Changelog\n\n## [Unreleased]\n\n- next\n\n## [0.2.0] - 2026-10-09\n\n- b\n\n## [0.1.0]\n\n- a\n", "v0.2.0"},
		{"only Unreleased", "# Changelog\n\n## [Unreleased]\n\n- next\n", ""},
		{"not a version", "## [next]\n\n## [1.0]\n", ""},
		{"pre-release", "## [1.0.0-rc.1]\n", "v1.0.0-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := release.Latest([]byte(tt.changelog)); got != tt.want {
				t.Errorf("Latest = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTargets(t *testing.T) {
	t.Parallel()

	if len(release.Targets) != 6 {
		t.Errorf("Targets = %v", release.Targets)
	}
}
