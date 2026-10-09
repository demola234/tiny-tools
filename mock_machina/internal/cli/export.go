package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
)

type exportOptions struct {
	dir, out, title, version, format string
	force                            bool
}

func newExportCmd() *cobra.Command {
	var o exportOptions
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write the contract as an OpenAPI 3.1 spec",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.run(cmd)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.dir, "dir", "", dirFlagUsage)
	f.StringVarP(&o.out, "output", "o", "", "file to write (default: standard output)")
	f.StringVar(&o.title, "title", "", "info.title (default: the name of the folder holding .mockmachina)")
	f.StringVar(&o.version, "version", "", "info.version (default 0.0.0)")
	f.BoolVar(&o.force, "force", false, "export even when lint finds errors")
	f.StringVar(&o.format, "format", "openapi", "openapi (a 3.1 spec) or postman (a collection and environment in the -o folder)")
	return cmd
}

func exportPostman(cmd *cobra.Command, p *model.Project, title, folder string) error {
	collection, environment, err := openapi.ExportPostman(p, openapi.Info{Title: title})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	base := filepath.Join(folder, title)
	if err := config.WriteFileAtomic(base+".postman_collection.json", collection, 0o644); err != nil {
		return err
	}
	if err := config.WriteFileAtomic(base+".postman_environment.json", environment, 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s.postman_collection.json and %s.postman_environment.json\n", base, base)
	return err
}

func (o *exportOptions) run(cmd *cobra.Command) error {
	if o.format != "openapi" && o.format != "postman" {
		return UsageError(fmt.Errorf("format %q isn't one of: openapi, postman", o.format))
	}
	if o.format == "postman" && o.out == "" {
		return UsageError(errors.New("postman export needs -o FOLDER, for the collection and environment files"))
	}
	dir, err := projectDir(cmd, o.dir)
	if err != nil {
		return err
	}
	p, probs, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	if probs.HasErrors() && !o.force {
		return errors.New("the contract has problems; fix them first, or pass --force (mockmachina lint lists them)")
	}
	title := o.title
	if title == "" {
		abs, _ := filepath.Abs(dir)
		title = filepath.Base(filepath.Dir(abs))
	}
	if o.format == "postman" {
		return exportPostman(cmd, p, title, o.out)
	}
	data, err := openapi.Export(p, openapi.Info{Title: title, Version: o.version})
	if err != nil {
		return err
	}
	if o.out == "" {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}
	return config.WriteFileAtomic(o.out, data, 0o644)
}
