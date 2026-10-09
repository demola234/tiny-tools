package cli

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/watch"
)

const (
	defaultWatchEvery = 250 * time.Millisecond
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

type startOptions struct {
	dir        string
	host       string
	port       int
	hostSet    bool
	portSet    bool
	noCORS     bool
	noMockHdrs bool
	watchEvery time.Duration
	proxy      string
	memory     *server.Memory
	seed       uint64
	source     seed.Source
	noValidate bool
	https      bool
	tlsCert    string
	tlsKey     string
	width      int
	dark       bool
	plain      bool
}

func newStartCmd() *cobra.Command {
	var opts startOptions
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Serve the mock API described in .mockmachina/",
		Long: "Serve the mock API described in .mockmachina/.\n\n" +
			"In a terminal it opens the live screen: routes and states on the left, requests on the right; ↑↓ and enter switch states. " +
			"With --plain, or when the output isn't a terminal (CI, Docker, a pipe), it prints one log line per request instead.",
		Args: startArgs(&opts),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.prepare(cmd); err != nil {
				return err
			}
			if useScreen(opts.plain, isTerminal(cmd.InOrStdin()), isTerminal(cmd.OutOrStdout())) {
				return runTUI(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), opts)
			}
			opts.width, opts.dark = terminal(cmd.OutOrStdout())
			return runStart(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), opts)
		},
	}
	addStartFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "print log lines instead of opening the live screen")
	return cmd
}

func startArgs(opts *startOptions) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := usageArgs(cobra.NoArgs)(cmd, args); err != nil {
			return err
		}
		if (opts.tlsCert == "") != (opts.tlsKey == "") {
			return UsageError(errors.New("--tls-cert and --tls-key go together"))
		}
		if opts.watchEvery <= 0 {
			return UsageError(errors.New("watch interval must be more than 0 (--watch-interval)"))
		}
		if opts.proxy != "" {
			if problem := config.ProxyProblem(opts.proxy); problem != "" {
				return UsageError(errors.New("--proxy " + problem))
			}
		}
		return nil
	}
}

func (o *startOptions) prepare(cmd *cobra.Command) error {
	dir, err := projectDir(cmd, o.dir)
	if err != nil {
		return err
	}
	o.dir = dir
	o.hostSet, o.portSet = cmd.Flags().Changed("host"), cmd.Flags().Changed("port")
	return nil
}

func addStartFlags(cmd *cobra.Command, opts *startOptions) {
	cmd.Flags().StringVar(&opts.dir, "dir", "", dirFlagUsage)
	defaults := model.DefaultConfig()
	cmd.Flags().StringVar(&opts.host, "host", defaults.Host, "address to listen on; 0.0.0.0 lets phones on your network connect")
	cmd.Flags().IntVar(&opts.port, "port", defaults.Ports.Mock, "port to listen on")
	cmd.Flags().BoolVar(&opts.noCORS, "no-cors", false, "don't send CORS headers (they're on so browser and Flutter web apps can call the mock)")
	cmd.Flags().BoolVar(&opts.noMockHdrs, "no-mock-headers", false, "don't add X-Mock-State and X-Mock-Route to responses")
	cmd.Flags().BoolVar(&opts.https, "https", false, "serve HTTPS with a certificate from the local certificate authority (see mockmachina cert)")
	cmd.Flags().StringVar(&opts.tlsCert, "tls-cert", "", "serve HTTPS with this certificate file instead (with --tls-key)")
	cmd.Flags().StringVar(&opts.tlsKey, "tls-key", "", "the private key for --tls-cert")
	cmd.Flags().BoolVar(&opts.noValidate, "no-request-validation", false, "serve states whatever the request looks like, ignoring the routes' request schemas")
	cmd.Flags().Uint64Var(&opts.seed, "seed", 0, "seed for random states and fake data, to replay a run (default: seed in config.yaml, or a new one each start)")
	cmd.Flags().StringVar(&opts.proxy, "proxy", "", "send requests that match no route, and routes with serve: proxy, to this backend URL (default: proxy in config.yaml)")
	cmd.Flags().DurationVar(&opts.watchEvery, "watch-interval", defaultWatchEvery, "how often to check .mockmachina/ for changes")
}

