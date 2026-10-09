package config_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func loadDir(t *testing.T, dir string) *model.Project {
	t.Helper()
	p, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 {
		t.Fatalf("Load = %v, %v", probs, err)
	}
	return p
}

func addState(t *testing.T, dir, route string, st config.NewState) config.AddedState {
	t.Helper()
	added, err := config.AddState(dir, route, st)
	if err != nil {
		t.Fatalf("AddState: %v", err)
	}
	return added
}

func TestAddState_InsertsAfterTheLastStateOfThatRoute(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	added := addState(t, dir, "users.list", config.NewState{
		Name: "server_error", Status: 500, Body: json.RawMessage(`{"error":"boom"}`),
	})
	want := strings.Replace(usersFile, "      status: 401\n",
		"      status: 401\n    server_error:\n      status: 500\n      body: {error: boom}\n", 1)
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, want)
	}
	if added != (config.AddedState{File: "routes/users.yaml", Line: 14}) {
		t.Errorf("AddState = %+v", added)
	}
}

func TestAddState_LastRouteInTheFile(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	addState(t, dir, "users.get", config.NewState{Name: "gone"})
	if got, want := routeFile(t, dir, "users.yaml"), usersFile+"    gone: {}\n"; got != want {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddState_AllFields(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	addState(t, dir, "users.get", config.NewState{
		Name:      "slow",
		Status:    503,
		Headers:   map[string]string{"X-B": "b", "Retry-After": "5"},
		Latency:   1500 * time.Millisecond,
		Generated: true,
		Body:      json.RawMessage(`{"error":"try later"}`),
	})
	want := usersFile + "    slow:\n      status: 503\n      headers: {Retry-After: \"5\", X-B: b}\n" +
		"      latency: 1.5s\n      generated: true\n      body: {error: try later}\n"
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddState_LongBodiesAreWrittenAsBlocks(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	body := `{"users":[{"id":"u_1","name":"Ada Obi","email":"ada@example.com"},{"id":"u_2","name":"Tunde Bello"}],"nextPage":null}`
	addState(t, dir, "users.get", config.NewState{Name: "many", Body: json.RawMessage(body)})
	want := usersFile + `    many:
      body:
        users:
          - id: u_1
            name: Ada Obi
            email: ada@example.com
          - id: u_2
            name: Tunde Bello
        nextPage: null
`
	if got := routeFile(t, dir, "users.yaml"); got != want {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddState_BodiesRoundTrip(t *testing.T) {
	t.Parallel()

	bodies := []string{
		`{"z":1,"a":"true","n":null,"f":1.5,"s":"a: b","list":[1,"2",false],"empty":{},"none":[]}`,
		`[{"id":"007"},{"id":"#tag"}]`,
		`{"quote":"it's \"quoted\"","multi":"line one\nline two","unicode":"café ✓"}`,
	}
	for _, body := range bodies {
		dir := project(t, map[string]string{"users.yaml": usersFile})
		addState(t, dir, "users.get", config.NewState{Name: "sample", Body: json.RawMessage(body)})
		p := loadDir(t, dir)
		r, _ := p.Route("users.get")
		st, _ := r.States.Get("sample")
		if string(st.Body.Data) != body {
			t.Errorf("body after a round trip:\n%s\nwant:\n%s", st.Body.Data, body)
		}
	}
}

func TestAddState_LargeBodiesGoToAFile(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	body := `{"users":["` + strings.Repeat("x", 5000) + `"]}`
	added := addState(t, dir, "users.list", config.NewState{Name: "huge", Body: json.RawMessage(body)})
	if added.BodyFile != "routes/users/list.huge.json" {
		t.Errorf("BodyFile = %q", added.BodyFile)
	}
	if got := routeFile(t, dir, "users.yaml"); !strings.Contains(got, "    huge:\n      body: list.huge.json\n\nget:") {
		t.Errorf("users.yaml:\n%s", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, "routes", "users", "list.huge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("{\n  \"users\": [\n")) || !bytes.HasSuffix(data, []byte("}\n")) {
		t.Errorf("body file isn't indented JSON with a final newline: %.40q…", data)
	}
	r, _ := loadDir(t, dir).Route("users.list")
	st, _ := r.States.Get("huge")
	var compact bytes.Buffer
	_ = json.Compact(&compact, st.Body.Data)
	if compact.String() != body {
		t.Error("the body file doesn't hold the body")
	}
}

func TestAddState_BodyFileAlreadyExists(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile, "users/list.huge.json": "{}"})
	body := `{"users":["` + strings.Repeat("x", 5000) + `"]}`
	_, err := config.AddState(dir, "users.list", config.NewState{Name: "huge", Body: json.RawMessage(body)})
	if err == nil || err.Error() != "routes/users/list.huge.json already exists; pick another state name" {
		t.Errorf("error = %v", err)
	}
	if got := routeFile(t, dir, "users.yaml"); got != usersFile {
		t.Errorf("users.yaml changed:\n%s", got)
	}
}

func TestAddState_NullBodyMeansNoBody(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	addState(t, dir, "users.get", config.NewState{Name: "gone", Status: 410, Body: json.RawMessage(`null`)})
	if got, want := routeFile(t, dir, "users.yaml"), usersFile+"    gone:\n      status: 410\n"; got != want {
		t.Errorf("users.yaml:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddState_FileStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, file, want string
	}{
		{
			"no final newline",
			"get:\n  route: GET /h\n  states:\n    up: {}",
			"get:\n  route: GET /h\n  states:\n    up: {}\n    down:\n      status: 503\n",
		},
		{
			"CRLF line endings",
			"get:\r\n  route: GET /h\r\n  states:\r\n    up: {}\r\n\r\nput:\r\n  route: PUT /h\r\n  states:\r\n    ok: {}\r\n",
			"get:\r\n  route: GET /h\r\n  states:\r\n    up: {}\r\n    down:\r\n      status: 503\r\n\r\nput:\r\n  route: PUT /h\r\n  states:\r\n    ok: {}\r\n",
		},
		{
			"four-space indentation",
			"get:\n    route: GET /h\n    states:\n        up:\n            status: 200\n",
			"get:\n    route: GET /h\n    states:\n        up:\n            status: 200\n        down:\n            status: 503\n",
		},
		{
			"comments and blank lines inside the last state",
			"get:\n  route: GET /h\n  states:\n    up:\n\n      # healthy\n      status: 200\n    # more soon\nput:\n  route: PUT /h\n  states:\n    ok: {}\n",
			"get:\n  route: GET /h\n  states:\n    up:\n\n      # healthy\n      status: 200\n    down:\n      status: 503\n    # more soon\nput:\n  route: PUT /h\n  states:\n    ok: {}\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := project(t, map[string]string{"h.yaml": tc.file})
			addState(t, dir, "h.get", config.NewState{Name: "down", Status: 503})
			if got := routeFile(t, dir, "h.yaml"); got != tc.want {
				t.Errorf("h.yaml:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestAddState_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		route string
		state config.NewState
		want  string
	}{
		{"unknown route", "users.lst", config.NewState{Name: "x"}, `no route "users.lst" (did you mean "users.list"?)`},
		{"state exists", "users.list", config.NewState{Name: "empty"}, `route users.list already has a state "empty"`},
		{"bad name", "users.list", config.NewState{Name: "Server Error"}, `state "Server Error" must use lowercase letters, digits and underscores, starting with a letter`},
		{"YAML keyword", "users.list", config.NewState{Name: "no"}, `state "no" is a YAML keyword; pick another name`},
		{"bad status", "users.list", config.NewState{Name: "odd", Status: 999}, "the new state would break routes/users.yaml:\n  routes/users.yaml:15: status 999 isn't an HTTP status code (100-599)"},
		{"latency too long", "users.list", config.NewState{Name: "stuck", Latency: 2 * time.Minute}, "the new state would break routes/users.yaml:\n  routes/users.yaml:15: latency \"2m0s\" is over the 1m limit"},
		{"text body", "users.list", config.NewState{Name: "text", Body: json.RawMessage(`"hello"`)}, "body must be a JSON object or array"},
		{"invalid body", "users.list", config.NewState{Name: "bad", Body: json.RawMessage(`{"a":`)}, "body isn't valid JSON: unexpected end of JSON input"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := project(t, map[string]string{"users.yaml": usersFile})
			_, err := config.AddState(dir, tc.route, tc.state)
			if err == nil || err.Error() != tc.want {
				t.Errorf("AddState error = %v\nwant: %s", err, tc.want)
			}
			if got := routeFile(t, dir, "users.yaml"); got != usersFile {
				t.Errorf("users.yaml changed after a refused add:\n%s", got)
			}
		})
	}
}

func TestAddState_UnknownRouteIsALookupError(t *testing.T) {
	t.Parallel()

	_, err := config.AddState(project(t, map[string]string{"users.yaml": usersFile}), "nope", config.NewState{Name: "x"})
	if !errors.Is(err, config.ErrUnknownRoute) {
		t.Errorf("error = %v, want ErrUnknownRoute", err)
	}
}

func TestAddState_OneLineStates(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"get:\n  route: GET /h\n  states: { up: {} }\n",
		"get: { route: GET /h, states: { up: {} } }\n",
	} {
		dir := project(t, map[string]string{"h.yaml": file})
		_, err := config.AddState(dir, "h.get", config.NewState{Name: "down"})
		if err == nil || err.Error() != "route h.get has its states on one line; add the state by hand" {
			t.Errorf("%q: error = %v", file, err)
		}
		if got := routeFile(t, dir, "h.yaml"); got != file {
			t.Errorf("h.yaml changed: %q", got)
		}
	}
}

func TestAddState_FileWithProblems(t *testing.T) {
	t.Parallel()

	broken := "get:\n  route: get /h\n  states:\n    up: {}\n"
	dir := project(t, map[string]string{"h.yaml": broken})
	_, err := config.AddState(dir, "h.get", config.NewState{Name: "down"})
	if err == nil || !strings.HasPrefix(err.Error(), "routes/h.yaml has problems; fix them first:\n  routes/h.yaml:2: ") {
		t.Errorf("error = %v", err)
	}
}

func TestAddState_MissingProject(t *testing.T) {
	t.Parallel()

	_, err := config.AddState(filepath.Join(t.TempDir(), "nope"), "h.get", config.NewState{Name: "down"})
	if err == nil || !strings.Contains(err.Error(), "can't read project folder") {
		t.Errorf("error = %v", err)
	}
}
