package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

var testInfo = buildinfo.Info{Version: "v0.0.0-test", Commit: "abcdef123456"}

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no args prints help", nil, cli.ExitOK, "mockmachina", ""},
		{"version flag", []string{"--version"}, cli.ExitOK, "v0.0.0-test (abcdef123456)", ""},
		{"unknown flag", []string{"--nope"}, cli.ExitUsage, "", "Unknown flag: --nope."},
		{"unknown command", []string{"nope"}, cli.ExitUsage, "", `Unknown command "nope" for "mockmachina".`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := cli.Run(t.Context(), cli.Env{
				Args:   tc.args,
				Stdout: &stdout,
				Stderr: &stderr,
				Info:   testInfo,
			})
			if code != tc.wantCode {
				t.Errorf("Run(%q) = %d, want %d (stderr: %q)", tc.args, code, tc.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tc.wantStdout)
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}
