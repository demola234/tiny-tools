package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const hookProject = `create:
  route: POST /payments
  states:
    pending:
      status: 202
      set: { payment: "pay_{{ random.int 1000 9999 }}" }
      body: { id: "{{ var.payment }}", status: pending }
      webhooks:
        - url: "{{ header.x-hook-url }}/events"
          after: 20ms
          headers: { X-Signature: test }
          body: { event: payment.succeeded, id: "{{ var.payment }}" }
          retries: 2
`

type receiver struct {
	mu    sync.Mutex
	got   []string
	fails int
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, req.Method+" "+req.URL.Path+" "+req.Header.Get("X-Signature")+" "+string(body))
	if r.fails > 0 {
		r.fails--
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (r *receiver) deliveries() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.got...)
}

func hookServer(t *testing.T, fails int) (*httptest.Server, *receiver, string, <-chan server.Request) {
	t.Helper()
	recv := &receiver{fails: fails}
	hooks := httptest.NewServer(recv)
	t.Cleanup(hooks.Close)
	proj, probs, err := config.LoadFS(testkit.MapFS(t, map[string]string{"routes/payments.yaml": hookProject}))
	if err != nil || len(probs) > 0 {
		t.Fatalf("load: %v %v", probs, err)
	}
	got := make(chan server.Request, 16)
	h, err := server.New(proj, server.Options{Report: func(r server.Request) { got <- r }, WebhookRetryWait: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, recv, hooks.URL, got
}

func pay(t *testing.T, srv *httptest.Server, hookURL string) map[string]string {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/payments", strings.NewReader("{}"))
	req.Header.Set("X-Hook-Url", hookURL)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]string
	_ = json.NewDecoder(res.Body).Decode(&body)
	return body
}

func nextWebhook(t *testing.T, got <-chan server.Request) server.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		select {
		case r := <-got:
			if r.Webhook {
				return r
			}
		case <-ctx.Done():
			t.Fatal("no webhook report")
		}
	}
}

func TestWebhook_SentAfterTheResponse(t *testing.T) {
	t.Parallel()

	srv, recv, hooks, got := hookServer(t, 0)
	body := pay(t, srv, hooks)
	r := nextWebhook(t, got)
	if r.Status != http.StatusOK || r.Route != "payments.create" || r.State != "pending" || r.Method != http.MethodPost || !strings.HasSuffix(r.Path, "/events") {
		t.Errorf("webhook report = %+v", r)
	}
	want := `POST /events test {"event":"payment.succeeded","id":"` + body["id"] + `"}`
	if d := recv.deliveries(); len(d) != 1 || d[0] != want {
		t.Errorf("deliveries = %q, want [%s]", d, want)
	}
}

func TestWebhook_RetriesAFailingReceiver(t *testing.T) {
	t.Parallel()

	srv, recv, hooks, got := hookServer(t, 2)
	pay(t, srv, hooks)
	statuses := []int{nextWebhook(t, got).Status, nextWebhook(t, got).Status, nextWebhook(t, got).Status}
	if statuses[0] != 500 || statuses[1] != 500 || statuses[2] != 200 || len(recv.deliveries()) != 3 {
		t.Errorf("statuses %v, %d deliveries", statuses, len(recv.deliveries()))
	}
}
