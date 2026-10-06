package cli

import (
	"context"
	"io"

	"charm.land/fang/v2"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
)

// Env is everything Run needs from the outside world. main fills it from the
// process; tests fill it with buffers.
type Env struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	Info   buildinfo.Info
}

// Run executes the command line and returns the process exit code. It is the
// only place Fang is used (ADR 004): Fang styles help and errors, prints
// errors once, and provides --version.
func Run(ctx context.Context, env Env) int {
	root := NewRootCmd()
	root.SetArgs(env.Args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	err := fang.Execute(ctx, root, fang.WithVersion(env.Info.String()))
	return ExitCode(err)
}
