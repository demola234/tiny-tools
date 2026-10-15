package config_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const hookRoute = `create:
  route: POST /payments
  states:
    pending:
      status: 202
      set: { payment: "pay_{{ random.int 1000 9999 }}" }
      body: { id: "{{ var.payment }}", status: pending }
      webhooks:
        - url: "{{ header.x-hook-url }}/events"
          after: 2s
          headers: { X-Signature: test }
          body: { event: payment.succeeded, id: "{{ var.payment }}" }
          retries: 3
        - url: http://localhost:3000/audit
          method: PUT
`

func TestLoadFS_Webhooks(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/payments.yaml": hookRoute})
	r, _ := p.Route("payments.create")
	st, _ := r.States.Get("pending")
	want := []model.Webhook{
		{
			URL: "{{ header.x-hook-url }}/events", Method: "POST", After: 2 * time.Second, Headers: map[string]string{"X-Signature": "test"},
			Body: model.Body{Data: []byte(`{"event":"payment.succeeded","id":"{{ var.payment }}"}`), ContentType: "application/json"}, Retries: 3,
			Src: model.Source{File: "routes/payments.yaml", Line: 9},
		},
		{URL: "http://localhost:3000/audit", Method: "PUT", Src: model.Source{File: "routes/payments.yaml", Line: 14}},
	}
	if diff := cmp.Diff(want, st.Webhooks); diff != "" {
		t.Errorf("webhooks (-want +got):\n%s", diff)
	}
}

func TestLoadFS_WebhookProblems(t *testing.T) {
	t.Parallel()

	route := func(hooks string) string {
		return "create:\n  route: POST /payments\n  states:\n    s:\n      webhooks:\n" + hooks
	}
	tests := []struct{ name, file, want string }{
		{"not a list", "create:\n  route: POST /p\n  states:\n    s:\n      webhooks: { url: x }\n", `routes/payments.yaml:5: webhooks must be a list of { url, body, after }`},
		{"no url", route("        - body: hi\n"), `routes/payments.yaml:6: webhook 1 needs "url"`},
		{"relative url", route("        - url: /events\n"), `routes/payments.yaml:6: webhook 1 url "/events" must start with http:// or https://, or a template like {{ var.hook_url }}`},
		{"bad method", route("        - url: http://x\n          method: post\n"), `routes/payments.yaml:7: method "post" must be uppercase: POST`},
		{"bad template", route("        - url: http://x\n          body: { a: \"{{ nope }}\" }\n"), `routes/payments.yaml:7: webhook 1 body: unknown template "nope" (did you mean "now"?)`},
		{"too many retries", route("        - url: http://x\n          retries: 20\n"), `routes/payments.yaml:7: webhook 1 retries must be 0 to 10`},
		{"unknown field", route("        - url: http://x\n          delay: 1s\n"), `routes/payments.yaml:7: unknown field "delay" (expected one of: url, method, after, headers, body, retries)`},
		{"on a stream route", "room:\n  route: WS /chat\n  states:\n    s:\n      webhooks:\n        - url: http://x\n", `routes/payments.yaml:6: webhooks only work on HTTP routes`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, map[string]string{"routes/payments.yaml": tc.file})
			if got := probs.String(); got != tc.want {
				t.Errorf("problems:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
