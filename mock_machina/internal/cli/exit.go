package cli

import (
	"errors"
	"strconv"
)

// Exit codes returned by Run (Phase 0 Spec P0-08).
const (
	ExitOK      = 0 // the command did what was asked
	ExitFailure = 1 // problems were found, or the command couldn't run
	ExitUsage   = 2 // the command was called wrongly: bad flag, argument or command
)

// ExitError ends a command with a specific exit code. Commands return it
// instead of calling os.Exit, so they stay testable.
type ExitError struct {
	Code int
	Err  error // may be nil when the command has already reported the problem
}

// Error returns the cause's message, or "exit status N" when there is none.
func (e *ExitError) Error() string {
	if e.Err == nil {
		return "exit status " + strconv.Itoa(e.Code)
	}
	return e.Err.Error()
}

// Unwrap returns the cause, so errors.Is and errors.As see through ExitError.
func (e *ExitError) Unwrap() error { return e.Err }

// usageError marks an error as the caller's mistake in invoking a command.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }

func (e *usageError) Unwrap() error { return e.err }

// UsageError marks err as a usage mistake, which exits with ExitUsage.
func UsageError(err error) error { return &usageError{err: err} }

// ExitCode maps the error a command returned to the process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	var usageErr *usageError
	if errors.As(err, &usageErr) {
		return ExitUsage
	}
	return ExitFailure
}
