package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
)

func newImportCmd() *cobra.Command {
	var dir string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import FILE",
		Short: "Create routes and schemas from an OpenAPI 3.0 or 3.1 spec",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := importDir(cmd, dir)
			if err != nil {
				return err
			}
			return runImport(cmd.OutOrStdout(), args[0], target, dryRun)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project folder (default: the nearest .mockmachina here or above, or a new one here)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be written without writing it")
	return cmd
}

func importDir(cmd *cobra.Command, flag string) (string, error) {
	dir, err := projectDir(cmd, flag)
	if errors.Is(err, ErrNoProject) {
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return "", wdErr
		}
		return filepath.Join(wd, config.DirName), nil
	}
	return dir, err
}

func runImport(out io.Writer, file, dir string, dryRun bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("can't read %s: %w", file, err)
	}
	res, err := openapi.Import(data)
	if err != nil {
		return fmt.Errorf("%s %w", filepath.Base(file), err)
	}
	examples, generated := 0, 0
	states := 0
	for _, r := range res.Routes {
		for _, ns := range r.States {
			states++
			switch {
			case ns.State.Body.Generate:
				generated++
			case len(ns.State.Body.Data) > 0:
				examples++
			}
		}
	}
	label := res.Version
	if !strings.HasPrefix(label, "Swagger") && !strings.HasPrefix(label, "Postman") {
		label = "OpenAPI " + label
	}
	_, _ = fmt.Fprintf(out, "read %s %q: %s, %s (%d from examples, %d generated from schemas), %s\n",
		label, res.Title, plural(len(res.Routes), "route"), plural(states, "state"), examples, generated, plural(len(res.Schemas), "schema"))
	verb := "wrote"
	if dryRun {
		verb = "would write"
	}
	files, err := writeImport(out, dir, res, dryRun)
	if err != nil {
		return err
	}
	for _, f := range files {
		_, _ = fmt.Fprintf(out, "%s %s\n", verb, f)
	}
	for _, n := range res.Notes {
		_, _ = fmt.Fprintf(out, "note: %s\n", n)
	}
	if !dryRun {
		_, _ = fmt.Fprintln(out, "next: mockmachina lint, then mockmachina start")
	}
	return nil
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func writeImport(out io.Writer, dir string, res *openapi.Result, dryRun bool) ([]string, error) {
	if _, err := os.Stat(filepath.Join(dir, "routes")); err != nil {
		if dryRun {
			return config.ContractFiles(res.Routes, res.Schemas)
		}
		return config.WriteContract(dir, res.Routes, res.Schemas)
	}
	rep, err := config.MergeContract(dir, res.Routes, res.Schemas, dryRun)
	if err != nil {
		return nil, err
	}
	for _, id := range rep.Added {
		_, _ = fmt.Fprintf(out, "added %s\n", id)
	}
	for _, id := range rep.Updated {
		_, _ = fmt.Fprintf(out, "updated %s\n", id)
	}
	for _, id := range rep.NewStates {
		_, _ = fmt.Fprintf(out, "added state %s\n", id)
	}
	if len(rep.NotInSpec) > 0 {
		_, _ = fmt.Fprintf(out, "not in the spec, kept: %s\n", strings.Join(rep.NotInSpec, ", "))
	}
	if len(rep.Files) == 0 {
		_, _ = fmt.Fprintln(out, "already up to date")
	}
	return rep.Files, nil
}
