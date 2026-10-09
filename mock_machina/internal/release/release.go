package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	binary   = "mockmachina"
	pkg      = "./cmd/mockmachina"
	ldflag   = "github.com/demola234/tiny-tools/mock_machina/internal/buildinfo.version"
	summary  = "Mock HTTP APIs from contract files in your repository"
	homepage = "https://github.com/demola234/tiny-tools/tree/main/mock_machina"
	tagPath  = "mock_machina/"
)

type Target struct{ OS, Arch string }

type Options struct {
	Version   string
	ModuleDir string
	Out       string
	Targets   []Target
}

type Artifact struct {
	Name   string
	Target Target
	SHA256 string
}

var Targets = []Target{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

var extras = []string{"README.md", "CHANGELOG.md", "LICENSE"}

var epoch = time.Unix(0, 0).UTC()

func plain(version string) string { return strings.TrimPrefix(version, "v") }

func (t Target) archive(version string) string {
	ext := ".tar.gz"
	if t.OS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s%s", binary, plain(version), t.OS, t.Arch, ext)
}

func (t Target) exe() string {
	if t.OS == "windows" {
		return binary + ".exe"
	}
	return binary
}

func Build(ctx context.Context, o Options) ([]Artifact, error) {
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return nil, err
	}
	var arts []Artifact
	var sums strings.Builder
	for _, t := range o.Targets {
		bin, err := compile(ctx, o, t)
		if err != nil {
			return nil, err
		}
		data, err := pack(o.ModuleDir, t, bin)
		if err != nil {
			return nil, err
		}
		name := t.archive(o.Version)
		if err := os.WriteFile(filepath.Join(o.Out, name), data, 0o644); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		art := Artifact{Name: name, Target: t, SHA256: hex.EncodeToString(sum[:])}
		arts = append(arts, art)
		fmt.Fprintf(&sums, "%s  %s\n", art.SHA256, art.Name)
	}
	return arts, os.WriteFile(filepath.Join(o.Out, "checksums.txt"), []byte(sums.String()), 0o644)
}

func compile(ctx context.Context, o Options, t Target) ([]byte, error) {
	dir, err := os.MkdirTemp("", "mockmachina-release-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	out := filepath.Join(dir, t.exe())
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", "-s -w -X "+ldflag+"="+o.Version, "-o", out, pkg) //nolint:gosec // the version comes from the maintainer's own release command
	cmd.Dir = o.ModuleDir
	cmd.Env = append(os.Environ(), "GOOS="+t.OS, "GOARCH="+t.Arch, "CGO_ENABLED=0")
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("building %s/%s: %w\n%s", t.OS, t.Arch, err, msg)
	}
	return os.ReadFile(out)
}

type entry struct {
	name string
	mode os.FileMode
	data []byte
}

func pack(moduleDir string, t Target, bin []byte) ([]byte, error) {
	files := []entry{{name: t.exe(), mode: 0o755, data: bin}}
	for _, name := range extras {
		data, err := os.ReadFile(filepath.Join(moduleDir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files = append(files, entry{name: name, mode: 0o644, data: data})
	}
	if t.OS == "windows" {
		return zipped(files)
	}
	return tarred(files)
}

func zipped(files []entry) ([]byte, error) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: epoch}
		h.SetMode(f.mode)
		fw, err := w.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(f.data); err != nil {
			return nil, err
		}
	}
	err := w.Close()
	return b.Bytes(), err
}

func tarred(files []entry) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.ModTime = epoch
	tw := tar.NewWriter(gz)
	for _, f := range files {
		h := &tar.Header{Name: f.name, Mode: int64(f.mode), Size: int64(len(f.data)), ModTime: epoch, Format: tar.FormatPAX}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	err := gz.Close()
	return b.Bytes(), err
}

func DownloadURL(repo, version, name string) string {
	return "https://github.com/" + repo + "/releases/download/" + tagPath + version + "/" + name
}

func find(arts []Artifact, osName, arch string) Artifact {
	for _, a := range arts {
		if a.Target.OS == osName && a.Target.Arch == arch {
			return a
		}
	}
	return Artifact{}
}

func Formula(version, repo string, arts []Artifact) []byte {
	var b strings.Builder
	block := func(osName, arch string) {
		a := find(arts, osName, arch)
		fmt.Fprintf(&b, "      url %q\n      sha256 %q\n", DownloadURL(repo, version, a.Name), a.SHA256)
	}
	fmt.Fprintf(&b, "class Mockmachina < Formula\n  desc %q\n  homepage %q\n  version %q\n  license \"Apache-2.0\"\n\n", summary, homepage, plain(version))
	for _, osName := range []string{"darwin", "linux"} {
		fmt.Fprintf(&b, "  on_%s do\n    on_arm do\n", map[string]string{"darwin": "macos", "linux": "linux"}[osName])
		block(osName, "arm64")
		b.WriteString("    end\n    on_intel do\n")
		block(osName, "amd64")
		b.WriteString("    end\n  end\n\n")
	}
	b.WriteString("  def install\n    bin.install \"mockmachina\"\n  end\n\n")
	b.WriteString("  test do\n    assert_match version.to_s, shell_output(\"#{bin}/mockmachina --version\")\n  end\nend\n")
	return []byte(b.String())
}

type scoopArch struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

type scoopManifest struct {
	Version      string               `json:"version"`
	Description  string               `json:"description"`
	Homepage     string               `json:"homepage"`
	License      string               `json:"license"`
	Architecture map[string]scoopArch `json:"architecture"`
	Bin          string               `json:"bin"`
}

func Scoop(version, repo string, arts []Artifact) []byte {
	arch := func(a string) scoopArch {
		art := find(arts, "windows", a)
		return scoopArch{URL: DownloadURL(repo, version, art.Name), Hash: art.SHA256}
	}
	m := scoopManifest{
		Version: plain(version), Description: summary, Homepage: homepage, License: "Apache-2.0",
		Architecture: map[string]scoopArch{"64bit": arch("amd64"), "arm64": arch("arm64")},
		Bin:          binary + ".exe",
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	return append(data, '\n')
}

var heading = regexp.MustCompile(`(?m)^## \[([^\]]+)\].*$`)

func Notes(changelog []byte, version string) (string, error) {
	text := string(changelog)
	sections := map[string]string{}
	matches := heading.FindAllStringSubmatchIndex(text, -1)
	for i, m := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections[text[m[2]:m[3]]] = strings.TrimSpace(text[m[1]:end]) + "\n"
	}
	if s, ok := sections[plain(version)]; ok {
		return s, nil
	}
	if s, ok := sections["Unreleased"]; ok {
		return s, nil
	}
	return "", fmt.Errorf("CHANGELOG.md has no section for %s and no Unreleased section", version)
}
