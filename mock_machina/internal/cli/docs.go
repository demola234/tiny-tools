package cli

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/docs"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
)

const defaultDocsPort = 4000

type docsOptions struct {
	dir, out, title, host string
	port                  int
	serve                 bool
}

func newDocsCmd() *cobra.Command {
	o := docsOptions{host: "127.0.0.1", port: defaultDocsPort}
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Browse the contract in Swagger UI, served live or written as static files",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.run(cmd)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.dir, "dir", "", dirFlagUsage)
	f.BoolVar(&o.serve, "serve", false, "serve the docs, following edits to .mockmachina/")
	f.StringVarP(&o.out, "output", "o", "", "write the docs to this folder instead, for any static web host")
	f.StringVar(&o.title, "title", "", "page title (default: the name of the folder holding .mockmachina)")
	f.StringVar(&o.host, "host", o.host, "address to serve on")
	f.IntVar(&o.port, "port", o.port, "port to serve on")
	return cmd
}

func (o *docsOptions) run(cmd *cobra.Command) error {
	if o.serve == (o.out != "") {
		return UsageError(errors.New("pass --serve, or -o FOLDER to write static files"))
	}
	dir, err := projectDir(cmd, o.dir)
	if err != nil {
		return err
	}
	title := o.title
	if title == "" {
		abs, _ := filepath.Abs(dir)
		title = filepath.Base(filepath.Dir(abs))
	}
	spec := func() ([]byte, error) {
		p, _, err := config.Load(dir)
		if err != nil {
			return nil, fmt.Errorf("can't read project folder %s: %w", dir, err)
		}
		return openapi.Export(p, openapi.Info{Title: title})
	}
	out := cmd.OutOrStdout()
	if o.out != "" {
		data, err := spec()
		if err != nil {
			return err
		}
		if _, err := docs.Write(o.out, title, data); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "wrote %s and the spec, for any static web host\n", filepath.Join(o.out, "index.html"))
		return nil
	}
	ln, err := new(net.ListenConfig).Listen(cmd.Context(), "tcp", net.JoinHostPort(o.host, strconv.Itoa(o.port)))
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "docs for %s at http://%s (Ctrl+C to stop)\n", title, ln.Addr())
	return serve(cmd.Context(), ln, docs.Handler(title, spec))
}
