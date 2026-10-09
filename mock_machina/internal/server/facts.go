package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"

	"github.com/demola234/tiny-tools/mock_machina/internal/match"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const maxBody = 1 << 20

var pathParam = regexp.MustCompile(`\{([^/{}]+)\}`)

func readsBody(rt *model.Route) bool {
	return slices.ContainsFunc(rt.Rules, func(r model.Rule) bool {
		return slices.ContainsFunc(r.When, func(c model.Condition) bool { return c.Scope == model.ScopeBody })
	})
}

func requestFacts(rt *model.Route, req *http.Request, call int, body []byte) match.Facts {
	f := match.Facts{
		Path:    map[string]string{},
		Query:   req.URL.Query(),
		Header:  req.Header,
		Cookies: map[string]string{},
		Call:    call,
	}
	for _, m := range pathParam.FindAllStringSubmatch(rt.Path, -1) {
		f.Path[m[1]] = req.PathValue(m[1])
	}
	for _, c := range req.Cookies() {
		f.Cookies[c.Name] = c.Value
	}
	if body != nil {
		_ = json.Unmarshal(body, &f.Body)
	}
	return f
}
