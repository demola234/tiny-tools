package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/demola234/tiny-tools/mock_machina/internal/match"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tmpl"
)

const normalClosure = 1000

type message struct {
	data []byte
	text bool
	json *tmpl.JSON
	tpl  *tmpl.Text
}

func (m message) render(env tmpl.Env) []byte {
	switch {
	case m.json != nil:
		return m.json.Render(env)
	case m.tpl != nil:
		return []byte(fmt.Sprint(m.tpl.Eval(env)))
	}
	return m.data
}

type reply struct {
	when []model.Condition
	send []message
}

type stream struct {
	send, push []message
	replies    []reply
	every      time.Duration
	close      *model.Close
}

func compileStreams(rt *model.Route) (map[string]*stream, error) {
	if !rt.Method.Stream() && rt.Method != model.MethodGRPC {
		return nil, nil
	}
	out := map[string]*stream{}
	for _, ns := range rt.States {
		s := ns.State.Stream
		if s == nil {
			continue
		}
		cs := &stream{every: s.Every, close: s.Close}
		var err error
		if cs.send, err = compileMessages(s.Send, ns.State.Verbatim); err != nil {
			return nil, fmt.Errorf("route %s state %s: %w", rt.ID, ns.Name, err)
		}
		if cs.push, err = compileMessages(s.Push, ns.State.Verbatim); err != nil {
			return nil, fmt.Errorf("route %s state %s: %w", rt.ID, ns.Name, err)
		}
		for _, r := range s.Reply {
			send, err := compileMessages(r.Send, ns.State.Verbatim)
			if err != nil {
				return nil, fmt.Errorf("route %s state %s: %w", rt.ID, ns.Name, err)
			}
			cs.replies = append(cs.replies, reply{when: r.When, send: send})
		}
		out[ns.Name] = cs
	}
	return out, nil
}

