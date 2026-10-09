package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func newInitCmd() *cobra.Command {
	dir := config.DirName
	var opts config.InitOptions
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create .mockmachina/ with settings and an example route",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := usageArgs(cobra.NoArgs)(cmd, args); err != nil {
				return err
			}
			if opts.Port < 0 || opts.Port > maxPort {
				return UsageError(fmt.Errorf("port %d isn't a port (1-%d)", opts.Port, maxPort))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			created, err := config.Init(dir, opts)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, name := range created {
				_, _ = fmt.Fprintf(out, "created %s\n", filepath.ToSlash(filepath.Join(dir, name)))
			}
			if opts.NoExample {
				_, _ = fmt.Fprintln(out, "next: add a route with mockmachina add GET /users, then mockmachina start")
				return nil
			}
			cfg := model.DefaultConfig()
			if opts.Port != 0 {
				cfg.Ports.Mock = opts.Port
			}
			_, _ = fmt.Fprintf(out, "next: mockmachina start, then open http://%s:%d/health\n", cfg.Host, cfg.Ports.Mock)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", dir, "project folder to create")
	cmd.Flags().IntVar(&opts.Port, "port", 0, "port to put in config.yaml (default 4001)")
	cmd.Flags().BoolVar(&opts.NoExample, "no-example", false, "don't add the example health route")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "rewrite config.yaml and the example route in an existing project")
	return cmd
}
