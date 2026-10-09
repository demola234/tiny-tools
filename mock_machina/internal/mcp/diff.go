package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/live"
)

type diffIn struct {
	Base string `json:"base,omitempty" jsonschema:"git ref to compare from, such as main; HEAD when left out"`
	Head string `json:"head,omitempty" jsonschema:"git ref to compare to; the working tree when left out"`
}

type diffLiveIn struct {
	URL    string            `json:"url" jsonschema:"base URL of the running API, such as https://staging.example.com"`
	Params map[string]string `json:"params,omitempty" jsonschema:"path parameter values, such as {\"id\": \"u_1\"}; routes' examples are used otherwise"`
}

type diffOut struct {
	Base    string   `json:"base"`
	Head    string   `json:"head"`
	Changes []change `json:"changes"`
}

type change struct {
	Severity string `json:"severity" jsonschema:"breaking, warning, info or safe"`
	Route    string `json:"route"`
	State    string `json:"state,omitempty"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

func addDiffTools(s *sdk.Server, t tools) {
	addRaw(s, &sdk.Tool{
		Name: "diff",
		Description: "Compare the contract between git refs, or a ref and the working tree. " +
			"Each change is breaking (apps built on the old contract fail), warning, info or safe.",
		Annotations: reads("Compare contract versions", false),
	}, t.diff)
	addRaw(s, &sdk.Tool{
		Name: "diff_live",
		Description: "Call a running API and compare its responses with the contract: statuses, content types, headers and JSON body shape. " +
			"Only GET, HEAD and OPTIONS are sent.",
		Annotations: reads("Compare with a live API", true),
	}, t.diffLive)
}

func (t tools) diff(ctx context.Context, in diffIn) (diffOut, string, error) {
	if in.Base == "" {
		in.Base = "HEAD"
	}
	base, err := config.LoadAt(ctx, t.dir, in.Base)
	if err != nil {
		return diffOut{}, "", err
	}
	head, err := config.LoadAt(ctx, t.dir, in.Head)
	if err != nil {
		return diffOut{}, "", err
	}
	to, toText := in.Head, in.Head
	if to == "" {
		to, toText = "working tree", "the working tree"
	}
	changes := diff.Compare(base, head)
	return report(in.Base, to, changes), summary(changes, "change", "contract changes", in.Base, toText), nil
}

func (t tools) diffLive(ctx context.Context, in diffLiveIn) (diffOut, string, error) {
	u, err := url.Parse(in.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return diffOut{}, "", fmt.Errorf("url %q must be an http or https URL", in.URL)
	}
	contract, err := config.LoadAt(ctx, t.dir, "")
	if err != nil {
		return diffOut{}, "", err
	}
	headers := t.liveHeaders.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	changes := live.Check(ctx, contract, live.Options{BaseURL: u, Headers: headers, Params: in.Params})
	return report("contract", in.URL, changes), summary(changes, "difference", "differences", "the contract", in.URL), nil
}

func report(base, head string, changes []diff.Change) diffOut {
	out := diffOut{Base: base, Head: head, Changes: make([]change, len(changes))}
	for i, c := range changes {
		out.Changes[i] = change{
			Severity: c.Severity.String(), Route: c.Route, State: c.State,
			Message: c.Message, File: c.Src.File, Line: c.Src.Line,
		}
	}
	return out
}

func summary(changes []diff.Change, noun, none, from, to string) string {
	if len(changes) == 0 {
		return fmt.Sprintf("no %s between %s and %s", none, from, to)
	}
	counts := map[diff.Severity]int{}
	for _, c := range changes {
		counts[c.Severity]++
	}
	var parts []string
	for _, s := range []diff.Severity{diff.Breaking, diff.Warning, diff.Info, diff.Safe} {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	if len(changes) != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s between %s and %s: %s", len(changes), noun, from, to, strings.Join(parts, ", "))
}
