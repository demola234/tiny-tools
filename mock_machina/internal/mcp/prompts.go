package mcp

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strings"
	"text/template"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed prompts/*.md
var promptFiles embed.FS

var promptTemplates = template.Must(template.ParseFS(promptFiles, "prompts/*.md"))

var prompts = []*sdk.Prompt{
	{
		Name:        "suggest_states",
		Description: "Find the states apps still need, such as errors, empty lists and slow responses, and add them",
		Arguments:   []*sdk.PromptArgument{{Name: "route", Description: "one route id, such as users.get; every route when left out"}},
	},
	{
		Name:        "draft_from_sample",
		Description: "Turn a real API response into a route with states",
		Arguments: []*sdk.PromptArgument{
			{Name: "method", Description: "HTTP method, such as GET", Required: true},
			{Name: "path", Description: "path with {parameters}, such as /orders/{id}", Required: true},
			{Name: "sample", Description: "the JSON response the API returned", Required: true},
		},
	},
	{
		Name:        "explain_diff",
		Description: "Explain contract changes for a pull request: what breaks and who needs to act",
		Arguments:   []*sdk.PromptArgument{{Name: "base", Description: "git ref to compare from, such as main; the last commit when left out"}},
	},
	{
		Name:        "check_live",
		Description: "Compare the contract with a running API and say which side needs fixing",
		Arguments:   []*sdk.PromptArgument{{Name: "url", Description: "base URL of the API, such as https://staging.example.com", Required: true}},
	},
}

func addPrompts(s *sdk.Server, readOnly bool) {
	for _, p := range prompts {
		s.AddPrompt(p, func(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
			text, err := renderPrompt(p, req.Params.Arguments, readOnly)
			if err != nil {
				return nil, err
			}
			return &sdk.GetPromptResult{
				Description: p.Description,
				Messages:    []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: text}}},
			}, nil
		})
	}
}

func renderPrompt(p *sdk.Prompt, args map[string]string, readOnly bool) (string, error) {
	data := map[string]any{"readOnly": readOnly}
	for _, a := range p.Arguments {
		v := args[a.Name]
		if a.Required && v == "" {
			return "", fmt.Errorf("prompt %s needs %q", p.Name, a.Name)
		}
		data[a.Name] = v
	}
	var b bytes.Buffer
	if err := promptTemplates.ExecuteTemplate(&b, p.Name+".md", data); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}
