package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type diffReport struct {
	from     label
	to       label
	noun     string
	repoPath string
	changes  []diff.Change
}

type label struct {
	text string
	json string
	code bool
}

func (l label) markdown() string {
	if l.code {
		return "`" + l.text + "`"
	}
	return l.text
}

func (r diffReport) write(out io.Writer, format string) error {
	switch format {
	case "json":
		return r.json(out)
	case "markdown":
		r.markdown(out)
	case "github":
		r.github(out)
	default:
		r.text(out)
	}
	return nil
}

func (r diffReport) reaches(level diff.Severity) bool {
	for _, c := range r.changes {
		if c.Severity <= level {
			return true
		}
	}
	return false
}

func (r diffReport) counts(bold bool) string {
	var parts []string
	for _, s := range []diff.Severity{diff.Breaking, diff.Warning, diff.Info, diff.Safe} {
		n := 0
		for _, c := range r.changes {
			if c.Severity == s {
				n++
			}
		}
		if n == 0 {
			continue
		}
		part := fmt.Sprintf("%d %s", n, s)
		if bold && s == diff.Breaking {
			part = "**" + part + "**"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func (r diffReport) total() string {
	if len(r.changes) == 1 {
		return "1 " + r.noun
	}
	return fmt.Sprintf("%d %ss", len(r.changes), r.noun)
}

func (r diffReport) none() string {
	if r.noun == "change" {
		return "no contract changes"
	}
	return "no " + r.noun + "s"
}

func (r diffReport) text(out io.Writer) {
	if len(r.changes) == 0 {
		_, _ = fmt.Fprintf(out, "%s between %s and %s\n", r.none(), r.from.text, r.to.text)
		return
	}
	_, _ = fmt.Fprintf(out, "%s between %s and %s: %s\n", r.total(), r.from.text, r.to.text, r.counts(false))
	for _, c := range r.changes {
		_, _ = fmt.Fprintf(out, "%-8s  %s\n", c.Severity, c.Text())
	}
}

type jsonChange struct {
	Severity string      `json:"severity"`
	Route    string      `json:"route"`
	State    string      `json:"state,omitempty"`
	Message  string      `json:"message"`
	File     string      `json:"file"`
	Line     int         `json:"line"`
	Owners   *jsonOwners `json:"owners,omitempty"`
}

type jsonOwners struct {
	Backend  []string `json:"backend,omitempty"`
	Frontend []string `json:"frontend,omitempty"`
}

func (r diffReport) json(out io.Writer) error {
	changes := make([]jsonChange, len(r.changes))
	for i, c := range r.changes {
		changes[i] = jsonChange{
			Severity: c.Severity.String(), Route: c.Route, State: c.State,
			Message: c.Message, File: c.Src.File, Line: c.Src.Line,
		}
		if !c.Owners.IsZero() {
			changes[i].Owners = &jsonOwners{Backend: handles(c.Owners.Backend), Frontend: handles(c.Owners.Frontend)}
		}
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(struct {
		Base    string       `json:"base"`
		Head    string       `json:"head"`
		Changes []jsonChange `json:"changes"`
	}{r.from.json, r.to.json, changes})
}

func (r diffReport) markdown(out io.Writer) {
	_, _ = fmt.Fprint(out, "### MockMachina contract changes\n\n")
	if len(r.changes) == 0 {
		none := r.none()
		_, _ = fmt.Fprintf(out, "%s%s between %s and %s.\n", strings.ToUpper(none[:1]), none[1:], r.from.markdown(), r.to.markdown())
		return
	}
	_, _ = fmt.Fprintf(out, "%s between %s and %s: %s.\n\n", r.total(), r.from.markdown(), r.to.markdown(), r.counts(true))
	owned := slices.ContainsFunc(r.changes, func(c diff.Change) bool { return !c.Owners.IsZero() })
	if owned {
		_, _ = fmt.Fprint(out, "| Severity | Route | Change | Owners |\n| --- | --- | --- | --- |\n")
	} else {
		_, _ = fmt.Fprint(out, "| Severity | Route | Change |\n| --- | --- | --- |\n")
	}
	for _, c := range r.changes {
		severity := c.Severity.String()
		if c.Severity == diff.Breaking {
			severity = "**" + severity + "**"
		}
		row := fmt.Sprintf("| %s | `%s` | %s |", severity, c.Route, strings.ReplaceAll(c.Message, "|", `\|`))
		if owned {
			row += " " + mentions(c.Owners) + " |"
		}
		_, _ = fmt.Fprintln(out, row)
	}
	if owned {
		_, _ = fmt.Fprintf(out, "\nOwners of the changed routes: %s.\n", r.ownerRoles())
	}
}

func handles(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = strings.TrimPrefix(n, "@")
	}
	return out
}

func mentions(o model.Owners) string {
	var seen []string
	for _, h := range handles(slices.Concat(o.Backend, o.Frontend)) {
		if !slices.Contains(seen, "@"+h) {
			seen = append(seen, "@"+h)
		}
	}
	return strings.Join(seen, ", ")
}

func (r diffReport) ownerRoles() string {
	var order []string
	roles := map[string][]string{}
	add := func(names []string, role string) {
		for _, h := range handles(names) {
			if _, ok := roles[h]; !ok {
				order = append(order, h)
			}
			if !slices.Contains(roles[h], role) {
				roles[h] = append(roles[h], role)
			}
		}
	}
	for _, c := range r.changes {
		add(c.Owners.Backend, "backend")
		add(c.Owners.Frontend, "frontend")
	}
	parts := make([]string, len(order))
	for i, h := range order {
		parts[i] = "@" + h + " (" + strings.Join(roles[h], ", ") + ")"
	}
	return strings.Join(parts, ", ")
}

var githubLevels = map[diff.Severity][2]string{
	diff.Breaking: {"error", "Breaking contract change"},
	diff.Warning:  {"warning", "Contract warning"},
	diff.Info:     {"notice", "Contract change"},
	diff.Safe:     {"notice", "Contract change"},
}

func (r diffReport) github(out io.Writer) {
	for _, c := range r.changes {
		level := githubLevels[c.Severity]
		file := path.Join(r.repoPath, c.Src.File)
		_, _ = fmt.Fprintf(out, "::%s file=%s,line=%d,title=%s::%s\n",
			level[0], githubProperty(file), c.Src.Line, githubProperty(level[1]), githubData(c.Text()))
	}
}

var (
	githubDataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	githubPropertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func githubData(s string) string { return githubDataEscaper.Replace(s) }

func githubProperty(s string) string { return githubPropertyEscaper.Replace(s) }
