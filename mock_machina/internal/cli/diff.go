package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/gitfs"
	"github.com/demola234/tiny-tools/mock_machina/internal/live"
)

var (
	diffFormats  = []string{"text", "json", "markdown", "github"}
	failOnLevels = map[string]diff.Severity{"breaking": diff.Breaking, "warning": diff.Warning}
)

type diffOptions struct {
	dir    string
	format string
	failOn string
	base   string
	head   string
	live   liveOptions
}

type liveOptions struct {
	url           string
	headers       []string
	params        []string
	includeWrites bool
	timeout       time.Duration
	parsed        live.Options
}

func newDiffCmd() *cobra.Command {
	opts := diffOptions{format: "text", failOn: "none"}
	cmd := &cobra.Command{
		Use:   "diff [BASE[..HEAD]]",
		Short: "Show how the contract changed between git refs, or since BASE in your working tree",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := usageArgs(cobra.MaximumNArgs(1))(cmd, args); err != nil {
				return err
			}
			return opts.parse(args)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := projectDir(cmd, opts.dir)
			if err != nil {
				return err
			}
			opts.dir = dir
			return runDiff(cmd.Context(), cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.dir, "dir", "", dirFlagUsage)
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "output format: "+strings.Join(diffFormats, ", "))
	cmd.Flags().StringVar(&opts.failOn, "fail-on", opts.failOn, "exit 1 when changes reach this level: breaking, warning or none")
	cmd.Flags().StringVar(&opts.live.url, "live", "", "compare the contract with a running API at this URL instead of a git ref")
	cmd.Flags().StringArrayVar(&opts.live.headers, "header", nil, `header to send to the live API, like "Authorization: Bearer …" (repeatable)`)
	cmd.Flags().StringArrayVar(&opts.live.params, "param", nil, "path parameter value for the live API, like id=u_1 (repeatable; overrides examples)")
	cmd.Flags().BoolVar(&opts.live.includeWrites, "include-writes", false, "also send POST, PUT, PATCH and DELETE to the live API")
	cmd.Flags().DurationVar(&opts.live.timeout, "timeout", 0, "time limit per live request (default 10s)")
	return cmd
}

func (o *diffOptions) parse(args []string) error {
	if !slices.Contains(diffFormats, o.format) {
		return UsageError(fmt.Errorf("format %q isn't one of: %s", o.format, strings.Join(diffFormats, ", ")))
	}
	if _, ok := failOnLevels[o.failOn]; !ok && o.failOn != "none" {
		return UsageError(fmt.Errorf("--fail-on %q isn't one of: breaking, warning, none", o.failOn))
	}
	if o.live.url != "" {
		if len(args) > 0 {
			return UsageError(errors.New("--live compares the contract with a running API, so it takes no git refs"))
		}
		return o.live.parse()
	}
	o.base = "HEAD"
	if len(args) == 1 {
		o.base, o.head, _ = strings.Cut(args[0], "..")
	}
	if o.base == "" {
		return UsageError(fmt.Errorf("%q has no base ref; write BASE or BASE..HEAD", args[0]))
	}
	return nil
}

func (l *liveOptions) parse() error {
	u, err := url.Parse(l.url)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return UsageError(fmt.Errorf("--live %q must be an http or https URL", l.url))
	}
	headers, err := parseHeaders("--header", l.headers)
	if err != nil {
		return err
	}
	l.parsed = live.Options{BaseURL: u, Headers: headers, Params: map[string]string{}, IncludeWrites: l.includeWrites, Timeout: l.timeout}
	for _, p := range l.params {
		name, value, ok := strings.Cut(p, "=")
		if !ok || name == "" {
			return UsageError(fmt.Errorf("--param %q must look like name=value", p))
		}
		l.parsed.Params[name] = value
	}
	return nil
}

func parseHeaders(flag string, values []string) (http.Header, error) {
	headers := http.Header{}
	for _, h := range values {
		name, value, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, UsageError(fmt.Errorf(`%s %q must look like "Name: value"`, flag, h))
		}
		headers.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	return headers, nil
}

func runDiff(ctx context.Context, cmd *cobra.Command, o diffOptions) error {
	report, err := buildReport(ctx, o)
	if err != nil {
		return err
	}
	if o.format == "github" {
		if report.repoPath, err = gitfs.PathInRepo(ctx, o.dir); err != nil {
			return err
		}
	}
	if err := report.write(cmd.OutOrStdout(), o.format); err != nil {
		return err
	}
	if level, ok := failOnLevels[o.failOn]; ok && report.reaches(level) {
		return &ExitError{Code: ExitFailure}
	}
	return nil
}

func buildReport(ctx context.Context, o diffOptions) (diffReport, error) {
	if o.live.url != "" {
		contract, err := config.LoadAt(ctx, o.dir, "")
		if err != nil {
			return diffReport{}, err
		}
		return diffReport{
			from:    label{text: "the contract", json: "contract"},
			to:      label{text: o.live.url, json: o.live.url, code: true},
			noun:    "difference",
			changes: live.Check(ctx, contract, o.live.parsed),
		}, nil
	}
	base, err := config.LoadAt(ctx, o.dir, o.base)
	if err != nil {
		return diffReport{}, err
	}
	head, err := config.LoadAt(ctx, o.dir, o.head)
	if err != nil {
		return diffReport{}, err
	}
	to := label{text: "the working tree", json: "working tree"}
	if o.head != "" {
		to = label{text: o.head, json: o.head, code: true}
	}
	return diffReport{
		from:    label{text: o.base, json: o.base, code: true},
		to:      to,
		noun:    "change",
		changes: diff.Compare(base, head),
	}, nil
}
