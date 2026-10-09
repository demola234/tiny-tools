package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/release"
	"github.com/demola234/tiny-tools/mock_machina/internal/update"
)

const (
	releasesAPI    = "https://api.github.com/repos/demola234/tiny-tools/releases?per_page=100"
	releasesBase   = "https://github.com/demola234/tiny-tools/releases/download"
	connectTimeout = 30 * time.Second
	installPackage = "github.com/demola234/tiny-tools/mock_machina/cmd/mockmachina"
)

type updateOptions struct {
	current string
	check   bool
	exe     string
	gobin   string
	animate bool
	src     update.Source
}

func defaultUpdateOptions(current string) updateOptions {
	o := updateOptions{
		current: current, gobin: goBin(),
		src: update.Source{
			Client:   &http.Client{Transport: updateTransport()},
			API:      cmp.Or(os.Getenv("MOCKMACHINA_API"), releasesAPI),
			Download: cmp.Or(os.Getenv("MOCKMACHINA_DOWNLOAD_BASE"), releasesBase),
		},
	}
	if exe, err := os.Executable(); err == nil {
		o.exe = exe
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			o.exe = real
		}
	}
	return o
}

func newUpdateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update mockmachina to the latest release",
		Long: "Update mockmachina to the latest release. A copy installed with Homebrew, Scoop or go install is updated with that tool; " +
			"any other copy is replaced in place after its checksum is checked. --check only says whether there's a newer release.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := defaultUpdateOptions(infoFrom(cmd.Context()).Version)
			o.check, o.animate = check, isTerminal(cmd.OutOrStdout())
			return runUpdate(cmd.Context(), cmd.OutOrStdout(), o)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only say whether a newer release is out")
	return cmd
}

func updateTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSHandshakeTimeout = connectTimeout
	t.ResponseHeaderTimeout = connectTimeout
	return t
}

func goBin() string {
	if dir := os.Getenv("GOBIN"); dir != "" {
		return dir
	}
	if dir := os.Getenv("GOPATH"); dir != "" {
		return filepath.Join(filepath.SplitList(dir)[0], "bin")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "go", "bin")
}

func runUpdate(ctx context.Context, out io.Writer, o updateOptions) error {
	latest, err := o.src.Latest(ctx)
	if err != nil {
		return fmt.Errorf("can't find the latest release: %w", err)
	}
	if !strings.HasPrefix(o.current, "v") {
		_, _ = fmt.Fprintf(out, "this is a development build (%s); the latest release is %s. See docs/install.md to install it\n", o.current, latest)
		return nil
	}
	if !update.Newer(o.current, latest) {
		_, _ = fmt.Fprintf(out, "mockmachina %s is the latest\n", o.current)
		return nil
	}
	if o.check {
		_, _ = fmt.Fprintf(out, "%s is out (you have %s); run mockmachina update\n", latest, o.current)
		return nil
	}
	return o.install(ctx, out, latest)
}

func (o updateOptions) install(ctx context.Context, out io.Writer, latest string) error {
	how := update.How(o.exe, runtime.GOOS, o.gobin)
	if args := managerCommand(how, latest); args != nil {
		_, _ = fmt.Fprintf(out, "updating with: %s\n", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // fixed package manager commands with a version from the release list
		cmd.Stdout, cmd.Stderr = out, out
		return cmd.Run()
	}
	target := release.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	bar := &progressBar{out: out, label: "downloading mockmachina " + latest}
	if o.animate {
		o.src.Progress = bar.update
	}
	if err := o.src.Replace(ctx, latest, target, o.exe); err != nil {
		if o.animate {
			_, _ = fmt.Fprintln(out)
		}
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("can't replace %s: %w (run it with sudo, or reinstall to a folder you own)", o.exe, err)
		}
		return err
	}
	done := fmt.Sprintf("updated mockmachina %s → %s (%s)", o.current, latest, o.exe)
	if o.animate {
		bar.finish(done)
		return nil
	}
	_, _ = fmt.Fprintln(out, done)
	return nil
}

func managerCommand(how update.Method, latest string) []string {
	switch how {
	case update.Homebrew:
		return []string{"brew", "upgrade", "mockmachina"}
	case update.Scoop:
		return []string{"scoop", "update", "mockmachina"}
	case update.Go:
		return []string{"go", "install", installPackage + "@" + latest}
	case update.Replace:
	}
	return nil
}
