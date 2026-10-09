package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing/fstest"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type AddOptions struct {
	Name      string
	Summary   string
	Generated bool
	States    []NewState
}

type Added struct {
	ID          string
	File        string
	CreatedFile bool
}

func AddRoute(dir, method, routePath string, opts AddOptions) (Added, error) {
	resource, name, err := addNames(method, routePath, opts)
	if err != nil {
		return Added{}, err
	}
	proj, probs, err := Load(dir)
	if err != nil {
		return Added{}, fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	file := path.Join(routesDir, resource+resourceExt)
	if own := problemsIn(probs, file); len(own) > 0 {
		return Added{}, fmt.Errorf("%s has problems; fix them first:\n  %s", file, strings.ReplaceAll(own.String(), "\n", "\n  "))
	}
	key := (&model.Route{Method: model.Method(method), Path: routePath}).Key()
	for _, r := range proj.Routes {
		if r.Key() == key {
			return Added{}, fmt.Errorf("%s is already defined by route %s", key, r.ID)
		}
	}
	id := resource + "." + name
	if _, taken := proj.Route(id); taken {
		return Added{}, fmt.Errorf("route %s already exists in %s (pass --name to pick another)", id, file)
	}

	block, bodies, err := routeBlock(name, method, routePath, opts)
	if err != nil {
		return Added{}, err
	}
	full := filepath.Join(dir, filepath.FromSlash(file))
	existing, err := os.ReadFile(full)
	created := errors.Is(err, fs.ErrNotExist)
	if err != nil && !created {
		return Added{}, err
	}
	content := appended(existing, block)
	if created {
		content = []byte(schemaLine("route") + block)
	}
	if len(opts.States) > 0 {
		if err := checkNewRoute(dir, file, resource, content, bodies, created); err != nil {
			return Added{}, err
		}
	}
	added := Added{ID: id, File: file, CreatedFile: created}
	return added, writeWithBodies(full, content, filepath.Join(dir, routesDir, resource), bodies)
}

func checkNewRoute(dir, file, resource string, content []byte, bodies map[string][]byte, created bool) error {
	overlay := fstest.MapFS{file: {Data: content}}
	for name, data := range bodies {
		overlay[path.Join(routesDir, resource, name)] = &fstest.MapFile{Data: data}
	}
	if created {
		return checkNew("route", file, overlay)
	}
	return checkNew("route", file, overlayFS{FS: os.DirFS(dir), files: overlay})
}

func addNames(method, routePath string, opts AddOptions) (resource, name string, err error) {
	if msg := methodProblem(method); msg != "" {
		return "", "", errors.New(msg)
	}
	if msg := pathProblem(routePath); msg != "" {
		return "", "", errors.New(msg)
	}
	if strings.Contains(opts.Summary, "\n") {
		return "", "", errors.New("summary must be one line of text")
	}
	resource, name = model.RouteNames(method, routePath)
	if opts.Name != "" {
		name = opts.Name
	}
	if !resourceName.MatchString(name) {
		return "", "", fmt.Errorf("route name %q must use lowercase letters, digits and dashes", name)
	}
	return resource, name, nil
}

func routeBlock(name, method, routePath string, opts AddOptions) (string, map[string][]byte, error) {
	var b strings.Builder
	b.WriteString(name + ":\n  route: " + method + " " + routePath + "\n")
	if opts.Summary != "" {
		quoted, _ := yaml.Marshal(opts.Summary)
		b.WriteString("  summary: " + string(quoted))
	}
	if opts.Generated {
		b.WriteString("  status: draft\n  generated: true\n")
	}
	b.WriteString("  states:\n")
	if len(opts.States) == 0 {
		b.WriteString(defaultState(model.Method(method)))
		return b.String(), nil, nil
	}
	bodies := map[string][]byte{}
	seen := map[string]bool{}
	for _, st := range opts.States {
		if seen[st.Name] {
			return "", nil, fmt.Errorf("state %q is listed twice", st.Name)
		}
		seen[st.Name] = true
		ins, err := newInsertion(st, name)
		if err != nil {
			return "", nil, err
		}
		text, err := ins.render(2*defaultIndent, defaultIndent, "\n")
		if err != nil {
			return "", nil, err
		}
		b.Write(text)
		if ins.bodyFile != "" {
			bodies[ins.bodyFile] = ins.bodyData
		}
	}
	return b.String(), bodies, nil
}

func defaultState(method model.Method) string {
	switch method {
	case model.MethodPost:
		return "    created:\n      status: 201\n      body: {}\n"
	case model.MethodDelete:
		return "    deleted:\n      status: 204\n"
	default:
		return "    success:\n      body: {}\n"
	}
}

func appended(existing []byte, block string) []byte {
	eol := eolOf(existing)
	var b bytes.Buffer
	b.Write(existing)
	if !bytes.HasSuffix(existing, []byte("\n")) {
		b.WriteString(eol)
	}
	b.WriteString(eol)
	b.WriteString(strings.ReplaceAll(block, "\n", eol))
	return b.Bytes()
}
