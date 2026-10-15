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
	if exitErr, ok := errors.AsType[*ExitError](err); ok {
		return exitErr.Code
	}
	if _, ok := errors.AsType[*usageError](err); ok {
		return ExitUsage
	}
	return ExitFailure
}
