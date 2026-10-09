package cli_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut strings.Builder
	code = cli.Run(t.Context(), cli.Env{Args: args, Stdout: &out, Stderr: &errOut, Info: testInfo})
	return code, out.String(), errOut.String()
}

func TestStateSet_ChangesTheDefault(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	code, stdout, stderr := runCLI(t, "state", "set", "users.list", "empty", "--dir", dir)

	if code != cli.ExitOK || stdout != "users.list: success → empty\n" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "routes", "users.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "active: empty\n") {
		t.Errorf("users.yaml = %q, want active: empty", data)
	}
}

func TestStateSet_AlreadyActive(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	code, stdout, _ := runCLI(t, "state", "set", "users.list", "success", "--dir", dir)
	if code != cli.ExitOK || stdout != "users.list is already success\n" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestStateSet_UnknownState(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	code, stdout, stderr := runCLI(t, "state", "set", "users.list", "emty", "--dir", dir)
	want := "error: route users.list has no state \"emty\" (did you mean \"empty\"?)\n"
	if code != cli.ExitFailure || stdout != "" || stderr != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1 and %q", code, stdout, stderr, want)
	}
}

func TestStateCommands_UsageErrors(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"state", "set", "users.list"},
		{"state", "set", "a", "b", "c"},
		{"state", "sett", "users.list", "empty"},
	}
	for _, args := range tests {
		if code, _, stderr := runCLI(t, args...); code != cli.ExitUsage {
			t.Errorf("mockmachina %s: exit %d (stderr %q), want 2", strings.Join(args, " "), code, stderr)
		}
	}
}

func TestState_NoSubcommandPrintsHelp(t *testing.T) {
	t.Parallel()

	code, stdout, _ := runCLI(t, "state")
	if code != cli.ExitOK || !strings.Contains(stdout, "set") {
		t.Errorf("exit %d, stdout %q; want 0 and help listing set", code, stdout)
	}
}

func TestStateSet_ReachesARunningServer(t *testing.T) {
	t.Parallel()

	dir := copyUsersProject(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, cli.Env{
			Args:   []string{"start", "--dir", dir, "--port", "0", "--watch-interval", "10ms"},
			Stdout: pw,
			Stderr: io.Discard,
			Info:   testInfo,
		})
		_ = pw.Close()
	}()
	lines := readLines(pr)
	base := servingURL.FindString(<-lines)

	if code, _, stderr := runCLI(t, "state", "set", "users.list", "unauthorized", "--dir", dir); code != cli.ExitOK {
		t.Fatalf("state set exit %d: %s", code, stderr)
	}
	expectLine(t, lines, "reloaded after changes to routes/users.yaml (1 route)")
	if status, _ := get(t, base+"/users", ""); status != 401 {
		t.Errorf("GET /users after state set = %d, want 401", status)
	}

	cancel()
	<-done
}
