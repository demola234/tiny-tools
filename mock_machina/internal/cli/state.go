package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Switch which state a route returns",
		Args:  cobra.ArbitraryArgs,
		RunE:  runGroup,
	}
	cmd.AddCommand(newStateSetCmd(), newStateListCmd())
	return cmd
}

func newStateSetCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "set ROUTE STATE",
		Short: "Make STATE the default response of ROUTE (a running server picks it up)",
		Args:  usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			route, state := args[0], args[1]
			dir, err := projectDir(cmd, dir)
			if err != nil {
				return err
			}
			previous, err := config.SetActive(dir, route, state)
			if err != nil {
				return err
			}
			if previous == state {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s is already %s\n", route, state)
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", route, previous, state)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", dirFlagUsage)
	return cmd
}
