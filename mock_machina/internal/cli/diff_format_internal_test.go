package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
)

func TestGithubEscaping(t *testing.T) {
	t.Parallel()

	if got := githubData("50% done\r\nnext"); got != "50%25 done%0D%0Anext" {
		t.Errorf("githubData = %q", got)
	}
	if got := githubProperty("a:b,c%"); got != "a%3Ab%2Cc%25" {
		t.Errorf("githubProperty = %q", got)
	}
}

func TestMarkdownEscapesPipes(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	r := diffReport{from: label{text: "main", code: true}, to: label{text: "the working tree"}, noun: "change", changes: []diff.Change{{Severity: diff.Info, Route: "a.get", Message: `summary now "x | y"`}}}
	r.markdown(&out)
	if !strings.Contains(out.String(), `| info | `+"`a.get`"+` | summary now "x \| y" |`) {
		t.Errorf("markdown = %q, want the pipe escaped", out.String())
	}
}
