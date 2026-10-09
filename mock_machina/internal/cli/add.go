package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

func newAddCmd() *cobra.Command {
	var dir string
	var opts config.AddOptions
	cmd := &cobra.Command{
		Use:   "add METHOD PATH",
		Short: "Add a route, e.g. add GET /users/{id}",
		Args:  usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := projectDir(cmd, dir)
			if err != nil {
				return err
			}
			method, path := args[0], args[1]
			added, err := config.AddRoute(dir, method, path, opts)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if added.CreatedFile {
				_, _ = fmt.Fprintf(out, "created %s with %s (%s %s)\n", added.File, added.ID, method, path)
			} else {
				_, _ = fmt.Fprintf(out, "added %s (%s %s) to %s\n", added.ID, method, path, added.File)
			}
			_, _ = fmt.Fprintf(out, "next: describe the response in %s\n", added.File)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", dirFlagUsage)
	cmd.Flags().StringVar(&opts.Name, "name", "", "route name, instead of one picked from the method (list, get, create…)")
	cmd.Flags().StringVar(&opts.Summary, "summary", "", "one line saying what the route returns")
	return cmd
}
