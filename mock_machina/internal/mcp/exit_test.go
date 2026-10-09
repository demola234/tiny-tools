package mcp_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mockmachina")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", bin, "./cmd/mockmachina")
	cmd.Dir = filepath.Dir(testkit.Path(t))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func launch(t *testing.T, bin string, args ...string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "exit-test", Version: "v0"}, nil)
	cmd := exec.CommandContext(t.Context(), bin, append([]string{"mcp"}, args...)...)
	cs, err := client.Connect(t.Context(), &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestExitPhase4_Stdio(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := shop(t)
	cs := launch(t, bin, "--dir", dir)
	res := call(t, cs, "add_state", map[string]any{
		"route": "users.list", "name": "server_error", "status": 500, "body": rawJSON(`{"error":"try again"}`),
	})
	if res.IsError {
		t.Fatalf("add_state: %s", text(res))
	}
	out, err := exec.CommandContext(t.Context(), bin, "lint", "--dir", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("lint failed after add_state: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `users.list state "server_error" was written by an AI assistant`) {
		t.Errorf("lint output doesn't flag the new state for review:\n%s", out)
	}
	if !strings.Contains(readRoutes(t, dir, "users.yaml"), "    server_error:\n      status: 500\n      generated: true\n") {
		t.Errorf("users.yaml:\n%s", readRoutes(t, dir, "users.yaml"))
	}
}

func TestExitPhase4_ReadOnly(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := shop(t)
	before := snapshot(t, dir)
	cs := launch(t, bin, "--dir", dir, "--read-only")
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if !tool.Annotations.ReadOnlyHint {
			t.Errorf("--read-only offers %s", tool.Name)
		}
		call(t, cs, tool.Name, map[string]any{"id": "users.get", "url": "http://127.0.0.1:1"})
	}
	if _, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "set_state", Arguments: map[string]any{"route": "users.get", "state": "found"}}); err == nil {
		t.Error("set_state worked in a read-only session")
	}
	if after := snapshot(t, dir); !slices.Equal(before, after) {
		t.Errorf("project changed in a read-only session:\n%v\n%v", before, after)
	}
}

func snapshot(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files = append(files, path+"\n"+string(data))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
