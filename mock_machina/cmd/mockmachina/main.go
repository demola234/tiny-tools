// Command mockmachina serves mock APIs from the contract files in .mockmachina/.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Run(ctx, cli.Env{
		Args:   os.Args[1:],
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Info:   buildinfo.Read(),
	})
	stop() // os.Exit skips deferred calls, so release the signal handler first
	os.Exit(code)
}
