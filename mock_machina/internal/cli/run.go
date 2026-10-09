package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"charm.land/fang/v2"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
)

type Env struct {
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Info   buildinfo.Info
}

func Run(ctx context.Context, env Env) int {
	if len(env.Args) == 0 && interactive(env) {
		args, err := launch(ctx, env)
		if err != nil {
			_, _ = fmt.Fprintf(env.Stderr, "error: %v\n", err)
			return ExitFailure
		}
		if args == nil {
			return ExitOK
		}
		env.Args = args
	}
	root := NewRootCmd()
	root.SetArgs(env.Args)
	if env.Stdin != nil {
		root.SetIn(env.Stdin)
	}
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	err := fang.Execute(withInfo(ctx, env.Info), root,
		fang.WithVersion(env.Info.String()),
		fang.WithErrorHandler(printError),
	)
	return ExitCode(err)
}

func printError(w io.Writer, styles fang.Styles, err error) {
	var exitErr *ExitError
	if errors.As(err, &exitErr) && exitErr.Err == nil {
		return
	}
	var usageErr *usageError
	if errors.As(err, &usageErr) {
		fang.DefaultErrorHandler(w, styles, err)
		return
	}
	_, _ = fmt.Fprintf(w, "error: %v\n", err)
}
