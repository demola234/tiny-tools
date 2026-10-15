package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

const (
	webhookTimeout   = 10 * time.Second
	defaultRetryWait = time.Second
)

type hook struct {
	spec    model.Webhook
	url     tmpl.Text
	headers map[string]tmpl.Text
	body    *templates
}

type outgoing struct {
	method, url string
	headers     http.Header
	body        []byte
}

func compileHooks(rt *model.Route) (map[string][]hook, error) {
	out := map[string][]hook{}
	for _, ns := range rt.States {
		for _, w := range ns.State.Webhooks {
			h, err := compileHook(w)
			if err != nil {
				return nil, fmt.Errorf("route %s state %s webhook: %w", rt.ID, ns.Name, err)
			}
			out[ns.Name] = append(out[ns.Name], h)
		}
	}
	return out, nil
}

func compileHook(w model.Webhook) (hook, error) {
	h := hook{spec: w, headers: map[string]tmpl.Text{}, body: &templates{}}
	var err error
	if h.url, err = tmpl.Parse(w.URL); err != nil {
		return hook{}, err
	}
	for name, value := range w.Headers {
		if h.headers[name], err = tmpl.Parse(value); err != nil {
			return hook{}, err
		}
	}
	return h, h.body.compileBody(w.Body)
}

func (h hook) render(env tmpl.Env) outgoing {
	o := outgoing{method: h.spec.Method, url: fmt.Sprint(h.url.Eval(env)), headers: http.Header{}}
	for name, value := range h.headers {
		o.headers.Set(name, fmt.Sprint(value.Eval(env)))
	}
	if len(h.spec.Body.Data) > 0 {
		o.headers.Set("Content-Type", h.spec.Body.ContentType)
		o.body = h.body.apply(http.Header{}, h.spec.Body.Data, env)
	}
	return o
}

func (x *exchange) sendWebhooks(ctx context.Context, hooks []hook) {
	if len(hooks) == 0 {
		return
	}
	env := x.env()
	for _, h := range hooks {
		go x.deliver(context.WithoutCancel(ctx), h.render(env), h.spec)
	}
}

func (x *exchange) deliver(ctx context.Context, o outgoing, spec model.Webhook) {
	if x.opts.Clock.Sleep(ctx, spec.After) != nil {
		return
	}
	wait := x.opts.WebhookRetryWait
	if wait == 0 {
		wait = defaultRetryWait
	}
	for attempt := 0; ; attempt++ {
		status := x.post(ctx, o)
		if status >= http.StatusOK && status < http.StatusMultipleChoices || attempt >= spec.Retries {
			return
		}
		if x.opts.Clock.Sleep(ctx, wait<<attempt) != nil {
			return
		}
	}
}

func (x *exchange) post(parent context.Context, o outgoing) int {
	ctx, cancel := context.WithTimeout(parent, webhookTimeout)
	defer cancel()
	report := x.opts.reporter()
	rec := Request{Method: o.method, Path: o.url, Route: x.rt.ID, State: x.state, Webhook: true}
	defer func() { report(rec) }()
	req, err := http.NewRequestWithContext(ctx, o.method, o.url, bytes.NewReader(o.body))
	if err != nil {
		rec.Problem, rec.Detail = ProblemBackend, err.Error()
		return 0
	}
	req.Header = o.headers
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		rec.Problem, rec.Detail = ProblemBackend, err.Error()
		return 0
	}
	_ = res.Body.Close()
	rec.Status = res.StatusCode
	return res.StatusCode
}
