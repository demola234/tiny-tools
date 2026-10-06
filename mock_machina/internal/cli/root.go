package cli

import "github.com/spf13/cobra"

// NewRootCmd builds the mockmachina command tree. It returns a fresh tree on
// every call so tests never share state.
func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use: "mockmachina",
	}
}
