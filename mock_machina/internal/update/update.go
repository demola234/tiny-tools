package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/release"
)

const (
	tagPrefix   = "mock_machina/"
	maxDownload = 256 << 20
	defaultIdle = 30 * time.Second
)

var errStalled = errors.New("download stalled")

type Method int

const (
	Replace Method = iota
	Homebrew
	Scoop
	Go
)

type Source struct {
	Client   *http.Client
	API      string
	Download string
	Progress func(done, total int64)
	Idle     time.Duration
}

var version = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$`)

func parse(v string) ([3]int, string, bool) {
	m := version.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, "", false
	}
	var n [3]int
	for i := range n {
		n[i], _ = strconv.Atoi(m[i+1])
	}
	return n, m[4], true
}

func Newer(current, latest string) bool {
	l, lpre, ok := parse(latest)
	if !ok {
		return false
	}
	c, cpre, ok := parse(current)
	if !ok {
		return true
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return cpre != "" && (lpre == "" || lpre > cpre)
}

func How(exe, goos, gobin string) Method {
	p := strings.ToLower(filepath.ToSlash(strings.ReplaceAll(exe, `\`, "/")))
	switch {
	case strings.Contains(p, "/cellar/"):
		return Homebrew
	case goos == "windows" && strings.Contains(p, "/scoop/apps/"):
		return Scoop
	case gobin != "" && path.Dir(p) == strings.ToLower(strings.ReplaceAll(gobin, `\`, "/")):
		return Go
	}
	return Replace
}

func (s Source) get(ctx context.Context, url string) ([]byte, error) {
	return s.fetch(ctx, url, nil)
}

type counter struct {
	r      io.Reader
	done   atomic.Int64
	total  int64
	report func(done, total int64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	done := c.done.Add(int64(n))
	if c.report != nil && (n > 0 || errors.Is(err, io.EOF)) {
		c.report(done, max(c.total, done))
	}
	return n, err
}

func watch(ctx context.Context, stop context.CancelCauseFunc, idle time.Duration, c *counter) {
	last := c.done.Load()
	for (clock.Real{}).Sleep(ctx, idle) == nil {
		now := c.done.Load()
		if now == last {
			stop(errStalled)
			return
		}
		last = now
	}
}

func (s Source) fetch(parent context.Context, url string, report func(done, total int64)) ([]byte, error) {
	ctx, stop := context.WithCancelCause(parent)
	defer stop(nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, res.Status)
	}
	if report != nil {
		report(0, max(res.ContentLength, 0))
	}
	c := &counter{r: io.LimitReader(res.Body, maxDownload), total: res.ContentLength, report: report}
	go watch(ctx, stop, cmp.Or(s.Idle, defaultIdle), c)
	data, err := io.ReadAll(c)
	if errors.Is(context.Cause(ctx), errStalled) {
		return nil, fmt.Errorf("%w: no data for %s from %s", errStalled, cmp.Or(s.Idle, defaultIdle), url)
	}
	return data, err
}

func (s Source) Latest(ctx context.Context) (string, error) {
	data, err := s.get(ctx, s.API)
	if err != nil {
		return "", err
	}
	var releases []struct {
		Tag   string `json:"tag_name"`
		Draft bool   `json:"draft"`
	}
	if err := json.Unmarshal(data, &releases); err != nil {
		return "", fmt.Errorf("reading the release list: %w", err)
	}
	for _, r := range releases {
		if v, ok := strings.CutPrefix(r.Tag, tagPrefix); ok && !r.Draft {
			return v, nil
		}
	}
	return "", errors.New("no MockMachina release found")
}

func (s Source) Replace(ctx context.Context, v string, t release.Target, exe string) error {
	name := t.Archive(v)
	base := s.Download + "/" + tagPrefix + v + "/"
	data, err := s.fetch(ctx, base+name, s.Progress)
	if err != nil {
		return err
	}
	sums, err := s.get(ctx, base+"checksums.txt")
	if err != nil {
		return err
	}
	if err := verify(data, sums, name); err != nil {
		return err
	}
	bin, err := extract(data, t)
	if err != nil {
		return err
	}
	return swap(exe, bin, t.OS == "windows")
}

func verify(data, sums []byte, name string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	for line := range strings.SplitSeq(string(sums), "\n") {
		if want, file, ok := strings.Cut(line, "  "); ok && file == name {
			if want != got {
				return fmt.Errorf("checksum doesn't match for %s; not updating", name)
			}
			return nil
		}
	}
	return fmt.Errorf("checksums.txt has no line for %s; not updating", name)
}

func extract(data []byte, t release.Target) ([]byte, error) {
	if t.OS == "windows" {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		f, err := zr.Open(t.Exe())
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		return io.ReadAll(io.LimitReader(f, maxDownload))
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("%s isn't in the archive: %w", t.Exe(), err)
		}
		if h.Name == t.Exe() {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
}

func swap(exe string, bin []byte, windows bool) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".mockmachina-*")
	if err != nil {
		return fmt.Errorf("can't write next to %s: %w", exe, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(bin); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil { //nolint:gosec // an executable must be executable
		return err
	}
	if windows {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
		defer func() { _ = os.Remove(old) }()
	}
	return os.Rename(tmp.Name(), exe)
}
