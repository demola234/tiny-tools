package cli_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestNewRootCmd_UsesBinaryName(t *testing.T) {
	cmd := cli.NewRootCmd()
	if cmd.Use != "mockmachina" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "mockmachina")
	}
}
