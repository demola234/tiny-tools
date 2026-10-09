package cli

import (
	"cmp"
	"context"
	"os"
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

var (
	folderField = tui.Field{Label: "Folder", Flag: "--dir", Hint: "nearest .mockmachina"}
	httpsField  = tui.Field{Label: "HTTPS", Flag: "--https", Toggle: true}
)

func catalog(proj *model.Project) []tui.Action {
	port := strconv.Itoa(model.DefaultConfig().Ports.Mock)
	if proj != nil && proj.Config.Ports.Mock != 0 {
		port = strconv.Itoa(proj.Config.Ports.Mock)
	}
	portField := tui.Field{Label: "Port", Flag: "--port", Default: port}
	initAction := tui.Action{Name: "init", Short: "Create a .mockmachina project here", Args: []string{"init"}, Fields: []tui.Field{
		{Label: "Port", Flag: "--port", Default: strconv.Itoa(model.DefaultConfig().Ports.Mock)},
		{Label: "Skip example", Flag: "--no-example", Toggle: true},
	}}
	actions := []tui.Action{
		{Name: "start", Short: "Serve the mock API with the live screen", Args: []string{"start"}, Fields: []tui.Field{
			portField, httpsField, {Label: "Proxy", Flag: "--proxy", Hint: "backend URL for requests no route matches"}, folderField,
		}},
		{Name: "start --plain", Short: "Serve the mock API, printing a log line per request", Args: []string{"start", "--plain"}, Fields: []tui.Field{portField, httpsField, folderField}},
		{Name: "tui", Short: "The live screen, the same as start in a terminal", Args: []string{"tui"}, Fields: []tui.Field{portField, httpsField, folderField}},
		{Name: "state set", Short: "Switch the state a route serves", Args: []string{"state", "set"}, Fields: []tui.Field{stateField(proj)}},
		{Name: "state list", Short: "Show routes and their states", Args: []string{"state", "list"}},
		{Name: "add", Short: "Add a route", Args: []string{"add"}, Fields: []tui.Field{
			{Label: "Method", Choices: []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
			{Label: "Path", Required: true, Hint: "/orders/{id}"},
			{Label: "Summary", Flag: "--summary", Hint: "one line saying what it returns"},
			{Label: "Name", Flag: "--name", Hint: "picked from the method"},
		}},
		{Name: "lint", Short: "Check .mockmachina/ for mistakes", Args: []string{"lint"}, Fields: []tui.Field{
			{Label: "Fail on warnings", Flag: "--strict", Toggle: true},
		}},
		{Name: "diff", Short: "Compare the contract with a git branch", Args: []string{"diff"}, Fields: []tui.Field{
			{Label: "Base", Hint: "main, or main..feature"},
		}},
		{Name: "diff --live", Short: "Compare the contract with a running API", Args: []string{"diff"}, Fields: []tui.Field{
			{Label: "API URL", Flag: "--live", Required: true, Hint: "https://staging.example.com"},
			{Label: "Send writes too", Flag: "--include-writes", Toggle: true},
		}},
		{Name: "import", Short: "Import an OpenAPI spec or Postman collection", Args: []string{"import"}, Fields: []tui.Field{
			{Label: "File", Required: true, Hint: "openapi.yaml"},
			{Label: "Dry run", Flag: "--dry-run", Toggle: true},
		}},
		{Name: "export", Short: "Export the contract as OpenAPI or Postman", Args: []string{"export"}, Fields: []tui.Field{
			{Label: "Format", Flag: "--format", Default: "openapi", Choices: []string{"openapi", "postman"}},
			{Label: "Output", Flag: "--output", Hint: "standard output"},
		}},
		{Name: "docs", Short: "Browse the contract in Swagger UI", Args: []string{"docs", "--serve"}, Fields: []tui.Field{
			{Label: "Port", Flag: "--port", Default: "4000"},
		}},
		{Name: "docs -o", Short: "Write the docs as static files", Args: []string{"docs"}, Fields: []tui.Field{
			{Label: "Folder", Flag: "--output", Required: true, Hint: "site/"},
		}},
		{Name: "mcp", Short: "Connect an AI assistant (Claude, Cursor, VS Code)", Args: []string{"mcp"}, Fields: []tui.Field{
			{Label: "Assistant", Flag: "--print-config", Required: true, Choices: []string{"claude-code", "claude-desktop", "cursor", "vscode"}},
		}},
		{Name: "cert", Short: "Show how to trust HTTPS on phones and simulators", Args: []string{"cert"}},
		{Name: "cert --install", Short: "Trust the local certificate authority on this computer", Args: []string{"cert", "--install"}},
		{Name: "update", Short: "Update mockmachina to the latest release", Args: []string{"update"}, Fields: []tui.Field{
			{Label: "Only check", Flag: "--check", Toggle: true},
		}},
	}
	if proj == nil {
		return slices.Concat([]tui.Action{initAction}, actions)
	}
	return slices.Concat(actions, []tui.Action{initAction})
}

func stateField(proj *model.Project) tui.Field {
	f := tui.Field{Label: "Route and state", Required: true, Split: true}
	if proj == nil {
		f.Hint = "no project found; run init first"
		return f
	}
	for _, r := range proj.Routes {
		if len(r.States) == 0 {
			continue
		}
		active := cmp.Or(r.Active, r.States[0].Name)
		for _, s := range r.States {
			if s.Name != active {
				f.Choices = append(f.Choices, r.ID+" "+s.Name)
			}
		}
	}
	if f.Choices == nil {
		f.Hint = "no route has another state to switch to"
	}
	return f
}

func interactive(env Env) bool {
	in, okIn := env.Stdin.(*os.File)
	out, okOut := env.Stdout.(*os.File)
	return okIn && okOut && term.IsTerminal(in.Fd()) && term.IsTerminal(out.Fd())
}

func launch(ctx context.Context, env Env) ([]string, error) {
	var proj *model.Project
	if dir, err := nearestProject(); err == nil {
		if p, _, err := config.Load(dir); err == nil {
			proj = p
		}
	}
	p := tea.NewProgram(tui.NewLauncher(env.Info.Version, catalog(proj)),
		tea.WithContext(ctx), tea.WithInput(env.Stdin), tea.WithOutput(env.Stdout))
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	l, _ := final.(tui.Launcher)
	args := l.Chosen()
	if args != nil {
		var s lipgloss.Style
		_, _ = lipgloss.Fprintln(env.Stdout, s.Faint(true).Render("$ mockmachina "+tui.Quote(args)))
	}
	return args, nil
}
