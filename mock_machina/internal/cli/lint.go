package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/gitfs"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

var lintFormats = []string{"text", "json", "github"}

type lintOptions struct {
	dir, format, only string
	strict            bool
}

func newLintCmd() *cobra.Command {
	o := lintOptions{format: "text"}
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Check .mockmachina/ for mistakes without serving it",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.run(cmd)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.dir, "dir", "", dirFlagUsage)
	f.BoolVar(&o.strict, "strict", false, "fail on warnings too")
	f.StringVar(&o.format, "format", o.format, "output format: "+strings.Join(lintFormats, ", "))
	f.StringVar(&o.only, "only", "", "only report problems in one routes file, like users for routes/users.yaml")
	return cmd
}

func (o lintOptions) run(cmd *cobra.Command) error {
	if !slices.Contains(lintFormats, o.format) {
		return UsageError(fmt.Errorf("format %q isn't one of: %s", o.format, strings.Join(lintFormats, ", ")))
	}
	dir, err := projectDir(cmd, o.dir)
	if err != nil {
		return err
	}
	proj, probs, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	all := slices.Concat(probs, config.Warnings(proj)).Sorted()
	if o.only != "" {
		if all, err = onlyFile(dir, o.only, all); err != nil {
			return err
		}
	}
	if err := writeProblems(cmd.Context(), cmd.OutOrStdout(), o.format, dir, all); err != nil {
		return err
	}
	if all.HasErrors() || (o.strict && len(all) > 0) {
		return &ExitError{Code: ExitFailure}
	}
	return nil
}

func onlyFile(dir, name string, all config.Problems) (config.Problems, error) {
	entries, _ := os.ReadDir(filepath.Join(dir, "routes"))
	var names []string
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && !e.IsDir() {
			names = append(names, n)
		}
	}
	if !slices.Contains(names, name) {
		if s, ok := suggest.Closest(name, names); ok {
			return nil, fmt.Errorf("no routes file %q (did you mean %q?)", name, s)
		}
		return nil, fmt.Errorf("no routes file %q (files: %s)", name, strings.Join(names, ", "))
	}
	file := path.Join("routes", name+".yaml")
	var out config.Problems
	for _, p := range all {
		if p.File == file {
			out = append(out, p)
		}
	}
	return out, nil
}

type jsonProblem struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

func severity(p config.Problem) string {
	if p.Severity == config.Warning {
		return "warning"
	}
	return "error"
}

func writeProblems(ctx context.Context, out io.Writer, format, dir string, all config.Problems) error {
	switch format {
	case "json":
		list := make([]jsonProblem, len(all))
		for i, p := range all {
			list[i] = jsonProblem{Severity: severity(p), File: p.File, Line: p.Line, Message: p.Msg}
		}
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		return enc.Encode(list)
	case "github":
		prefix := projectPath(ctx, dir)
		for _, p := range all {
			_, _ = fmt.Fprintf(out, "::%s file=%s,line=%d::%s\n", severity(p), githubProperty(path.Join(prefix, p.File)), p.Line, githubData(p.Msg))
		}
		return nil
	}
	if len(all) == 0 {
		_, _ = fmt.Fprintln(out, "no problems")
		return nil
	}
	_, _ = fmt.Fprintf(out, "%s\n%s\n", all, all.Summary())
	return nil
}

func projectPath(ctx context.Context, dir string) string {
	if p, err := gitfs.PathInRepo(ctx, dir); err == nil {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(dir)
	}
	rel, err := filepath.Rel(wd, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}
