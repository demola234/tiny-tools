package cli_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestInit_UsageErrors(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"init", "extra"},
		{"init", "--port", "70000"},
		{"init", "--port", "-1"},
	} {
		if code, _, stderr := runCLI(t, args...); code != cli.ExitUsage {
			t.Errorf("mockmachina %v: exit %d (%q), want 2", args, code, stderr)
		}
	}
}
