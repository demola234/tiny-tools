package cli

import (
	"errors"
	"strconv"
)

const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return "exit status " + strconv.Itoa(e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }

func (e *usageError) Unwrap() error { return e.err }

func UsageError(err error) error { return &usageError{err: err} }

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
