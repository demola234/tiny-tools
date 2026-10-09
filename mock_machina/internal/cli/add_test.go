package cli_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestAdd_NeedsMethodAndPath(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"add"}, {"add", "GET"}, {"add", "GET", "/a", "/b"}} {
		if code, _, stderr := runCLI(t, args...); code != cli.ExitUsage {
			t.Errorf("mockmachina %v: exit %d (%q), want 2", args, code, stderr)
		}
	}
}
