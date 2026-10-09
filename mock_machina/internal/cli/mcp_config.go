package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

var mcpClients = []string{"claude-code", "claude-desktop", "cursor", "vscode"}

func checkClient(client string) error {
	if slices.Contains(mcpClients, client) {
		return nil
	}
	if s, ok := suggest.Closest(client, mcpClients); ok {
		return UsageError(fmt.Errorf("unknown client %q (did you mean %q?)", client, s))
	}
	return UsageError(fmt.Errorf("unknown client %q (clients: %s)", client, strings.Join(mcpClients, ", ")))
}

func printMCPConfig(w io.Writer, client, exe string, args []string) {
	switch client {
	case "claude-code":
		words := make([]string, 0, len(args)+1)
		for _, a := range append([]string{exe}, args...) {
			words = append(words, shellQuote(a))
		}
		_, _ = fmt.Fprintf(w, "claude mcp add mockmachina -- %s\n", strings.Join(words, " "))
	case "vscode":
		_, _ = fmt.Fprintf(w, "{\n  \"servers\": {\n    \"mockmachina\": {\n      \"type\": \"stdio\",\n      \"command\": %s,\n      \"args\": %s\n    }\n  }\n}\n",
			jsonString(exe), jsonList(args))
	default:
		_, _ = fmt.Fprintf(w, "{\n  \"mcpServers\": {\n    \"mockmachina\": {\n      \"command\": %s,\n      \"args\": %s\n    }\n  }\n}\n",
			jsonString(exe), jsonList(args))
	}
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

func jsonList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = jsonString(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
