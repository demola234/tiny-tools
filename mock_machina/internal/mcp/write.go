package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

type newState struct {
	Name    string            `json:"name" jsonschema:"state name: lowercase letters, digits and underscores, such as not_found"`
	Status  int               `json:"status,omitempty" jsonschema:"HTTP status code; 200 when left out"`
	Headers map[string]string `json:"headers,omitempty" jsonschema:"response headers, such as {\"Retry-After\": \"60\"}"`
	Latency string            `json:"latency,omitempty" jsonschema:"delay before responding, such as 800ms or 2s, at most 1m"`
	Body    json.RawMessage   `json:"body,omitempty" jsonschema:"the JSON response body: an object or array, in the order fields should appear"`
}

type addStateIn struct {
	Route string `json:"route" jsonschema:"route id, such as users.get"`
	newState
}

type addStateOut struct {
	Route    string `json:"route"`
	State    string `json:"state"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	BodyFile string `json:"bodyFile,omitempty" jsonschema:"file the body was written to, for bodies over 4 KB"`
}

type addRouteIn struct {
	Method  string     `json:"method" jsonschema:"HTTP method in capitals, such as GET"`
	Path    string     `json:"path" jsonschema:"path with {parameters}, such as /users/{id}"`
	Name    string     `json:"name,omitempty" jsonschema:"route name within its resource; picked from the method and path when left out (GET /users/{id} is users.get)"`
	Summary string     `json:"summary,omitempty" jsonschema:"one line saying what the route returns"`
	States  []newState `json:"states" jsonschema:"the responses the route can give; the first is served by default"`
}

type addRouteOut struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	CreatedFile bool   `json:"createdFile"`
}

type setStateIn struct {
	Route string `json:"route" jsonschema:"route id, such as users.get"`
	State string `json:"state" jsonschema:"the state to serve by default"`
}

type setStateOut struct {
	Route    string `json:"route"`
	Previous string `json:"previous"`
	Current  string `json:"current"`
}

func addWriteTools(s *sdk.Server, t tools) {
	addRaw(s, &sdk.Tool{
		Name: "add_state",
		Description: "Add a state (a response) to an existing route, such as a 404, an empty list or a slow response. " +
			"The state is marked generated: true until a person reviews it.",
		Annotations: adds("Add a state", false),
	}, t.addState)
	addRaw(s, &sdk.Tool{
		Name: "add_route",
		Description: "Add a route with its states to routes/<resource>.yaml, creating the file if needed. " +
			"The route is marked generated: true and status: draft until a person reviews it.",
		Annotations: adds("Add a route", false),
	}, t.addRoute)
	addRaw(s, &sdk.Tool{
		Name:        "set_state",
		Description: "Choose which state a route serves by default, for everyone using the mock.",
		Annotations: adds("Set the default state", true),
	}, t.setState)
}

func adds(title string, idempotent bool) *sdk.ToolAnnotations {
	no := false
	return &sdk.ToolAnnotations{Title: title, IdempotentHint: idempotent, DestructiveHint: &no, OpenWorldHint: &no}
}

var schemaOptions = &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[json.RawMessage](): {Types: []string{"object", "array"}},
}}

func addRaw[In, Out any](s *sdk.Server, tool *sdk.Tool, h func(context.Context, In) (Out, string, error)) {
	in, err := jsonschema.For[In](schemaOptions)
	if err != nil {
		panic(err)
	}
	out, err := jsonschema.For[Out](schemaOptions)
	if err != nil {
		panic(err)
	}
	tool.InputSchema, tool.OutputSchema = in, out
	s.AddTool(tool, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		var args In
		raw := req.Params.Arguments
		if len(raw) == 0 {
			raw = json.RawMessage("{}")
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&args); err != nil {
			return failed(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		result, msg, err := h(ctx, args)
		if err != nil {
			return failed(err), nil
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: msg}}, StructuredContent: result}, nil
	})
}

func failed(err error) *sdk.CallToolResult {
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: err.Error()}}}
}

func (s newState) config() (config.NewState, error) {
	st := config.NewState{Name: s.Name, Status: s.Status, Headers: s.Headers, Body: s.Body, Generated: true}
	if s.Latency != "" {
		d, err := time.ParseDuration(s.Latency)
		if err != nil {
			return config.NewState{}, fmt.Errorf("latency %q isn't a duration, like 250ms or 2s", s.Latency)
		}
		st.Latency = d
	}
	return st, nil
}

func (t tools) addState(_ context.Context, in addStateIn) (addStateOut, string, error) {
	st, err := in.config()
	if err != nil {
		return addStateOut{}, "", err
	}
	added, err := config.AddState(t.dir, in.Route, st)
	if err != nil {
		return addStateOut{}, "", err
	}
	out := addStateOut{Route: in.Route, State: in.Name, File: added.File, Line: added.Line, BodyFile: added.BodyFile}
	msg := fmt.Sprintf("added state %q to %s at %s:%d, marked generated: true until a person reviews it", in.Name, in.Route, added.File, added.Line)
	return out, msg, nil
}

func (t tools) addRoute(_ context.Context, in addRouteIn) (addRouteOut, string, error) {
	if len(in.States) == 0 {
		return addRouteOut{}, "", errors.New("states needs at least one state")
	}
	opts := config.AddOptions{Name: in.Name, Summary: in.Summary, Generated: true}
	for _, s := range in.States {
		st, err := s.config()
		if err != nil {
			return addRouteOut{}, "", err
		}
		st.Generated = false
		opts.States = append(opts.States, st)
	}
	added, err := config.AddRoute(t.dir, in.Method, in.Path, opts)
	if err != nil {
		return addRouteOut{}, "", err
	}
	where := "in " + added.File
	if added.CreatedFile {
		where = "in a new file, " + added.File
	}
	msg := fmt.Sprintf("added %s (%s %s) %s, marked generated: true and status: draft until a person reviews it",
		added.ID, in.Method, in.Path, where)
	return addRouteOut{ID: added.ID, File: added.File, CreatedFile: added.CreatedFile}, msg, nil
}

func (t tools) setState(_ context.Context, in setStateIn) (setStateOut, string, error) {
	previous, err := config.SetActive(t.dir, in.Route, in.State)
	if err != nil {
		return setStateOut{}, "", err
	}
	msg := fmt.Sprintf("%s now serves %q (was %q)", in.Route, in.State, previous)
	if previous == in.State {
		msg = fmt.Sprintf("%s already serves %q", in.Route, in.State)
	}
	return setStateOut{Route: in.Route, Previous: previous, Current: in.State}, msg, nil
}
