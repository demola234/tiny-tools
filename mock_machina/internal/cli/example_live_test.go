package cli_test

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func streamExample(t *testing.T) string {
	t.Helper()
	src := filepath.Join(filepath.Dir(testkit.Path(t)), "examples", "storefront", ".mockmachina", "routes", "live.yaml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "live.yaml"), string(data))
	return dir
}

func TestExample_LiveChat(t *testing.T) {
	t.Parallel()

	lines, stop := startProject(t, streamExample(t))
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))
	go func() {
		for range lines {
		}
	}()
	ws := "ws" + strings.TrimPrefix(base, "http")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	_, res, err := websocket.Dial(ctx, ws+"/support/help", nil)
	if res != nil {
		_ = res.Body.Close()
	}
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %v %v", res, err)
	}

	c, res, err := websocket.Dial(ctx, ws+"/support/help?token=t_1", nil)
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	if _, data, err := c.Read(ctx); err != nil || !strings.Contains(string(data), `"room":"help"`) {
		t.Fatalf("welcome = %s, %v", data, err)
	}
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"message","text":"where is my order?"}`)); err != nil {
		t.Fatal(err)
	}
	if _, data, err := c.Read(ctx); err != nil || string(data) != `{"type":"message","from":"agent","text":"Thanks, noted: where is my order?"}` {
		t.Errorf("answer = %s, %v", data, err)
	}
}

func TestExample_LiveTracking(t *testing.T) {
	t.Parallel()

	lines, stop := startProject(t, streamExample(t))
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))
	go func() {
		for range lines {
		}
	}()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/orders/o_1001/live", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	if !sc.Scan() || sc.Text() != `data: {"order":"o_1001","stage":"packed"}` {
		t.Errorf("first event = %q", sc.Text())
	}
}
