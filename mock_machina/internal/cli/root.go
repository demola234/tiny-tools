package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mockmachina",
		Short: "Serve mock APIs from contract files in .mockmachina/",

		SilenceUsage:  true,
		SilenceErrors: true,

		Args: cobra.ArbitraryArgs,
		RunE: runGroup,
	}
	cmd.AddCommand(newInitCmd(), newAddCmd(), newStartCmd(), newStateCmd(), newLintCmd(), newDiffCmd(), newMCPCmd(), newImportCmd(), newExportCmd(), newDocsCmd(), newCertCmd(), newTUICmd(), newUpdateCmd())
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return UsageError(err)
	})
	return cmd
}

func runGroup(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	msg := fmt.Sprintf("unknown command %q for %q", args[0], cmd.Name())
	if s, ok := suggest.Closest(args[0], commandNames(cmd)); ok {
		msg += fmt.Sprintf(" (did you mean %q?)", s)
	}
	return UsageError(errors.New(msg))
}

func commandNames(cmd *cobra.Command) []string {
	var names []string
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			names = append(names, c.Name())
		}
	}
	return names
}

func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return UsageError(err)
		}
		return nil
	}
}
