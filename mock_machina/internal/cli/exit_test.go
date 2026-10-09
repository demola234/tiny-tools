package cli_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, cli.ExitOK},
		{"plain error", errors.New("boom"), cli.ExitFailure},
		{"exit error with code 1", &cli.ExitError{Code: 1}, 1},
		{"exit error with code 2", &cli.ExitError{Code: 2}, 2},
		{"wrapped exit error", fmt.Errorf("lint: %w", &cli.ExitError{Code: 2}), 2},
		{"usage error", cli.UsageError(errors.New("unknown flag: --nope")), cli.ExitUsage},
		{"wrapped usage error", fmt.Errorf("run: %w", cli.UsageError(errors.New("bad"))), cli.ExitUsage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := cli.ExitCode(tc.err); got != tc.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestExitError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *cli.ExitError
		want string
	}{
		{"with cause", &cli.ExitError{Code: 1, Err: errors.New("3 problems")}, "3 problems"},
		{"without cause", &cli.ExitError{Code: 1}, "exit status 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExitError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	err := fmt.Errorf("outer: %w", &cli.ExitError{Code: 1, Err: cause})
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(%v, cause) = false, want true", err)
	}
}

func TestUsageError_KeepsMessageAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("unknown flag: --nope")
	err := cli.UsageError(cause)
	if err.Error() != cause.Error() {
		t.Errorf("Error() = %q, want %q", err.Error(), cause.Error())
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(UsageError(cause), cause) = false, want true")
	}
}
