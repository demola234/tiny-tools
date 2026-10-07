package cli

import (
	"context"
	"io"

	"charm.land/fang/v2"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
)

type Env struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	Info   buildinfo.Info
}

func Run(ctx context.Context, env Env) int {
	root := NewRootCmd()
	root.SetArgs(env.Args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	err := fang.Execute(ctx, root, fang.WithVersion(env.Info.String()))
	return ExitCode(err)
}
