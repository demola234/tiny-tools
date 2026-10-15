package config_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const streamRoutes = `room:
  route: WS /chat/{room}
  states:
    busy:
      send:
        - { type: welcome, room: "{{ path.room }}" }
        - hello
      reply:
        - when: { message.type: ping }
          send: { type: pong, echo: "{{ message.id }}" }
        - when: { message: bye }
          send: [see you]
      every: 2s
      push:
        - { type: message, from: "{{ fake.person.first_name }}" }
        - { type: typing }
      close: { after: 10s, code: 1011, reason: server restarting }
    dropped:
      send: [{ type: welcome }]
      close: { after: 5s, drop: true }
    unauthorized:
      status: 401
      body: { error: sign in first }
events:
  route: SSE /orders/{id}/events
  states:
    on_the_way:
      send: { status: dispatched }
      every: 1s
      push: { status: out_for_delivery }
      close: { after: 3s }
`

func TestLoadFS_StreamRoutes(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/live.yaml": streamRoutes})
	room, _ := p.Route("live.room")
	if room.Method != model.MethodWS || !room.Method.Stream() {
		t.Fatalf("method = %q", room.Method)
	}
	busy, _ := room.States.Get("busy")
	want := &model.Stream{
		Send: []model.Message{{Data: []byte(`{"type":"welcome","room":"{{ path.room }}"}`)}, {Data: []byte("hello"), Text: true}},
		Reply: []model.Reply{
			{
				When: []model.Condition{{Scope: model.ScopeMessage, Key: "type", Op: model.OpEq, Value: "ping"}},
				Send: []model.Message{{Data: []byte(`{"type":"pong","echo":"{{ message.id }}"}`)}}, Src: model.Source{File: "routes/live.yaml", Line: 9},
			},
			{
				When: []model.Condition{{Scope: model.ScopeMessage, Op: model.OpEq, Value: "bye"}},
				Send: []model.Message{{Data: []byte("see you"), Text: true}}, Src: model.Source{File: "routes/live.yaml", Line: 11},
			},
		},
		Every: 2 * time.Second,
		Push:  []model.Message{{Data: []byte(`{"type":"message","from":"{{ fake.person.first_name }}"}`)}, {Data: []byte(`{"type":"typing"}`)}},
		Close: &model.Close{After: 10 * time.Second, Code: 1011, Reason: "server restarting"},
	}
	if diff := cmp.Diff(want, busy.Stream, cmpopts.IgnoreFields(model.Condition{}, "Pattern")); diff != "" {
		t.Errorf("busy stream (-want +got):\n%s", diff)
	}
	if d, _ := room.States.Get("dropped"); d.Stream.Close == nil || !d.Stream.Close.Drop {
		t.Errorf("dropped close = %+v", d.Stream.Close)
	}
	if u, _ := room.States.Get("unauthorized"); u.Stream != nil || u.Status != 401 {
		t.Errorf("a refusing state has no stream: %+v", u)
	}
	events, _ := p.Route("live.events")
	if s, _ := events.States.Get("on_the_way"); events.Method != model.MethodSSE || len(s.Stream.Send) != 1 || s.Stream.Close.After != 3*time.Second {
		t.Errorf("sse = %+v", s.Stream)
	}
}

func TestLoadFS_StreamProblems(t *testing.T) {
	t.Parallel()

	ws := func(state string) string { return "room:\n  route: WS /chat\n  states:\n    s:\n" + state }
	sse := func(state string) string { return "ev:\n  route: SSE /events\n  states:\n    s:\n" + state }
	tests := []struct{ name, file, want string }{
		{"lowercase method", "room:\n  route: ws /chat\n  states:\n    s: {}\n", `routes/live.yaml:2: method "ws" must be uppercase: WS`},
		{"every without push", ws("      every: 1s\n"), `routes/live.yaml:5: every needs push: the messages to send each time`},
		{"push without every", ws("      push: hi\n"), `routes/live.yaml:5: push needs every, like every: 2s`},
		{"every zero", ws("      every: 0s\n      push: hi\n"), `routes/live.yaml:5: every must be more than 0s`},
		{"reply on sse", sse("      reply:\n        - when: { message: hi }\n          send: hi\n"), `routes/live.yaml:6: reply only works on WS routes; an SSE client can't send messages`},
		{"reply not a list", ws("      reply: { when: {}, send: hi }\n"), `routes/live.yaml:5: reply must be a list of { when, send }`},
		{"reply without send", ws("      reply:\n        - when: { message: hi }\n"), `routes/live.yaml:6: reply 1 needs "send"`},
		{"reply selector on request", ws("      reply:\n        - when: { body.x: 1 }\n          send: hi\n"), `routes/live.yaml:6: reply 1 matches the client's message; use message or message.<field>, not body.x`},
		{"message selector in a route rule", "room:\n  route: WS /chat\n  rules:\n    - when: { message.type: x }\n      state: s\n  states:\n    s: {}\n", `routes/live.yaml:4: message.type only works in a reply; rules run before any message arrives`},
		{"stream fields on http", "get:\n  route: GET /x\n  states:\n    s:\n      send: hi\n", `routes/live.yaml:5: send only works on WS and SSE routes`},
		{"body on a stream state", ws("      body: { a: 1 }\n"), `routes/live.yaml:5: body is for refusing the connection; add status 400 or more, or use send`},
		{"close code out of range", ws("      close: { after: 1s, code: 99 }\n"), `routes/live.yaml:5: close code 99 isn't valid; use 1000-1015 or 4000-4999`},
		{"close code on sse", sse("      close: { after: 1s, code: 1011 }\n"), `routes/live.yaml:5: close code and reason only work on WS routes`},
		{"bad message template", ws("      send: { a: \"{{ nope }}\" }\n"), `routes/live.yaml:5: send 1: unknown template "nope" (did you mean "now"?)`},
		{"same path as get", "a:\n  route: GET /chat\n  states:\n    s: {}\nb:\n  route: WS /chat\n  states:\n    s: {}\n", `routes/live.yaml:5: GET /chat is already defined by route "live.a" (WS and SSE routes are served on GET)`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, map[string]string{"routes/live.yaml": tc.file})
			if got := probs.String(); got != tc.want {
				t.Errorf("problems:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
