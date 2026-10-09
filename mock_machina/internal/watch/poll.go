package watch

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
)

type Poller struct {
	Dir      string
	Interval time.Duration
	Clock    clock.Clock
}

type Watcher struct {
	poller Poller
	prev   map[string]stamp
}

func (p Poller) Start() *Watcher { return &Watcher{poller: p, prev: snapshot(p.Dir)} }

func (w *Watcher) Run(ctx context.Context, onChange func(changed []string)) {
	p, prev := w.poller, w.prev
	pending := map[string]struct{}{}
	for {
		if p.Clock.Sleep(ctx, p.Interval) != nil {
			return
		}
		cur := snapshot(p.Dir)
		if changed := diff(prev, cur); len(changed) > 0 {
			for _, name := range changed {
				pending[name] = struct{}{}
			}
			prev = cur
			continue
		}
		if len(pending) > 0 {
			onChange(slices.Sorted(maps.Keys(pending)))
			clear(pending)
		}
	}
}

type stamp struct {
	size    int64
	modTime time.Time
}

func snapshot(dir string) map[string]stamp {
	files := map[string]stamp{}
	_ = fs.WalkDir(os.DirFS(dir), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || name == "." {
			return nil //nolint:nilerr // unreadable entries are skipped; the next tick retries
		}
		if ignored(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files[name] = stamp{size: info.Size(), modTime: info.ModTime()}
		}
		return nil
	})
	return files
}

func ignored(name string) bool {
	return strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "~") ||
		name == "4913" ||
		(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#"))
}

func diff(prev, cur map[string]stamp) []string {
	var changed []string
	for name, s := range cur {
		if old, ok := prev[name]; !ok || old.size != s.size || !old.modTime.Equal(s.modTime) {
			changed = append(changed, name)
		}
	}
	for name := range prev {
		if _, ok := cur[name]; !ok {
			changed = append(changed, name)
		}
	}
	return changed
}
