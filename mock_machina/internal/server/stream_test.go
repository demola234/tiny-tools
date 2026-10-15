package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const liveRoutes = `room:
  route: WS /chat/{room}
  states:
    busy:
      send:
        - { type: welcome, room: "{{ path.room }}" }
        - hello
      reply:
        - when: { message.type: ping }
          send: { type: pong, id: "{{ message.id }}" }
        - when: { message: bye }
          send: see you
      every: 20ms
      push: [{ type: tick, n: 1 }, { type: tick, n: 2 }]
    closing:
      send: welcome
      close: { after: 30ms, code: 4001, reason: shift over }
    dropped:
      send: welcome
      close: { after: 30ms, drop: true }
    unauthorized:
      status: 401
      body: { error: sign in first }
events:
  route: SSE /orders/{id}/events
  states:
    on_the_way:
      send: [{ order: "{{ path.id }}", status: dispatched }, "two\nlines"]
      every: 20ms
      push: { status: out_for_delivery }
      close: { after: 70ms }
    expired:
      status: 401
`

func streamServer(t *testing.T) (*httptest.Server, <-chan server.Request) {
	t.Helper()
	proj, probs, err := config.LoadFS(testkit.MapFS(t, map[string]string{"routes/live.yaml": liveRoutes}))
	if err != nil || len(probs) > 0 {
		t.Fatalf("load: %v %v", probs, err)
	}
	got := make(chan server.Request, 16)
	h, err := server.New(proj, server.Options{Report: func(r server.Request) { got <- r }})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, got
}

func dial(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	c, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+path, nil)
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func read(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data)
}

func readUntil(t *testing.T, c *websocket.Conn, want string) {
	t.Helper()
	for range 20 {
		if read(t, c) == want {
			return
		}
	}
	t.Fatalf("never got %s", want)
}

func TestWebSocket_SendReplyAndPush(t *testing.T) {
	t.Parallel()

	srv, got := streamServer(t)
	c := dial(t, srv, "/chat/lagos")
	if r := <-got; r.Method != "WS" || r.Status != http.StatusSwitchingProtocols || r.Route != "live.room" || r.State != "busy" {
		t.Errorf("report = %+v", r)
	}
	if m := read(t, c); m != `{"type":"welcome","room":"lagos"}` {
		t.Errorf("first message = %s", m)
	}
	if m := read(t, c); m != "hello" {
		t.Errorf("second message = %q", m)
	}
	if err := c.Write(t.Context(), websocket.MessageText, []byte(`{"type":"ping","id":7}`)); err != nil {
		t.Fatal(err)
	}
	readUntil(t, c, `{"type":"pong","id":7}`)
	if err := c.Write(t.Context(), websocket.MessageText, []byte("bye")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, c, "see you")
	readUntil(t, c, `{"type":"tick","n":2}`)
	readUntil(t, c, `{"type":"tick","n":1}`)
}

func TestWebSocket_CloseWithCode(t *testing.T) {
	t.Parallel()

	srv, _ := streamServer(t)
	c := dial(t, srv, "/chat/x?__state=closing")
	if m := read(t, c); m != "welcome" {
		t.Errorf("message = %q", m)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if status := websocket.CloseStatus(err); status != 4001 {
		t.Errorf("close status = %d (%v), want 4001", status, err)
	}
	var ce websocket.CloseError
	if errors.As(err, &ce) && ce.Reason != "shift over" {
		t.Errorf("reason = %q", ce.Reason)
	}
}

func TestWebSocket_Drop(t *testing.T) {
	t.Parallel()

	srv, _ := streamServer(t)
	c := dial(t, srv, "/chat/x?__state=dropped")
	_ = read(t, c)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if err == nil || websocket.CloseStatus(err) != -1 {
		t.Errorf("want a dropped connection with no close frame, got %v", err)
	}
}

func TestWebSocket_RefusedWithStatus(t *testing.T) {
	t.Parallel()

	srv, _ := streamServer(t)
	_, res, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/chat/x?__state=unauthorized", nil)
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want a 401 refusal, got %v %v", res, err)
	}
	defer res.Body.Close()
	var body map[string]string
	_ = json.NewDecoder(res.Body).Decode(&body)
	if body["error"] != "sign in first" {
		t.Errorf("body = %v", body)
	}
}

func TestSSE_Stream(t *testing.T) {
	t.Parallel()

	srv, got := streamServer(t)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/orders/o_1/events", nil)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content type = %q", ct)
	}
	if r := <-got; r.Status != http.StatusOK || r.State != "on_the_way" {
		t.Errorf("report = %+v", r)
	}
	var events []string
	var data []string
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			events = append(events, strings.Join(data, "\n"))
			data = nil
			continue
		}
		data = append(data, strings.TrimPrefix(line, "data: "))
	}
	if len(events) < 4 {
		t.Fatalf("events = %q; want the 2 sends and at least 2 pushes before close", events)
	}
	if events[0] != `{"order":"o_1","status":"dispatched"}` || events[1] != "two\nlines" || events[2] != `{"status":"out_for_delivery"}` {
		t.Errorf("events = %q", events)
	}
}

func TestSSE_Refused(t *testing.T) {
	t.Parallel()

	srv, _ := streamServer(t)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/orders/o_1/events?__state=expired", nil)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d", res.StatusCode)
	}
}
