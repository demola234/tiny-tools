package diff

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type Severity int

const (
	Breaking Severity = iota
	Warning
	Info
	Safe
)

var severityNames = [...]string{"breaking", "warning", "info", "safe"}

func (s Severity) String() string { return severityNames[s] }

type Change struct {
	Severity Severity
	Route    string
	State    string
	Message  string
	Src      model.Source
	Owners   model.Owners
}

func Compare(base, head *model.Project) []Change {
	c := collector{base: index(base.Schemas), head: index(head.Schemas)}
	baseByID, headByID := byID(base), byID(head)

	added := map[string]*model.Route{}
	for _, h := range head.Routes {
		if baseByID[h.ID] == nil {
			added[h.Key()] = h
		}
	}
	for _, b := range base.Routes {
		h := headByID[b.ID]
		switch {
		case h != nil:
			c.routes(b, h)
		case added[b.Key()] != nil:
			h = added[b.Key()]
			delete(added, b.Key())
			c.add(Warning, h, "", "renamed from %s (%s)", b.ID, request(h))
			c.routes(b, h)
		default:
			c.add(Breaking, b, "", "route removed (%s)", request(b))
		}
	}
	for _, h := range added {
		c.add(Safe, h, "", "route added (%s)", request(h))
	}

	Sort(c.changes)
	return c.changes
}

func Sort(changes []Change) {
	slices.SortStableFunc(changes, func(a, b Change) int {
		return cmp.Or(cmp.Compare(a.Severity, b.Severity), strings.Compare(a.Route, b.Route), strings.Compare(a.Message, b.Message))
	})
}

type collector struct {
	changes    []Change
	base, head schemaIndex
}

func (c Change) Text() string { return c.Route + ": " + c.Message }

func (c *collector) add(s Severity, r *model.Route, state, format string, args ...any) {
	c.changes = append(c.changes, Change{
		Severity: s,
		Route:    r.ID,
		State:    state,
		Message:  fmt.Sprintf(format, args...),
		Src:      r.Src,
		Owners:   r.Owners,
	})
}

func (c *collector) routes(b, h *model.Route) {
	if b.Key() != h.Key() {
		c.add(Breaking, h, "", "now answers %s, was %s", request(h), request(b))
	}
	for _, bs := range b.States {
		hs, ok := h.States.Get(bs.Name)
		if !ok {
			c.add(Breaking, h, bs.Name, "state %q removed", bs.Name)
			continue
		}
		c.states(h, bs.Name, bs.State, hs)
	}
	for _, hs := range h.States {
		if _, ok := b.States.Get(hs.Name); !ok {
			c.add(Safe, h, hs.Name, "state %q added", hs.Name)
		}
	}
	c.metadata(b, h)
	c.schemas(b, h)
}

func (c *collector) states(h *model.Route, name string, b, s *model.State) {
	was, now := b.EffectiveStatus(), s.EffectiveStatus()
	if was != now {
		severity := Warning
		if was/100 != now/100 {
			severity = Breaking
		}
		c.add(severity, h, name, "state %q now returns %d, was %d", name, now, was)
	}
	if b.Body.ContentType != s.Body.ContentType {
		c.add(Breaking, h, name, "state %q now returns %s, was %s", name, contentType(s), contentType(b))
	}
	if was, now := faultText(b.Fault), faultText(s.Fault); was != now {
		if now == "" {
			c.add(Info, h, name, "state %q no longer fails", name)
		} else {
			c.add(Info, h, name, "state %q now fails with %s", name, now)
		}
	}
}

func faultText(f *model.Fault) string {
	if f == nil {
		return ""
	}
	text := string(f.Type)
	if f.Rate < 1 {
		text += fmt.Sprintf(" (rate %g)", f.Rate)
	}
	if f.After > 0 {
		text += " after " + f.After.String()
	}
	return text
}

func (c *collector) metadata(b, h *model.Route) {
	if b.Active != h.Active {
		c.add(Info, h, "", "default state now %q, was %q", h.Active, b.Active)
	}
	if !slices.Equal(b.Owners.Backend, h.Owners.Backend) || !slices.Equal(b.Owners.Frontend, h.Owners.Frontend) {
		c.add(Info, h, "", "owners changed")
	}
	if b.Status.Effective() != h.Status.Effective() {
		c.add(Info, h, "", "status now %s, was %s", h.Status.Effective(), b.Status.Effective())
	}
	if b.Summary != h.Summary {
		c.add(Info, h, "", "summary now %q", h.Summary)
	}
	if was, now := mode(b), mode(h); was != now {
		c.add(Info, h, "", "mode now %s, was %s", now, was)
	}
	if rulesText(b.Rules) != rulesText(h.Rules) {
		c.add(Info, h, "", "rules changed")
	}
}

func mode(r *model.Route) model.Mode {
	if r.Mode == "" {
		return model.ModeActive
	}
	return r.Mode
}

func rulesText(rules []model.Rule) string {
	var b strings.Builder
	for _, r := range rules {
		b.WriteString(r.State)
		for _, c := range r.When {
			fmt.Fprintf(&b, "|%s.%s %s %v %v", c.Scope, c.Key, c.Op, c.Value, c.Values)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func byID(p *model.Project) map[string]*model.Route {
	m := make(map[string]*model.Route, len(p.Routes))
	for _, r := range p.Routes {
		m[r.ID] = r
	}
	return m
}

func request(r *model.Route) string { return string(r.Method) + " " + r.Path }

func contentType(s *model.State) string {
	if s.Body.ContentType == "" {
		return "no body"
	}
	return s.Body.ContentType
}
