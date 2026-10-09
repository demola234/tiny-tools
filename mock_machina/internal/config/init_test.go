package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInit_CreatesALoadableProject(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), config.DirName)
	created, err := config.Init(dir, config.InitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(created, ",") != "config.yaml,routes/health.yaml" {
		t.Errorf("created %v, want config.yaml and routes/health.yaml", created)
	}
	testkit.Golden(t, readFile(t, filepath.Join(dir, "config.yaml")), "init/config.yaml")
	testkit.Golden(t, readFile(t, filepath.Join(dir, "routes", "health.yaml")), "init/health.yaml")

	p, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 {
		t.Fatalf("Load after Init = %v, %v", probs, err)
	}
	if len(p.Routes) != 1 || p.Routes[0].ID != "health.get" {
		t.Errorf("routes = %v, want health.get", p.Routes)
	}
}

func TestInit_Port(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), config.DirName)
	if _, err := config.Init(dir, config.InitOptions{Port: 5000}); err != nil {
		t.Fatal(err)
	}
	p, _, err := config.Load(dir)
	if err != nil || p.Config.Ports.Mock != 5000 {
		t.Errorf("port = %d, %v; want 5000", p.Config.Ports.Mock, err)
	}
}

func TestInit_NoExample(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), config.DirName)
	created, err := config.Init(dir, config.InitOptions{NoExample: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(created, ",") != "config.yaml" {
		t.Errorf("created %v, want only config.yaml", created)
	}
	p, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 || len(p.Routes) != 0 {
		t.Errorf("Load = %v routes, %v, %v; want an empty, clean project", len(p.Routes), probs, err)
	}
}

func TestInit_RefusesAnExistingProject(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile})
	_, err := config.Init(dir, config.InitOptions{})
	if !errors.Is(err, config.ErrProjectExists) {
		t.Fatalf("Init over a project = %v, want ErrProjectExists", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want it to mention --force", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "config.yaml")); statErr == nil {
		t.Error("Init wrote config.yaml into an existing project")
	}
}

func TestInit_ForceRewritesOnlyItsOwnFiles(t *testing.T) {
	t.Parallel()

	dir := project(t, map[string]string{"users.yaml": usersFile, "health.yaml": "old"})
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("ports: { mock: 9 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(dir, config.InitOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	testkit.Golden(t, readFile(t, filepath.Join(dir, "config.yaml")), "init/config.yaml")
	testkit.Golden(t, readFile(t, filepath.Join(dir, "routes", "health.yaml")), "init/health.yaml")
	if got := routeFile(t, dir, "users.yaml"); got != usersFile {
		t.Error("Init --force changed a route file it didn't create")
	}
}
