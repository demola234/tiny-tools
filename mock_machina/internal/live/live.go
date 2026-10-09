package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	defaultTimeout     = 10 * time.Second
	defaultConcurrency = 4
	maxBody            = 1 << 20
	jsonType           = "application/json"
)

var (
	safeMethods = []model.Method{model.MethodGet, model.MethodHead, model.MethodOptions}
	pathParam   = regexp.MustCompile(`\{([^/{}]+)\}`)
)

type Options struct {
	BaseURL       *url.URL
	Headers       http.Header
	Params        map[string]string
	IncludeWrites bool
	Timeout       time.Duration
	Concurrency   int
}

func Check(ctx context.Context, p *model.Project, opts Options) []diff.Change {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = defaultConcurrency
	}
	client := &http.Client{Timeout: opts.Timeout}
	results := make([][]diff.Change, len(p.Routes))
	slots := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	for i, r := range p.Routes {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i] = checkRoute(ctx, client, r, opts)
		})
	}
	wg.Wait()
	all := slices.Concat(results...)
	diff.Sort(all)
	return all
}

type routeCheck struct {
	route *model.Route
	out   []diff.Change
}

func (c *routeCheck) add(s diff.Severity, state, format string, args ...any) {
	c.out = append(c.out, diff.Change{
		Severity: s, Route: c.route.ID, State: state, Message: fmt.Sprintf(format, args...), Src: c.route.Src,
	})
}

func checkRoute(ctx context.Context, client *http.Client, r *model.Route, opts Options) []diff.Change {
	c := &routeCheck{route: r}
	if r.Method == model.MethodCRUD {
		c.add(diff.Info, "", "not checked: a CRUD route is a whole simulated collection, not one request")
		return c.out
	}
	if !opts.IncludeWrites && !slices.Contains(safeMethods, r.Method) {
		c.add(diff.Info, "", "not checked: %s isn't sent to a live API without --include-writes", r.Method)
		return c.out
	}
	target, missing := targetURL(opts.BaseURL, r, opts.Params)
	if missing != "" {
		c.add(diff.Info, "", "not checked: no example value for {%s} (add examples: { %s: ... })", missing, missing)
		return c.out
	}
	res, err := fetch(ctx, client, string(r.Method), target, opts.Headers)
	if err != nil {
		c.add(diff.Breaking, "", "request failed: %v", err)
		return c.out
	}
	c.compare(res)
	return c.out
}

func targetURL(base *url.URL, r *model.Route, params map[string]string) (target, missing string) {
	filled := pathParam.ReplaceAllStringFunc(r.Path, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := params[name]; ok {
			return url.PathEscape(v)
		}
		if v, ok := r.Examples[name]; ok {
			return url.PathEscape(v)
		}
		if missing == "" {
			missing = name
		}
		return m
	})
	return strings.TrimSuffix(base.String(), "/") + filled, missing
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func fetch(ctx context.Context, client *http.Client, method, target string, headers http.Header) (response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return response{}, err
	}
	for k, values := range headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	res, err := client.Do(req)
	if err != nil {
		return response{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	return response{res.StatusCode, res.Header, body}, err
}

func (c *routeCheck) compare(res response) {
	name, st := pickState(c.route, res.status)
	if st == nil {
		c.add(diff.Breaking, "", "live API returned %d; no state returns it (states return %s)", res.status, statuses(c.route))
		return
	}
	want, got := mediaType(st.Body.ContentType), mediaType(res.header.Get("Content-Type"))
	sameType := want == "" || want == got
	if !sameType {
		c.add(diff.Breaking, name, "state %q returns %s, live API returned %s", name, want, got)
	}
	for _, header := range slices.Sorted(maps.Keys(st.Headers)) {
		if res.header.Get(header) == "" {
			c.add(diff.Warning, name, "state %q sets header %s, live API doesn't", name, header)
		}
	}
	if !sameType || want != jsonType || len(st.Body.Data) == 0 {
		return
	}
	var expected, actual any
	_ = json.Unmarshal(st.Body.Data, &expected)
	if err := json.Unmarshal(res.body, &actual); err != nil {
		c.add(diff.Breaking, name, "state %q: live response isn't valid JSON", name)
		return
	}
	for _, f := range compareShape(expected, actual) {
		c.add(f.severity, name, "state %q: %s", name, f.message)
	}
}

func pickState(r *model.Route, status int) (string, *model.State) {
	var name string
	var found *model.State
	for _, ns := range r.States {
		if ns.State.EffectiveStatus() != status {
			continue
		}
		if ns.Name == r.Active {
			return ns.Name, ns.State
		}
		if found == nil {
			name, found = ns.Name, ns.State
		}
	}
	return name, found
}

func statuses(r *model.Route) string {
	var seen []string
	for _, ns := range r.States {
		if s := strconv.Itoa(ns.State.EffectiveStatus()); !slices.Contains(seen, s) {
			seen = append(seen, s)
		}
	}
	return strings.Join(seen, ", ")
}

func mediaType(contentType string) string {
	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		return mt
	}
	return strings.TrimSpace(contentType)
}
