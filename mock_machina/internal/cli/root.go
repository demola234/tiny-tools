package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mockmachina",
		Short: "Serve mock APIs from contract files in .mockmachina/",

		SilenceUsage:  true,
		SilenceErrors: true,

		Args: cobra.ArbitraryArgs,
		RunE: runRoot,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return UsageError(err)
	})
	return cmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return UsageError(fmt.Errorf("unknown command %q for %q", args[0], cmd.Name()))
}