func compileMessages(ms []model.Message, verbatim bool) ([]message, error) {
	out := make([]message, 0, len(ms))
	for _, m := range ms {
		c, err := compileMessage(m, verbatim)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func compileMessage(m model.Message, verbatim bool) (message, error) {
	c := message{data: m.Data, text: m.Text}
	switch {
	case verbatim || !tmpl.Has(m.Data):
		return c, nil
	case m.Text:
		t, err := tmpl.Parse(string(m.Data))
		c.tpl = &t
		return c, err
	}
	j, err := tmpl.ParseJSON(m.Data)
	c.json = &j
	return c, err
}

func opensStream(rt *model.Route, st *model.State) bool {
	return rt.Method.Stream() && st.EffectiveStatus() < http.StatusBadRequest && st.Fault == nil
}

type sender func(ctx context.Context, data []byte) error

func (x *exchange) messageEnv(n int64, msg any) tmpl.Env {
	f := x.facts()
	f.Message = msg
	return tmpl.Env{
		Facts: f, Rand: x.opts.Seed.Stream(x.rt.ID, "stream", strconv.Itoa(x.call), strconv.FormatInt(n, 10)),
		Now: x.opts.Clock.Now(), Locale: x.opts.Locale,
	}
}

func (x *exchange) schedule(ctx context.Context, s *stream, send sender, counter *atomic.Int64) error {
	for _, m := range s.send {
		if err := send(ctx, m.render(x.messageEnv(counter.Add(1), nil))); err != nil {
			return err
		}
	}
	start := x.opts.Clock.Now()
	for i := 0; ; i++ {
		next, closing := nextEvent(s, i)
		wait := next - x.opts.Clock.Now().Sub(start)
		if err := x.opts.Clock.Sleep(ctx, max(wait, 0)); err != nil {
			return err
		}
		if closing {
			return errClosing
		}
		m := s.push[i%len(s.push)]
		if err := send(ctx, m.render(x.messageEnv(counter.Add(1), nil))); err != nil {
			return err
		}
	}
}

var errClosing = errors.New("closing")

const forever = time.Duration(1<<63 - 1)

func nextEvent(s *stream, pushed int) (time.Duration, bool) {
	nextPush := forever
	if s.every > 0 && len(s.push) > 0 {
		nextPush = s.every * time.Duration(pushed+1)
	}
	if s.close != nil && s.close.After <= nextPush {
		return s.close.After, true
	}
	return nextPush, false
}

func (x *exchange) serveSSE(ctx context.Context, w http.ResponseWriter, s *stream, rec *Request) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	rc := http.NewResponseController(w)
	w.WriteHeader(http.StatusOK)
	_ = rc.Flush()
	rec.Status = http.StatusOK
	x.report(*rec)
	send := func(_ context.Context, data []byte) error {
		var b strings.Builder
		for line := range strings.SplitSeq(string(data), "\n") {
			b.WriteString("data: " + line + "\n")
		}
		b.WriteString("\n")
		if _, err := w.Write([]byte(b.String())); err != nil {
			return err
		}
		return rc.Flush()
	}
	var n atomic.Int64
	err := x.schedule(ctx, s, send, &n)
	if errors.Is(err, errClosing) && s.close != nil && s.close.Drop {
		hangUp(w, false)
	}
}

func (x *exchange) serveWS(parent context.Context, w http.ResponseWriter, s *stream, rec *Request) {
	opts := &websocket.AcceptOptions{InsecureSkipVerify: !x.opts.DisableCORS}
	conn, err := websocket.Accept(w, x.req, opts)
	if err != nil {
		return
	}
	rec.Status = http.StatusSwitchingProtocols
	x.report(*rec)
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	defer cancel()
	var n atomic.Int64
	send := func(ctx context.Context, data []byte) error {
		return conn.Write(ctx, websocket.MessageText, data)
	}
	go x.answer(ctx, cancel, conn, s, send, &n)
	err = x.schedule(ctx, s, send, &n)
	switch {
	case errors.Is(err, errClosing) && s.close.Drop:
		_ = conn.CloseNow()
	case errors.Is(err, errClosing):
		code := s.close.Code
		if code == 0 {
			code = normalClosure
		}
		_ = conn.Close(websocket.StatusCode(code), s.close.Reason)
	default:
		_ = conn.CloseNow()
	}
}

func (x *exchange) answer(ctx context.Context, done context.CancelFunc, conn *websocket.Conn, s *stream, send sender, n *atomic.Int64) {
	defer done()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		msg := decodeMessage(data)
		r, ok := firstReply(s.replies, msg)
		if !ok {
			continue
		}
		for _, m := range r.send {
			if send(ctx, m.render(x.messageEnv(n.Add(1), msg))) != nil {
				return
			}
		}
	}
}

func decodeMessage(data []byte) any {
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return string(data)
	}
	if _, isString := decoded.(string); isString {
		return string(data)
	}
	return decoded
}

func firstReply(replies []reply, msg any) (reply, bool) {
	for _, r := range replies {
		if matches(r.when, msg) {
			return r, true
		}
	}
	return reply{}, false
}

func matches(when []model.Condition, msg any) bool {
	f := match.Facts{Message: msg}
	for _, c := range when {
		if !match.Condition(c, f) {
			return false
		}
	}
	return true
}

func (x *exchange) open(ctx context.Context, w http.ResponseWriter, st *model.State, t *templates, s *stream, rec *Request) {
	rec.Method = string(x.rt.Method)
	if len(st.Set) > 0 {
		x.assign(st.Set, t)
	}
	x.facts()
	if s == nil {
		s = &stream{}
	}
	if !x.opts.NoMockHeaders {
		w.Header().Set(stateHeader, rec.State)
		w.Header().Set(routeHeader, x.rt.ID)
	}
	if x.rt.Method == model.MethodWS {
		x.serveWS(ctx, w, s, rec)
		return
	}
	x.serveSSE(ctx, w, s, rec)
}
