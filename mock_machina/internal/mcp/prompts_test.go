package mcp_test

import (
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/mcp"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestPrompts_Listed(t *testing.T) {
	t.Parallel()

	res, err := connect(t, shop(t)).ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	testkit.Golden(t, indent(t, res.Prompts), "mcp/prompts.json")
}

func TestPrompts_Render(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     map[string]string
		readOnly bool
		golden   string
	}{
		{"suggest_states", nil, false, "suggest_states_all"},
		{"suggest_states", map[string]string{"route": "users.get"}, false, "suggest_states_route"},
		{"suggest_states", map[string]string{"route": "users.get"}, true, "suggest_states_read_only"},
		{"draft_from_sample", map[string]string{"method": "GET", "path": "/orders/{id}", "sample": `{"id":"o_1","total":12.5}`}, false, "draft_from_sample"},
		{"draft_from_sample", map[string]string{"method": "GET", "path": "/orders/{id}", "sample": `{"id":"o_1"}`}, true, "draft_from_sample_read_only"},
		{"explain_diff", nil, false, "explain_diff"},
		{"explain_diff", map[string]string{"base": "main"}, false, "explain_diff_base"},
		{"check_live", map[string]string{"url": "https://staging.example.com"}, false, "check_live"},
	}
	dir := shop(t)
	sessions := map[bool]*sdk.ClientSession{
		false: connect(t, dir),
		true:  connectWith(t, mcp.Options{Dir: dir, ReadOnly: true}),
	}
	for _, tc := range tests {
		res, err := sessions[tc.readOnly].GetPrompt(t.Context(), &sdk.GetPromptParams{Name: tc.name, Arguments: tc.args})
		if err != nil {
			t.Fatalf("%s %v: %v", tc.name, tc.args, err)
		}
		if len(res.Messages) != 1 || res.Messages[0].Role != "user" {
			t.Fatalf("%s: messages %+v; want one from the user", tc.name, res.Messages)
		}
		msg, _ := res.Messages[0].Content.(*sdk.TextContent)
		testkit.Golden(t, []byte(msg.Text+"\n"), "mcp/prompts/"+tc.golden+".md")
	}
}

func TestPrompts_RequiredArguments(t *testing.T) {
	t.Parallel()

	cs := connect(t, shop(t))
	_, err := cs.GetPrompt(t.Context(), &sdk.GetPromptParams{Name: "draft_from_sample", Arguments: map[string]string{"method": "GET"}})
	if err == nil || !strings.Contains(err.Error(), `prompt draft_from_sample needs "path"`) {
		t.Errorf("error = %v", err)
	}
}