func runStart(ctx context.Context, stdout, stderr io.Writer, o startOptions) error {
	log := &syncWriter{w: stdout}
	sess, err := openSession(ctx, o, func(r server.Request) { log.line(formatRequest(r)) }, log.line)
	if err != nil {
		return sessionError(stderr, err)
	}
	home, _ := os.UserHomeDir()
	if card, ok := o.card(sess, infoFrom(ctx), home); ok {
		playIntro(ctx, stdout, func(at time.Duration) string { return card.RenderAt(o.width, o.dark, at) }, clock.Real{})
	} else {
		for _, l := range append([]string{sess.banner}, sess.notes...) {
			_, _ = lipgloss.Fprintln(stdout, l)
		}
	}
	if err := sess.run(ctx); err != nil {
		return err
	}
	log.line("stopped")
	return nil
}

func sessionError(stderr io.Writer, err error) error {
	var probs problemsError
	if errors.As(err, &probs) {
		_, _ = fmt.Fprintf(stderr, "%s\n%s; not serving until it's fixed\n", probs.probs, probs.probs.Summary())
		return &ExitError{Code: ExitFailure}
	}
	return err
}

type problemsError struct{ probs config.Problems }

func (e problemsError) Error() string { return e.probs.Summary() }

type session struct {
	proj    *model.Project
	ln      net.Listener
	rl      *reloader
	watcher *watch.Watcher
	url     string
	seed    uint64
	banner  string
	notes   []string
}

func openSession(ctx context.Context, o startOptions, report func(server.Request), logf func(string)) (*session, error) {
	o.memory = server.NewMemory()
	watcher := watch.Poller{Dir: o.dir, Interval: o.watchEvery, Clock: clock.Real{}}.Start()
	proj, probs, err := config.Load(o.dir)
	if err != nil {
		return nil, fmt.Errorf("can't read project folder %s: %w", o.dir, err)
	}
	if len(probs) > 0 {
		return nil, problemsError{probs: probs}
	}
	o.source = seed.New(cmp.Or(o.seed, proj.Config.Seed, seed.Pick()))
	handler, err := server.New(proj, o.serverOptions(proj.Config, report))
	if err != nil {
		return nil, err
	}
	s := &session{
		proj:    proj,
		watcher: watcher,
		rl:      &reloader{dir: o.dir, opts: o, cfg: proj.Config, live: server.NewLive(handler), report: report, log: logf},
	}
	if s.ln, err = listen(ctx, o.address(proj.Config)); err != nil {
		return nil, err
	}
	scheme := "http"
	if o.https || o.tlsCert != "" {
		cfg, err := tlsConfig(o, func(line string) { s.notes = append(s.notes, line) })
		if err != nil {
			_ = s.ln.Close()
			return nil, err
		}
		s.ln, scheme = tls.NewListener(s.ln, cfg), "https"
	}
	s.url = scheme + "://" + s.ln.Addr().String()
	s.banner, s.notes = o.banner(proj, s.url), append(s.notes, o.notes(proj)...)
	if server.UsesRandomness(proj) {
		s.seed = o.source.Value()
	}
	return s, nil
}

func listen(ctx context.Context, addr string) (net.Listener, error) {
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", addr)
	if err == nil {
		return ln, nil
	}
	if free, ok := freePortNear(ctx, addr); ok {
		return nil, fmt.Errorf("can't listen on %s (port %d is free, try --port %d): %w", addr, free, free, err)
	}
	return nil, fmt.Errorf("can't listen on %s: %w", addr, err)
}

func (o startOptions) banner(proj *model.Project, url string) string {
	proxying := ""
	if target := o.proxyTarget(proj.Config); target != nil {
		proxying = " and proxying the rest to " + target.String()
	}
	return fmt.Sprintf("serving %s from %s on %s%s (Ctrl+C to stop)", countRoutes(len(proj.Routes)), o.dir, url, proxying)
}

func (o startOptions) notes(proj *model.Project) []string {
	var notes []string
	if server.UsesRandomness(proj) {
		notes = append(notes, fmt.Sprintf("seed %d (start with --seed %d to get the same responses again)", o.source.Value(), o.source.Value()))
	}
	if o.proxyTarget(proj.Config) != nil {
		return notes
	}
	for _, r := range proj.Routes {
		if r.Serve == model.ServeProxy {
			notes = append(notes, "warning: "+r.ID+" has serve: proxy, but there's no proxy target (--proxy, or proxy in config.yaml), so it's mocked")
		}
	}
	return notes
}

func (s *session) run(ctx context.Context) error {
	var watching sync.WaitGroup
	watching.Go(func() {
		s.watcher.Run(ctx, s.rl.reload)
	})
	err := serve(ctx, s.ln, s.rl.live)
	watching.Wait()
	return err
}

