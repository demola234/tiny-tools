package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

func newImportCmd() *cobra.Command {
	var dir string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import [FILE]",
		Short: "Create routes and schemas from an OpenAPI, Swagger or Postman file",
		Long: "Create routes and schemas from an OpenAPI 3.x, Swagger 2.0 or Postman collection file. " +
			"Without FILE, in a terminal, it lists the specs found in this folder and below to pick from.",
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := ""
			if len(args) == 1 {
				file = args[0]
			} else {
				picked, dry, err := pickSpec(cmd)
				if err != nil || picked == "" {
					return err
				}
				file, dryRun = picked, dryRun || dry
			}
			target, err := importDir(cmd, dir)
			if err != nil {
				return err
			}
			return runImport(cmd.OutOrStdout(), file, target, dryRun)
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
	files, err := writeImport(out, dir, file, res, dryRun)
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

func writeImport(out io.Writer, dir, file string, res *openapi.Result, dryRun bool) ([]string, error) {
	if _, err := os.Stat(filepath.Join(dir, "routes")); err != nil {
		if dryRun {
			return config.ContractFiles(res.Routes, res.Schemas)
		}
		return config.WriteContract(dir, res.Routes, res.Schemas)
	}
	renameClashes(out, dir, file, res)
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

var (
	notWord   = regexp.MustCompile(`[^A-Za-z0-9]+`)
	versionly = regexp.MustCompile(`^(v?\d.*)$`)
	specStart = regexp.MustCompile(`(?m)^\s*(openapi|swagger)\s*:|"(openapi|swagger)"\s*:|schema\.getpostman\.com`)
	skipDirs  = []string{"node_modules", "vendor", "build", "dist", "Pods", config.DirName}
)

const (
	specSearchDepth = 4
	specPeekBytes   = 4096
	specMaxBytes    = 20 << 20
)

func schemaPrefix(file string) string {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	var b strings.Builder
	for _, w := range notWord.Split(base, -1) {
		if w == "" || versionly.MatchString(w) {
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	if b.Len() == 0 {
		return "Imported"
	}
	return b.String()
}

func findSpecs(root string) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains(skipDirs, d.Name()) || strings.Count(rel, string(filepath.Separator)) >= specSearchDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if isSpec(path, d) {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	return found
}

func isSpec(path string, d fs.DirEntry) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".json":
	default:
		return false
	}
	if info, err := d.Info(); err != nil || info.Size() > specMaxBytes {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, specPeekBytes)
	n, _ := io.ReadFull(f, head)
	return specStart.Match(head[:n])
}

func renameClashes(out io.Writer, dir, file string, res *openapi.Result) {
	proj, _, err := config.Load(dir)
	if err != nil {
		return
	}
	owner := map[string]string{}
	for _, s := range proj.Schemas {
		owner[s.Name] = s.Src.File
	}
	imported := map[string]bool{}
	for _, s := range res.Schemas {
		imported[s.Name] = true
	}
	prefix := schemaPrefix(file)
	to := map[string]string{}
	for _, s := range res.Schemas {
		taken, ok := owner[s.Name]
		if !ok || taken == s.Src.File {
			continue
		}
		name := prefix + s.Name
		for n := 2; (owner[name] != "" && owner[name] != s.Src.File) || imported[name]; n++ {
			name = fmt.Sprintf("%s%s%d", prefix, s.Name, n)
		}
		to[s.Name] = name
		_, _ = fmt.Fprintf(out, "renamed schema %s → %s (%s already has %s)\n", s.Name, name, taken, s.Name)
	}
	res.RenameSchemas(to)
}

func specsHere() []string {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	return findSpecs(wd)
}

func noFileError(specs []string) error {
	if len(specs) == 0 {
		return UsageError(errors.New("import needs a FILE, and there's no OpenAPI, Swagger or Postman file in this folder or below"))
	}
	return UsageError(fmt.Errorf("import needs a FILE; found here:\n  %s", strings.Join(specs, "\n  ")))
}

func pickSpec(cmd *cobra.Command) (string, bool, error) {
	specs := specsHere()
	if !isTerminal(cmd.InOrStdin()) || !isTerminal(cmd.OutOrStdout()) {
		return "", false, noFileError(specs)
	}
	var importAction tui.Action
	for _, a := range catalog(nil, specs) {
		if a.Name == "import" {
			importAction = a
		}
	}
	l := tui.NewLauncher(infoFrom(cmd.Context()).Version, []tui.Action{importAction}).Open("import")
	args, err := pick(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), l)
	if err != nil || len(args) < 2 {
		return "", false, err
	}
	return args[1], slices.Contains(args, "--dry-run"), nil
}
