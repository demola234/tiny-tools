package config

import (
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

var webhookFields = []string{"url", "method", "after", "headers", "body", "retries"}

const maxRetries = 10

func (l *loader) webhooks(file, dir string, method model.Method, v *yaml.Node) []model.Webhook {
	switch {
	case method.Stream():
		l.add(file, v.Line, "webhooks only work on HTTP routes")
		return nil
	case v.Kind != yaml.SequenceNode:
		l.add(file, v.Line, "webhooks must be a list of { url, body, after }")
		return nil
	}
	var out []model.Webhook
	for i, item := range v.Content {
		if h, ok := l.webhook(file, dir, i+1, item); ok {
			out = append(out, h)
		}
	}
	return out
}

func (l *loader) webhook(file, dir string, n int, item *yaml.Node) (model.Webhook, bool) {
	if item.Kind != yaml.MappingNode {
		l.add(file, item.Line, "webhook %d must be a mapping of url, body and after", n)
		return model.Webhook{}, false
	}
	f := l.fields(file, item, webhookFields, nil)
	u := f["url"]
	if u == nil {
		l.add(file, item.Line, "webhook %d needs %q", n, "url")
		return model.Webhook{}, false
	}
	h := model.Webhook{URL: u.Value, Method: string(model.MethodPost), Src: model.Source{File: file, Line: item.Line}}
	l.webhookURL(file, n, u)
	if m := f["method"]; m != nil {
		h.Method = m.Value
		l.checkMethod(file, m.Line, m.Value)
	}
	if a := f["after"]; a != nil {
		h.After = l.duration(file, "after", a)
	}
	if hd := f["headers"]; hd != nil {
		l.webhookHeaders(file, n, hd, &h)
	}
	if b := f["body"]; b != nil {
		h.Body = l.body(file, dir, b)
		if msg := BodyTemplateProblem(h.Body); msg != "" {
			l.add(file, b.Line, "webhook %d body: %s", n, msg)
		}
	}
	if r := f["retries"]; r != nil {
		if err := r.Decode(&h.Retries); err != nil || h.Retries < 0 || h.Retries > maxRetries {
			l.add(file, r.Line, "webhook %d retries must be 0 to %d", n, maxRetries)
		}
	}
	return h, true
}

func (l *loader) webhookURL(file string, n int, u *yaml.Node) {
	if _, err := tmpl.Parse(u.Value); err != nil {
		l.add(file, u.Line, "webhook %d url: %v", n, err)
		return
	}
	if !strings.HasPrefix(u.Value, "http://") && !strings.HasPrefix(u.Value, "https://") && !strings.HasPrefix(u.Value, "{{") {
		l.add(file, u.Line, "webhook %d url %q must start with http:// or https://, or a template like {{ var.hook_url }}", n, u.Value)
	}
}

func (l *loader) webhookHeaders(file string, n int, v *yaml.Node, h *model.Webhook) {
	if err := v.Decode(&h.Headers); err != nil {
		l.add(file, v.Line, "webhook %d headers must map names to values", n)
		return
	}
	for _, name := range slices.Sorted(maps.Keys(h.Headers)) {
		if _, err := tmpl.Parse(h.Headers[name]); err != nil {
			l.add(file, v.Line, "webhook %d header %s: %v", n, name, err)
		}
	}
}