func (o startOptions) serverOptions(cfg model.Config, report func(server.Request)) server.Options {
	return server.Options{
		Report: report, Clock: clock.Real{}, DisableCORS: o.noCORS, NoMockHeaders: o.noMockHdrs,
		Proxy: o.proxyTarget(cfg), Memory: o.memory, Seed: o.source, Locale: cfg.Locale,
		NoRequestValidation: o.noValidate,
	}
}

func (o startOptions) proxyTarget(cfg model.Config) *url.URL {
	target := cmp.Or(o.proxy, cfg.Proxy)
	if target == "" {
		return nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil
	}
	return u
}

func (o startOptions) address(cfg model.Config) string {
	host, port := cfg.Host, cfg.Ports.Mock
	if o.hostSet {
		host = o.host
	}
	if o.portSet {
		port = o.port
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

type reloader struct {
	dir    string
	opts   startOptions
	cfg    model.Config
	live   *server.Live
	report func(server.Request)
	log    func(string)
	loaded func(*model.Project, config.Problems)
}

func (rl *reloader) reload(changed []string) {
	const failed = "reload failed, still serving the previous version:"
	proj, probs, err := config.Load(rl.dir)
	if err != nil {
		rl.log(fmt.Sprintf("%s\n  can't read project folder %s: %v", failed, rl.dir, err))
		return
	}
	if len(probs) > 0 {
		rl.log(failed + "\n  " + strings.ReplaceAll(probs.String(), "\n", "\n  "))
		rl.notify(nil, probs)
		return
	}
	handler, err := server.New(proj, rl.opts.serverOptions(proj.Config, rl.report))
	if err != nil {
		rl.log(failed + "\n  " + err.Error())
		return
	}
	rl.live.Swap(handler)
	rl.notify(proj, nil)
	rl.log(fmt.Sprintf("reloaded%s (%s)", describe(changed), countRoutes(len(proj.Routes))))
	if was, now := rl.opts.address(rl.cfg), rl.opts.address(proj.Config); now != was {
		rl.log(fmt.Sprintf("config.yaml now says %s; restart to use it (still serving on %s)", now, was))
	}
}

func (rl *reloader) notify(proj *model.Project, probs config.Problems) {
	if rl.loaded != nil {
		rl.loaded(proj, probs)
	}
}

func describe(changed []string) string {
	switch len(changed) {
	case 0:
		return ""
	case 1:
		return " after changes to " + changed[0]
	}
	return fmt.Sprintf(" after changes to %s and %d more", changed[0], len(changed)-1)
}

func serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: readHeaderTimeout}
	stopped := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		stopped <- srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-stopped
}

func formatRequest(r server.Request) string {
	if r.Proxied {
		line := fmt.Sprintf("%s %s %d", r.Method, r.Path, r.Status)
		if r.Route != "" {
			line += " " + r.Route
		}
		line += " proxied"
		if r.Problem != "" {
			line += " " + string(r.Problem)
		}
		return line
	}
	if r.Problem == server.ProblemNoRoute {
		line := fmt.Sprintf("%s %s %d %s", r.Method, r.Path, r.Status, r.Problem)
		if r.Suggestion != "" {
			line += " (closest: " + r.Suggestion + ")"
		}
		return line
	}
	reason := string(r.Reason)
	if r.Rule > 0 {
		reason += " " + strconv.Itoa(r.Rule)
	}
	if r.Fault != "" {
		return fmt.Sprintf("%s %s %s:%s (%s) fault: %s", r.Method, r.Path, r.Route, r.State, reason, r.Fault)
	}
	line := fmt.Sprintf("%s %s %d %s:%s (%s)", r.Method, r.Path, r.Status, r.Route, r.State, reason)
	if r.Problem != "" {
		line += " " + string(r.Problem)
	}
	if r.Detail != "" {
		line += ": " + r.Detail
	}
	return line
}

func countRoutes(n int) string {
	if n == 1 {
		return "1 route"
	}
	return strconv.Itoa(n) + " routes"
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) line(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = io.WriteString(s.w, text+"\n")
}

const portsToProbe = 20

func freePortNear(ctx context.Context, addr string) (int, bool) {
	host, portText, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portText)
	for p := port + 1; p <= min(port+portsToProbe, maxPort); p++ {
		ln, err := new(net.ListenConfig).Listen(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(p)))
		if err == nil {
			_ = ln.Close()
			return p, true
		}
	}
	return 0, false
}

const maxPort = 65535
