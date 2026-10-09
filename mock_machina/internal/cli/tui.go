package cli

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/tui"
)

const (
	fallbackWidth  = 80
	fallbackHeight = 24
)

func newTUICmd() *cobra.Command {
	var opts startOptions
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Serve the mock with a live screen of routes, states and requests",
		Long: "Serve the mock like start does, with a full-screen view: routes and their states on the left, requests as they arrive on the right.\n\n" +
			"Press enter on a state to make it the default (the routes file changes, as with state set), / to filter, tab to look at requests, r to reload, l to list problems and q to quit.",
		Args: startArgs(&opts),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.prepare(cmd); err != nil {
				return err
			}
			return runTUI(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), opts)
		},
	}
	addStartFlags(cmd, &opts)
	return cmd
}

func runTUI(ctx context.Context, in io.Reader, out, stderr io.Writer, o startOptions) error {
	var p *tea.Program
	now := clock.Real{}
	sess, err := openSession(ctx, o,
		func(r server.Request) { p.Send(screenRequest(r, now.Now())) },
		func(line string) { p.Send(tui.Note(line)) })
	if err != nil {
		return sessionError(stderr, err)
	}
	sess.rl.loaded = func(proj *model.Project, probs config.Problems) {
		if len(probs) > 0 {
			p.Send(tui.Problems{Lines: strings.Split(probs.Sorted().String(), "\n")})
			return
		}
		p.Send(tui.Problems{})
		p.Send(tui.Reloaded{Project: proj})
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p = tea.NewProgram(tui.New(sess.proj, sess.screenOptions(o)), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out), tea.WithWindowSize(fallbackWidth, fallbackHeight))
	served := make(chan error, 1)
	go func() {
		served <- sess.run(ctx)
		p.Quit()
	}()
	if len(sess.notes) > 0 {
		go p.Send(tui.Note(strings.Join(sess.notes, " · ")))
	}
	_, runErr := p.Run()
	cancel()
	serveErr := <-served
	if runErr != nil && !errors.Is(runErr, tea.ErrProgramKilled) {
		return runErr
	}
	return serveErr
}

func (s *session) screenOptions(o startOptions) tui.Options {
	header := tui.Header{Project: projectName(o.dir), URL: s.url, Seed: s.seed}
	if target := o.proxyTarget(s.proj.Config); target != nil {
		header.Proxy = target.String()
	}
	return tui.Options{
		Header: header,
		SetActive: func(route, state string) error {
			_, err := config.SetActive(o.dir, route, state)
			return err
		},
		Reload: func() { s.rl.reload(nil) },
	}
}

func screenRequest(r server.Request, at time.Time) tui.Request {
	return tui.Request{
		Time: at, Method: r.Method, Path: r.Path, Status: r.Status,
		Route: r.Route, State: r.State, Note: strings.Join(requestNotes(r), " · "),
	}
}

func requestNotes(r server.Request) []string {
	var notes []string
	if r.Proxied {
		notes = append(notes, "proxied")
	}
	switch r.Reason {
	case "", server.ReasonActive:
	case server.ReasonRule:
		notes = append(notes, "rule "+strconv.Itoa(r.Rule))
	case server.ReasonHeader:
		notes = append(notes, "X-Mock-State header")
	case server.ReasonQuery:
		notes = append(notes, "?__state")
	default:
		notes = append(notes, string(r.Reason))
	}
	if r.Fault != "" {
		notes = append(notes, "fault: "+string(r.Fault))
	}
	return append(notes, problemNote(r)...)
}

func problemNote(r server.Request) []string {
	switch {
	case r.Problem == "":
		return nil
	case r.Suggestion != "":
		return []string{string(r.Problem) + " (closest: " + r.Suggestion + ")"}
	case r.Detail != "":
		return []string{string(r.Problem) + ": " + r.Detail}
	}
	return []string{string(r.Problem)}
}
