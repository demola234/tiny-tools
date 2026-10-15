package archtest_test

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const module = "github.com/demola234/tiny-tools/mock_machina/"

var allowed = map[string][]string{
	"cmd/mockmachina":    {"internal/buildinfo", "internal/cli"},
	"internal/buildinfo": {},
	"internal/cli": {
		"internal/buildinfo", "internal/certs", "internal/clock", "internal/config", "internal/control", "internal/diff", "internal/docs", "internal/gitfs", "internal/live", "internal/mcp", "internal/model", "internal/openapi",
		"internal/release", "internal/seed", "internal/server", "internal/suggest", "internal/tui", "internal/update", "internal/watch", "github.com/spf13/cobra", "google.golang.org/grpc", "charm.land/fang/v2", "charm.land/bubbletea/v2", "charm.land/lipgloss/v2", "github.com/charmbracelet/x/term",
	},
	"internal/control": {"internal/clock", "internal/config", "internal/model", "internal/server", "internal/suggest"},
	"internal/diff":    {"internal/model"},
	"internal/gitfs":   {},
	"internal/live":    {"internal/diff", "internal/model"},
	"internal/certs":   {},
	"internal/clock":   {},
	"internal/mcp": {
		"internal/config", "internal/diff", "internal/live", "internal/model", "internal/suggest",
		"github.com/google/jsonschema-go/jsonschema", "github.com/modelcontextprotocol/go-sdk/mcp",
	},
	"internal/config":   {"internal/gitfs", "internal/model", "internal/protos", "internal/schema", "internal/suggest", "internal/tmpl", "go.yaml.in/yaml/v3", "google.golang.org/protobuf/reflect/protoreflect"},
	"internal/match":    {"internal/model"},
	"internal/model":    {},
	"internal/openapi":  {"internal/model", "go.yaml.in/yaml/v3"},
	"internal/release":  {},
	"internal/protos":   {"github.com/bufbuild/protocompile", "github.com/bufbuild/protocompile/linker", "github.com/bufbuild/protocompile/reporter", "google.golang.org/protobuf/encoding/protojson", "google.golang.org/protobuf/proto", "google.golang.org/protobuf/reflect/protodesc", "google.golang.org/protobuf/reflect/protoreflect", "google.golang.org/protobuf/reflect/protoregistry", "google.golang.org/protobuf/types/dynamicpb"},
	"internal/update":   {"internal/clock", "internal/release"},
	"tools/release":     {"internal/release"},
	"internal/schema":   {"internal/model", "github.com/santhosh-tekuri/jsonschema/v6", "github.com/santhosh-tekuri/jsonschema/v6/kind", "golang.org/x/text/language", "golang.org/x/text/message"},
	"internal/seed":     {},
	"internal/server":   {"internal/clock", "internal/match", "internal/model", "internal/protos", "internal/schema", "internal/seed", "internal/suggest", "internal/tmpl", "github.com/coder/websocket", "github.com/santhosh-tekuri/jsonschema/v6", "google.golang.org/grpc", "google.golang.org/grpc/codes", "google.golang.org/grpc/metadata", "google.golang.org/grpc/reflection", "google.golang.org/grpc/reflection/grpc_reflection_v1", "google.golang.org/grpc/status", "google.golang.org/protobuf/reflect/protodesc", "google.golang.org/protobuf/reflect/protoreflect", "google.golang.org/protobuf/reflect/protoregistry"},
	"internal/suggest":  {},
	"internal/docs":     {},
	"internal/fake":     {},
	"internal/tmpl":     {"internal/fake", "internal/match", "internal/model", "internal/seed", "internal/suggest"},
	"internal/testkit":  {"github.com/google/go-cmp/cmp"},
	"internal/tui":      {"internal/model", "charm.land/bubbletea/v2", "charm.land/lipgloss/v2", "github.com/charmbracelet/x/ansi"},
	"internal/watch":    {"internal/clock"},
	"internal/archtest": {},
}

type pkg struct {
	ImportPath string
	Imports    []string
}

func listPackages(t *testing.T) []pkg {
	t.Helper()
	root := filepath.Dir(testkit.Path(t))
	touchSources(t, root)
	cmd := exec.CommandContext(t.Context(), "go", "list", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []pkg
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var p pkg
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return pkgs
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
}

func touchSources(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		_, err = os.Stat(path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isStandard(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

func TestImports_FollowThePackageMap(t *testing.T) {
	t.Parallel()

	for _, p := range listPackages(t) {
		name := strings.TrimPrefix(p.ImportPath, module)
		rules, known := allowed[name]
		if !known {
			t.Errorf("%s isn't in the package map; add it with the imports it's allowed", name)
			continue
		}
		for _, imp := range p.Imports {
			if isStandard(imp) {
				continue
			}
			if !slices.Contains(rules, strings.TrimPrefix(imp, module)) {
				t.Errorf("%s imports %s, which the package map doesn't allow", name, imp)
			}
		}
	}
}

func TestImports_TestkitOnlyFromTests(t *testing.T) {
	t.Parallel()

	for _, p := range listPackages(t) {
		if slices.Contains(p.Imports, module+"internal/testkit") {
			t.Errorf("%s imports testkit outside its tests", p.ImportPath)
		}
	}
}
