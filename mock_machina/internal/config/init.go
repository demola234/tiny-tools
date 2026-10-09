package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var ErrProjectExists = errors.New("project already exists")

type InitOptions struct {
	Port      int
	NoExample bool
	Force     bool
}

const schemaBase = "https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/schema/"

const exampleRoutes = `get:
  route: GET /health
  summary: Is the API up?
  states:
    up:
      body: { status: up }
    down:
      status: 503
      body: { status: down }
`

func Init(dir string, opts InitOptions) ([]string, error) {
	if _, err := os.Stat(dir); err == nil && !opts.Force {
		return nil, fmt.Errorf("%w: %s (pass --force to rewrite its config.yaml and example route)", ErrProjectExists, dir)
	}
	defaults := model.DefaultConfig()
	port := defaults.Ports.Mock
	if opts.Port != 0 {
		port = opts.Port
	}
	files := []generatedFile{{
		name: configFile,
		content: schemaLine("config") + "version: " + strconv.Itoa(model.ConfigVersion) + "\n" +
			"host: " + defaults.Host + "\n" +
			"ports:\n  mock: " + strconv.Itoa(port) + "\n",
	}}
	if !opts.NoExample {
		files = append(files, generatedFile{routesDir + "/health.yaml", schemaLine("route") + exampleRoutes})
	}
	if err := os.MkdirAll(filepath.Join(dir, routesDir), 0o755); err != nil {
		return nil, err
	}
	created := make([]string, 0, len(files))
	for _, f := range files {
		if err := WriteFileAtomic(filepath.Join(dir, filepath.FromSlash(f.name)), []byte(f.content), 0o644); err != nil {
			return nil, err
		}
		created = append(created, f.name)
	}
	return created, nil
}

type generatedFile struct{ name, content string }

func schemaLine(kind string) string {
	return "# yaml-language-server: $schema=" + schemaBase + kind + ".schema.json\n"
}
