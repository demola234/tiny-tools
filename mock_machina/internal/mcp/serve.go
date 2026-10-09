package mcp

import (
	"context"
	"io"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func Serve(ctx context.Context, opts Options, in io.Reader, out io.Writer) error {
	if _, _, err := (tools{dir: opts.Dir}).load(); err != nil {
		return err
	}
	return New(opts).Run(ctx, &sdk.IOTransport{Reader: io.NopCloser(in), Writer: nopCloser{out}})
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
