package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/certs"
	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
)

func caDir() (string, error) {
	if dir := os.Getenv("MOCKMACHINA_CA_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mockmachina", "ca"), nil
}

func localCA() (*certs.CA, string, bool, error) {
	dir, err := caDir()
	if err != nil {
		return nil, "", false, err
	}
	ca, created, err := certs.LoadOrCreate(dir, clock.Real{}.Now())
	return ca, dir, created, err
}

func tlsConfig(o startOptions, log func(string)) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.tlsCert != "" {
		pair, err := tls.LoadX509KeyPair(o.tlsCert, o.tlsKey)
		if err != nil {
			return nil, fmt.Errorf("can't use --tls-cert and --tls-key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
		return cfg, nil
	}
	ca, dir, created, err := localCA()
	if err != nil {
		return nil, fmt.Errorf("can't set up the local certificate authority: %w", err)
	}
	if created {
		log("created a local certificate authority in " + dir + "; run mockmachina cert --install to trust it")
	}
	addrs, _ := net.InterfaceAddrs()
	host, _ := os.Hostname()
	extra := []string{host}
	if host != "" && !strings.Contains(host, ".") {
		extra = append(extra, host+".local")
	}
	if o.host != "" && o.host != "0.0.0.0" && o.host != "::" {
		extra = append(extra, o.host)
	}
	leaf, err := ca.Leaf(certs.Names(addrs, extra...), clock.Real{}.Now())
	if err != nil {
		return nil, err
	}
	cfg.Certificates = []tls.Certificate{leaf}
	return cfg, nil
}

func newCertCmd() *cobra.Command {
	var printPEM, install bool
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Show, or trust, the local certificate authority behind start --https",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ca, dir, created, err := localCA()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			path := filepath.Join(dir, "rootCA.pem")
			switch {
			case printPEM:
				_, err = out.Write(ca.PEM())
				return err
			case install:
				return installCA(cmd.Context(), out, cmd.ErrOrStderr(), path)
			}
			printCertHelp(out, path, created)
			return nil
		},
	}
	cmd.Flags().BoolVar(&printPEM, "pem", false, "print the CA certificate, to copy to a device")
	cmd.Flags().BoolVar(&install, "install", false, "trust the CA on this computer, and in a booted iOS Simulator")
	return cmd
}

func printCertHelp(out io.Writer, path string, created bool) {
	_, _ = fmt.Fprintf(out, "local certificate authority: %s\n", path)
	if created {
		_, _ = fmt.Fprintln(out, "(created just now)")
	}
	_, _ = fmt.Fprintf(out, `
To use HTTPS:                mockmachina start --https
Trust it on this computer:   mockmachina cert --install
iOS Simulator:               xcrun simctl keychain booted add-root-cert %s
iPhone or iPad:              send rootCA.pem to the device (AirDrop or email), install the profile,
                             then turn on full trust in Settings > General > About > Certificate Trust Settings
Android emulator or phone:   copy rootCA.pem to the device and install it under Settings > Security >
                             Encryption & credentials > Install a certificate > CA certificate; the app also
                             needs a network security config that trusts user certificates (see docs/https.md)
`, path)
}

func installSteps(goos, caPath, keychain string, simulator bool) [][]string {
	switch goos {
	case "darwin":
		steps := [][]string{{"security", "add-trusted-cert", "-r", "trustRoot", "-k", keychain, caPath}}
		if simulator {
			steps = append(steps, []string{"xcrun", "simctl", "keychain", "booted", "add-root-cert", caPath})
		}
		return steps
	case "windows":
		return [][]string{{"certutil", "-user", "-addstore", "Root", caPath}}
	default:
		return [][]string{
			{"sudo", "cp", caPath, "/usr/local/share/ca-certificates/mockmachina.crt"},
			{"sudo", "update-ca-certificates"},
		}
	}
}

func simulatorBooted(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "booted").Output()
	return err == nil && strings.Contains(string(out), "(Booted)")
}

func installCA(ctx context.Context, out, errOut io.Writer, caPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	for _, step := range installSteps(runtime.GOOS, caPath, keychain, runtime.GOOS == "darwin" && simulatorBooted(ctx)) {
		_, _ = fmt.Fprintf(out, "running: %s\n", strings.Join(step, " "))
		cmd := exec.CommandContext(ctx, step[0], step[1:]...) //nolint:gosec // fixed commands, with the CA path from this tool's own config folder
		cmd.Stdout, cmd.Stderr, cmd.Stdin = out, errOut, os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", step[0], err)
		}
	}
	_, _ = fmt.Fprintln(out, "trusted; restart browsers and apps that were already open")
	return nil
}
