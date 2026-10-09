package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

type templates struct {
	json    *tmpl.JSON
	text    *tmpl.Text
	headers map[string]tmpl.Text
	set     map[string]tmpl.Text
}

func (t *templates) setTemplate(name string) (tmpl.Text, bool) {
	if t == nil {
		return tmpl.Text{}, false
	}
	tx, ok := t.set[name]
	return tx, ok
}

func compileStates(rt *model.Route) (map[string]*templates, bool, error) {
	compiled := map[string]*templates{}
	usesBody := false
	for _, ns := range rt.States {
		t, err := compileState(ns.State)
		if err != nil {
			return nil, false, fmt.Errorf("route %s state %s: %w", rt.ID, ns.Name, err)
		}
		if t != nil {
			compiled[ns.Name] = t
			usesBody = usesBody || t.usesBody()
		}
	}
	return compiled, usesBody, nil
}

func compileState(st *model.State) (*templates, error) {
	if st.Verbatim {
		return nil, nil
	}
	t := &templates{headers: map[string]tmpl.Text{}, set: map[string]tmpl.Text{}}
	if err := t.compileSet(st.Set); err != nil {
		return nil, err
	}
	for name, value := range st.Headers {
		if tmpl.Has([]byte(value)) {
			parsed, err := tmpl.Parse(value)
			if err != nil {
				return nil, err
			}
			t.headers[name] = parsed
		}
	}
	if err := t.compileBody(st.Body); err != nil {
		return nil, err
	}
	if t.json == nil && t.text == nil && len(t.headers) == 0 && len(t.set) == 0 {
		return nil, nil
	}
	return t, nil
}

func (t *templates) compileSet(set []model.Assignment) error {
	for _, a := range set {
		if s, ok := a.Value.(string); ok && tmpl.Has([]byte(s)) {
			parsed, err := tmpl.Parse(s)
			if err != nil {
				return err
			}
			t.set[a.Name] = parsed
		}
	}
	return nil
}

func (t *templates) compileBody(b model.Body) error {
	if !tmpl.Has(b.Data) {
		return nil
	}
	switch {
	case strings.HasPrefix(b.ContentType, "application/json"):
		j, err := tmpl.ParseJSON(b.Data)
		t.json = &j
		return err
	case strings.HasPrefix(b.ContentType, "text/"):
		x, err := tmpl.Parse(string(b.Data))
		t.text = &x
		return err
	}
	return nil
}

func (t *templates) usesBody() bool {
	if (t.json != nil && t.json.UsesBody()) || (t.text != nil && t.text.UsesBody()) {
		return true
	}
	for _, h := range t.headers {
		if h.UsesBody() {
			return true
		}
	}
	for _, v := range t.set {
		if v.UsesBody() {
			return true
		}
	}
	return false
}

func (t *templates) apply(h http.Header, body []byte, env tmpl.Env) []byte {
	if t == nil {
		return body
	}
	for name, value := range t.headers {
		h.Set(name, fmt.Sprint(value.Eval(env)))
	}
	switch {
	case t.json != nil:
		return t.json.Render(env)
	case t.text != nil:
		return []byte(fmt.Sprint(t.text.Eval(env)))
	}
	return body
}

func UsesRandomness(p *model.Project) bool {
	for _, rt := range p.Routes {
		if rt.Mode == model.ModeRandom {
			return true
		}
		for _, ns := range rt.States {
			if t, _ := compileState(ns.State); t.usesRandom() || ns.State.Jitter > 0 || (ns.State.Fault != nil && ns.State.Fault.Rate < 1) {
				return true
			}
		}
	}
	return false
}

func (t *templates) usesRandom() bool {
	if t == nil {
		return false
	}
	if (t.json != nil && t.json.UsesRandom()) || (t.text != nil && t.text.UsesRandom()) {
		return true
	}
	for _, h := range t.headers {
		if h.UsesRandom() {
			return true
		}
	}
	return false
}
