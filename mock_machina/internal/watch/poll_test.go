package watch_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/watch"
)

const tick = 100 * time.Millisecond

type recorder struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *recorder) onChange(changed []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, changed)
}

func (r *recorder) get() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type harness struct {
	t      *testing.T
	dir    string
	rec    *recorder
	cancel context.CancelFunc
	done   chan struct{}
}

func start(t *testing.T, files map[string]string) *harness {
	t.Helper()
	h := &harness{t: t, dir: t.TempDir(), rec: &recorder{}, done: make(chan struct{})}
	for name, content := range files {
		h.write(name, content)
	}
	ctx, cancel := context.WithCancel(t.Context())
	h.cancel = cancel
	w := watch.Poller{Dir: h.dir, Interval: tick, Clock: clock.Real{}}.Start()
	go func() {
		defer close(h.done)
		w.Run(ctx, h.rec.onChange)
	}()
	synctest.Wait()
	return h
}

func (h *harness) write(name, content string) {
	h.t.Helper()
	path := filepath.Join(h.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) ticks(n int) {
	h.t.Helper()
	if err := (clock.Real{}).Sleep(h.t.Context(), time.Duration(n)*tick); err != nil {
		h.t.Fatal(err)
	}
	synctest.Wait()
}

func (h *harness) stop() {
	h.cancel()
	<-h.done
}

func (h *harness) expect(want ...[]string) {
	h.t.Helper()
	if diff := cmp.Diff(want, h.rec.get()); diff != "" {
		h.t.Errorf("change reports (-want +got):\n%s", diff)
	}
}

func TestPoller_ReportsAChangeOnceItSettles(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h := start(t, map[string]string{"routes/a/route.yaml": "v1"})
		h.write("routes/a/route.yaml", "version 2")
		h.ticks(1)
		h.expect()
		h.ticks(1)
		h.expect([]string{"routes/a/route.yaml"})
		h.ticks(3)
		h.expect([]string{"routes/a/route.yaml"})
		h.stop()
	})
}

func TestPoller_GroupsABurstOfWrites(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h := start(t, map[string]string{"routes/a/route.yaml": "a"})
		h.write("routes/b/route.yaml", "b")
		h.ticks(1)
		h.write("routes/a/route.yaml", "a2")
		h.ticks(1)
		h.expect()
		h.ticks(1)
		h.expect([]string{"routes/a/route.yaml", "routes/b/route.yaml"})
		h.stop()
	})
}

func TestPoller_ReportsDeletesAndNewFolders(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h := start(t, map[string]string{"routes/a/route.yaml": "a", "routes/a/ok.json": "{}"})
		if err := os.Remove(filepath.Join(h.dir, "routes", "a", "ok.json")); err != nil {
			t.Fatal(err)
		}
		h.write("routes/new/route.yaml", "n")
		h.ticks(2)
		h.expect([]string{"routes/a/ok.json", "routes/new/route.yaml"})
		h.stop()
	})
}

func TestPoller_IgnoresHiddenAndEditorFiles(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h := start(t, map[string]string{"routes/a/route.yaml": "a"})
		for _, name := range []string{
			"routes/a/.route.yaml.swp",
			"routes/a/route.yaml~",
			"routes/a/4913",
			"routes/a/#route.yaml#",
			".git/HEAD",
			".run/server.json",
		} {
			h.write(name, "noise")
		}
		h.ticks(3)
		h.expect()
		h.stop()
	})
}

func TestPoller_ReportsChangesMadeBetweenStartAndRun(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "route.yaml")
		if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
			t.Fatal(err)
		}
		w := watch.Poller{Dir: dir, Interval: tick, Clock: clock.Real{}}.Start()
		if err := os.WriteFile(path, []byte("version 2"), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := &recorder{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			defer close(done)
			w.Run(ctx, rec.onChange)
		}()
		if err := (clock.Real{}).Sleep(t.Context(), 2*tick); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		cancel()
		<-done
		if diff := cmp.Diff([][]string{{"route.yaml"}}, rec.get()); diff != "" {
			t.Errorf("change reports (-want +got):\n%s", diff)
		}
	})
}

func TestPoller_StopsWhenCancelled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h := start(t, nil)
		h.ticks(2)
		h.stop()
		h.expect()
	})
}
