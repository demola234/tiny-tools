package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/mcp"
)

const tokenEnv = "MOCKMACHINA_MCP_TOKEN" //nolint:gosec // the name of an environment variable, not a credential

type mcpOptions struct {
	dir         string
	readOnly    bool
	liveHeaders []string
	client      string
	httpAddr    string
	token       string
}

func newMCPCmd() *cobra.Command {
	var o mcpOptions
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Let AI assistants read and add to the contract, over the Model Context Protocol",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.run(cmd)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.dir, "dir", "", dirFlagUsage)
	f.StringArrayVar(&o.liveHeaders, "live-header", nil, `header diff_live sends to the API, like "Authorization: Bearer …" (repeatable); kept out of the chat`)
	f.StringVar(&o.client, "print-config", "", "print the setup for an assistant instead of serving: "+strings.Join(mcpClients, ", "))
	f.BoolVar(&o.readOnly, "read-only", false, "only offer tools that read; leave out add_route, add_state and set_state")
	f.StringVar(&o.httpAddr, "http", "", "serve over HTTP at this address, like 127.0.0.1:4002, for assistants that connect by URL")
	f.StringVar(&o.token, "token", "", "with --http, the token every request must carry (default $"+tokenEnv+")")
	return cmd
}

func (o *mcpOptions) run(cmd *cobra.Command) error {
	headers, err := parseHeaders("--live-header", o.liveHeaders)
	if err != nil {
		return err
	}
	if err := o.checkHTTP(cmd); err != nil {
		return err
	}
	if o.client != "" {
		if err := checkClient(o.client); err != nil {
			return err
		}
	}
	dir, err := projectDir(cmd, o.dir)
	if err != nil {
		return err
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return err
	}
	opts := mcp.Options{Dir: dir, Version: cmd.Root().Version, ReadOnly: o.readOnly, LiveHeaders: headers}
	switch {
	case o.client != "":
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		printMCPConfig(cmd.OutOrStdout(), o.client, exe, launchArgs(dir, o.readOnly, o.liveHeaders))
		return nil
	case o.httpAddr != "":
		return o.serveHTTP(cmd, opts)
	default:
		return mcp.Serve(cmd.Context(), opts, cmd.InOrStdin(), cmd.OutOrStdout())
	}
}

func (o *mcpOptions) checkHTTP(cmd *cobra.Command) error {
	if o.httpAddr == "" {
		if cmd.Flags().Changed("token") {
			return UsageError(errors.New("--token only applies with --http"))
		}
		return nil
	}
	host, _, err := net.SplitHostPort(o.httpAddr)
	if err != nil {
		return UsageError(fmt.Errorf("--http %q must be host:port, like 127.0.0.1:4002", o.httpAddr))
	}
	if o.token == "" {
		o.token = os.Getenv(tokenEnv)
	}
	if o.token == "" && !isLoopback(host) {
		return UsageError(fmt.Errorf("--http %s can be reached from other machines; set --token too", o.httpAddr))
	}
	return nil
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func (o *mcpOptions) serveHTTP(cmd *cobra.Command, opts mcp.Options) error {
	if _, _, err := config.Load(opts.Dir); err != nil {
		return fmt.Errorf("can't read project folder %s: %w", opts.Dir, err)
	}
	ln, err := new(net.ListenConfig).Listen(cmd.Context(), "tcp", o.httpAddr)
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/mcp"
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "serving MCP on %s (Ctrl+C to stop)\n", url)
	if o.token != "" {
		_, _ = fmt.Fprintf(out, "requests need the token: send \"Authorization: Bearer <token>\", or connect to %s/<token>\n", url)
	}
	return serve(cmd.Context(), ln, mcp.Handler(opts, o.token))
}

func launchArgs(dir string, readOnly bool, liveHeaders []string) []string {
	args := []string{"mcp", "--dir", dir}
	if readOnly {
		args = append(args, "--read-only")
	}
	for _, h := range liveHeaders {
		args = append(args, "--live-header", h)
	}
	return args
}
