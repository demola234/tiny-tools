package config

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

var (
	streamFields = []string{"send", "reply", "every", "push", "close"}
	replyFields  = []string{"when", "send"}
	closeFields  = []string{"after", "code", "reason", "drop"}
)

const (
	minCloseCode     = 1000
	maxStandardCode  = 1015
	minAppCloseCode  = 4000
	maxAppCloseCode  = 4999
	refusalStatusMin = 400
)

func (l *loader) stream(file string, method model.Method, f map[string]*yaml.Node, st *model.State) {
	present := false
	for _, name := range streamFields {
		if f[name] == nil {
			continue
		}
		present = true
		if !method.Stream() && method != model.MethodGRPC {
			l.add(file, f[name].Line, "%s only works on WS and SSE routes", name)
			return
		}
	}
	if !method.Stream() && method != model.MethodGRPC {
		return
	}
	if v := f["body"]; v != nil && method.Stream() && st.Status < refusalStatusMin {
		l.add(file, v.Line, "body is for refusing the connection; add status 400 or more, or use send")
	}
	if !present {
		return
	}
	s := &model.Stream{}
	if v := f["send"]; v != nil {
		s.Send = l.messages(file, "send", v, !st.Verbatim)
	}
	if v := f["reply"]; v != nil {
		s.Reply = l.replies(file, method, v, !st.Verbatim)
	}
	l.pushes(file, f, s, !st.Verbatim)
	if v := f["close"]; v != nil {
		s.Close = l.closing(file, method, v)
	}
	st.Stream = s
}

func (l *loader) pushes(file string, f map[string]*yaml.Node, s *model.Stream, templated bool) {
	every, push := f["every"], f["push"]
	switch {
	case every != nil && push == nil:
		l.add(file, every.Line, "every needs push: the messages to send each time")
	case push != nil && every == nil:
		l.add(file, push.Line, "push needs every, like every: 2s")
	case every != nil:
		if every.Value == "0s" || every.Value == "0" {
			l.add(file, every.Line, "every must be more than 0s")
			return
		}
		s.Every = l.duration(file, "every", every)
		s.Push = l.messages(file, "push", push, templated)
	}
}

func (l *loader) messages(file, label string, v *yaml.Node, templated bool) []model.Message {
	items := []*yaml.Node{v}
	if v.Kind == yaml.SequenceNode {
		items = v.Content
	}
	out := make([]model.Message, 0, len(items))
	for i, item := range items {
		m, err := message(item)
		if err == nil && templated {
			err = messageTemplateProblem(m)
		}
		if err != nil {
			l.add(file, v.Line, "%s %d: %v", label, i+1, err)
			continue
		}
		out = append(out, m)
	}
	return out
}

func message(item *yaml.Node) (model.Message, error) {
	if item.Kind == yaml.ScalarNode {
		return model.Message{Data: []byte(item.Value), Text: true}, nil
	}
	data, err := toJSON(item)
	if err != nil {
		return model.Message{}, fmt.Errorf("can't be written as JSON: %w", err)
	}
	return model.Message{Data: data}, nil
}

func messageTemplateProblem(m model.Message) error {
	if !tmpl.Has(m.Data) {
		return nil
	}
	var err error
	if m.Text {
		_, err = tmpl.Parse(string(m.Data))
	} else {
		_, err = tmpl.ParseJSON(m.Data)
	}
	return err
}

func (l *loader) replies(file string, method model.Method, v *yaml.Node, templated bool) []model.Reply {
	switch {
	case method != model.MethodWS:
		l.add(file, v.Line, "reply only works on WS routes; an SSE client can't send messages")
		return nil
	case v.Kind != yaml.SequenceNode:
		l.add(file, v.Line, "reply must be a list of { when, send }")
		return nil
	}
	var out []model.Reply
	for i, item := range v.Content {
		if r, ok := l.reply(file, i+1, item, templated); ok {
			out = append(out, r)
		}
	}
	return out
}

func (l *loader) reply(file string, n int, item *yaml.Node, templated bool) (model.Reply, bool) {
	if item.Kind != yaml.MappingNode {
		l.add(file, item.Line, "reply %d must be a mapping of when and send", n)
		return model.Reply{}, false
	}
	f := l.fields(file, item, replyFields, nil)
	for _, field := range replyFields {
		if f[field] == nil {
			l.add(file, item.Line, "reply %d needs %q", n, field)
			return model.Reply{}, false
		}
	}
	r := model.Reply{Src: model.Source{File: file, Line: item.Line}}
	when := f["when"]
	if when.Kind != yaml.MappingNode || len(when.Content) == 0 {
		l.add(file, when.Line, "reply %d's when must map message selectors to values, like { message.type: ping }", n)
		return model.Reply{}, false
	}
	for key, val := range pairs(when) {
		c, ok := l.messageCondition(file, n, key, val)
		if !ok {
			return model.Reply{}, false
		}
		r.When = append(r.When, c)
	}
	r.Send = l.messages(file, fmt.Sprintf("reply %d send", n), f["send"], templated)
	return r, true
}

func (l *loader) messageCondition(file string, n int, key, val *yaml.Node) (model.Condition, bool) {
	scope, field, _ := strings.Cut(key.Value, ".")
	if model.Scope(scope) != model.ScopeMessage {
		l.add(file, key.Line, "reply %d matches the client's message; use message or message.<field>, not %s", n, key.Value)
		return model.Condition{}, false
	}
	c := model.Condition{Scope: model.ScopeMessage, Key: field}
	if msg := matcher(key.Value, val, &c); msg != "" {
		l.add(file, key.Line, "%s", msg)
		return model.Condition{}, false
	}
	return c, true
}

func (l *loader) closing(file string, method model.Method, v *yaml.Node) *model.Close {
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "close must be a mapping like { after: 5s, code: 1011 }")
		return nil
	}
	f := l.fields(file, v, closeFields, nil)
	c := &model.Close{}
	if a := f["after"]; a != nil {
		c.After = l.duration(file, "close after", a)
	}
	if d := f["drop"]; d != nil {
		c.Drop = l.boolean(file, "drop", d)
	}
	if r := f["reason"]; r != nil {
		c.Reason = r.Value
	}
	if code := f["code"]; code != nil {
		if err := code.Decode(&c.Code); err != nil {
			l.add(file, code.Line, "close code must be a number, like 1011")
		}
	}
	if (f["code"] != nil || f["reason"] != nil) && method != model.MethodWS {
		l.add(file, v.Line, "close code and reason only work on WS routes")
		return c
	}
	if c.Code != 0 && !validCloseCode(c.Code) {
		l.add(file, v.Line, "close code %d isn't valid; use %d-%d or %d-%d", c.Code, minCloseCode, maxStandardCode, minAppCloseCode, maxAppCloseCode)
	}
	return c
}

func validCloseCode(code int) bool {
	return (code >= minCloseCode && code <= maxStandardCode) || (code >= minAppCloseCode && code <= maxAppCloseCode)
}
