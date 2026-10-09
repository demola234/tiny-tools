package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func schemaPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(testkit.Path(t)), "schema", name)
}

func readSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(schemaPath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s is not JSON: %v", name, err)
	}
	return doc
}

func compile(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(schemaPath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(name, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(name)
	if err != nil {
		t.Fatalf("%s doesn't compile: %v", name, err)
	}
	return sch
}

func stringKeys(v any) any {
	switch v := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[fmt.Sprint(k)] = stringKeys(val)
		}
		return out
	case map[string]any:
		for k, val := range v {
			v[k] = stringKeys(val)
		}
		return v
	case []any:
		for i, val := range v {
			v[i] = stringKeys(val)
		}
		return v
	}
	return v
}

func yamlInstance(t *testing.T, src string) any {
	t.Helper()
	var v any
	if err := yaml.Unmarshal([]byte(src), &v); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(stringKeys(v))
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

func keys(t *testing.T, v any, path ...string) []string {
	t.Helper()
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("schema has no %v", path)
		}
		v = m[p]
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("schema has no object at %v", path)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

func TestSchemas_MatchTheLoader(t *testing.T) {
	t.Parallel()

	route := readSchema(t, "route.schema.json")
	config := readSchema(t, "config.schema.json")
	tests := []struct {
		name   string
		schema []string
		loader []string
	}{
		{"file fields", keys(t, route, "properties"), fileFields},
		{"route fields", keys(t, route, "$defs", "route", "properties"), routeFields},
		{"state fields", keys(t, route, "$defs", "state", "properties"), stateFields},
		{"owner fields", keys(t, route, "$defs", "owners", "properties"), ownerFields},
		{"config fields", keys(t, config, "properties"), configFields},
		{"port fields", keys(t, config, "properties", "ports", "properties"), portFields},
	}
	for _, tc := range tests {
		if !slices.Equal(tc.schema, sorted(tc.loader)) {
			t.Errorf("%s: schema has %v, loader has %v", tc.name, tc.schema, sorted(tc.loader))
		}
	}
	listed := route["$defs"].(map[string]any)["route"].(map[string]any)["required"].([]any)
	required := make([]string, 0, len(listed))
	for _, r := range listed {
		required = append(required, r.(string))
	}
	if !slices.Equal(sorted(required), sorted(requiredRouteFields)) {
		t.Errorf("required: schema has %v, loader has %v", required, requiredRouteFields)
	}
}

func TestRouteSchema_AcceptsGoodRoutes(t *testing.T) {
	t.Parallel()

	sch := compile(t, "route.schema.json")
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(testkit.Path(t)), "examples", "*", ".mockmachina", "routes", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, testkit.Path(t, "projects", "users", ".mockmachina", "routes", "users.yaml"))
	full := `
owners: { backend: [ademola], frontend: [ada] }
status: agreed
x-team: payments
get:
  route: GET /users/{id}
  summary: One user
  status: implemented
  owners: { backend: [tunde] }
  x-note: v2
  active: found
  states:
    found:
      status: 200
      headers: { Cache-Control: no-store, X-Count: 5 }
      body: found.json
      latency: 800ms
      x-note: happy path
    empty:
      body: { users: [] }
    gone:
list:
  route: GET /users
  states:
    ok:
`
	if err := sch.Validate(yamlInstance(t, full)); err != nil {
		t.Errorf("a route using every field is rejected: %v", err)
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := sch.Validate(yamlInstance(t, string(data))); err != nil {
			t.Errorf("%s is rejected: %v", p, err)
		}
	}
	if len(paths) < 2 {
		t.Errorf("checked only %d route files; are the examples missing?", len(paths))
	}
}

func TestRouteSchema_RejectsMistakes(t *testing.T) {
	t.Parallel()

	sch := compile(t, "route.schema.json")
	tests := map[string]string{
		"unknown route field":  "get:\n  route: GET /r\n  rotue: x\n  states:\n    ok: {}\n",
		"lowercase method":     "get:\n  route: get /r\n  states:\n    ok: {}\n",
		"route without path":   "get:\n  route: GET\n  states:\n    ok: {}\n",
		"path without slash":   "get:\n  route: GET r\n  states:\n    ok: {}\n",
		"route status":         "get:\n  route: GET /r\n  status: approved\n  states:\n    ok: {}\n",
		"file status":          "status: approved\nget:\n  route: GET /r\n  states:\n    ok: {}\n",
		"bad route name":       "Get Users:\n  route: GET /r\n  states:\n    ok: {}\n",
		"state name":           "get:\n  route: GET /r\n  states:\n    Server Error: {}\n",
		"keyword state name":   "get:\n  route: GET /r\n  states:\n    'no': {}\n",
		"HTTP status range":    "get:\n  route: GET /r\n  states:\n    ok:\n      status: 999\n",
		"unknown state field":  "get:\n  route: GET /r\n  states:\n    ok:\n      bdoy: {}\n",
		"missing states":       "get:\n  route: GET /r\n",
		"latency without unit": "get:\n  route: GET /r\n  states:\n    ok:\n      latency: '800'\n",
		"owners as a list":     "owners: [a]\nget:\n  route: GET /r\n  states:\n    ok: {}\n",
	}
	for name, src := range tests {
		if err := sch.Validate(yamlInstance(t, src)); err == nil {
			t.Errorf("%s: accepted, want rejected", name)
		}
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	sch := compile(t, "config.schema.json")
	if err := sch.Validate(yamlInstance(t, "version: 1\nhost: 0.0.0.0\nports: { mock: 5000 }\n")); err != nil {
		t.Errorf("a full config is rejected: %v", err)
	}
	for name, src := range map[string]string{
		"unknown field":   "prots: { mock: 1 }\n",
		"port too big":    "ports: { mock: 70000 }\n",
		"version zero":    "version: 0\n",
		"unknown in port": "ports: { mok: 1 }\n",
	} {
		if err := sch.Validate(yamlInstance(t, src)); err == nil {
			t.Errorf("%s: accepted, want rejected", name)
		}
	}
}
