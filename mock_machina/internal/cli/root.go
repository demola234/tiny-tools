package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCmd builds the mockmachina command tree. It returns a fresh tree on
// every call so tests never share state.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mockmachina",
		Short: "Serve mock APIs from contract files in .mockmachina/",
		// Errors are printed once, by Run; usage is not repeated after them.
		SilenceUsage:  true,
		SilenceErrors: true,
		// Arbitrary args reach RunE, so an unknown command becomes a usage
		// error instead of being silently accepted.
		Args: cobra.ArbitraryArgs,
		RunE: runRoot,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return UsageError(err)
	})
	return cmd
}

// runRoot prints help when called with no arguments and rejects anything else
// as an unknown command.
func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return UsageError(fmt.Errorf("unknown command %q for %q", args[0], cmd.Name()))
}
