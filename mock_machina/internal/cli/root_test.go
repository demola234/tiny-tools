package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestNewRootCmd_UsesBinaryName(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCmd()
	if cmd.Use != "mockmachina" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "mockmachina")
	}
}

func TestNewRootCmd_SilencesCobraOutput(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCmd()
	if !cmd.SilenceUsage || !cmd.SilenceErrors {
		t.Errorf("SilenceUsage = %v, SilenceErrors = %v, want both true", cmd.SilenceUsage, cmd.SilenceErrors)
	}
}

func TestNewRootCmd_NoArgsPrintsHelp(t *testing.T) {
	t.Parallel()

	out, err := execute(t)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("output = %q, want it to contain the usage text", out)
	}
}

func TestNewRootCmd_UsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"unknown command", []string{"nope"}, `unknown command "nope" for "mockmachina"`},
		{"unknown flag", []string{"--nope"}, "unknown flag: --nope"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := execute(t, tc.args...)
			if got := cli.ExitCode(err); got != cli.ExitUsage {
				t.Errorf("ExitCode(%v) = %d, want %d", err, got, cli.ExitUsage)
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantMsg)
			}
		})
	}
}

// execute runs a fresh root command with args and returns its combined output.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	cmd := cli.NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}
