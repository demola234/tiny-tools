package mcp

import (
	"fmt"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type Options struct {
	Dir         string
	Version     string
	ReadOnly    bool
	LiveHeaders http.Header
}

const instructions = `MockMachina serves a mock HTTP API from contract files in a .mockmachina folder.
Each file in routes/ holds one resource (routes/users.yaml) with named routes (users.list, users.get).
A route has states: the different responses it can give, such as success, empty, not_found or server_error.
Apps pick a state per request with the X-Mock-State header; otherwise the route's active state is served.
Read the contract with list_routes and get_route, and check it with lint.
diff shows what changed between git refs; diff_live compares the contract with a running API.
add_route and add_state write new routes and states, marked generated: true so a person reviews them; set_state changes the default state.
Before writing, show the person what you plan to add.`

func New(opts Options) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "mockmachina", Version: opts.Version}, &sdk.ServerOptions{Instructions: instructions})
	t := tools{dir: opts.Dir, liveHeaders: opts.LiveHeaders}
	sdk.AddTool(s, &sdk.Tool{
		Name:        "list_routes",
		Description: "List the contract's routes with their states and which state each serves by default.",
		Annotations: reads("List routes", false),
	}, t.listRoutes)
	sdk.AddTool(s, &sdk.Tool{
		Name:        "get_route",
		Description: "Show one route in full: every state's status, headers, latency and body, and where it's defined.",
		Annotations: reads("Get a route", false),
	}, t.getRoute)
	sdk.AddTool(s, &sdk.Tool{
		Name:        "lint",
		Description: "Check the contract for mistakes. Errors stop the mock from serving a file; warnings are suggestions.",
		Annotations: reads("Lint the contract", false),
	}, t.lint)
	addDiffTools(s, t)
	addPrompts(s, opts.ReadOnly)
	if !opts.ReadOnly {
		addWriteTools(s, t)
	}
	return s
}

func reads(title string, openWorld bool) *sdk.ToolAnnotations {
	return &sdk.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &openWorld}
}

type tools struct {
	dir         string
	liveHeaders http.Header
}

func (t tools) load() (*model.Project, config.Problems, error) {
	p, probs, err := config.Load(t.dir)
	if err != nil {
		return nil, nil, fmt.Errorf("can't read project folder %s: %w", t.dir, err)
	}
	return p, probs, nil
}
