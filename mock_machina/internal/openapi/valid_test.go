package openapi_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func oasSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile(testkit.Path(t, "specs", "oas", "schema-3.1.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("https://spec.openapis.org/oas/3.1/schema/2022-10-07", doc); err != nil {
		t.Fatal(err)
	}
	sc, err := c.Compile("https://spec.openapis.org/oas/3.1/schema/2022-10-07")
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func assertValid(t *testing.T, sc *jsonschema.Schema, name string, exported []byte) {
	t.Helper()
	var v any
	if err := yaml.Unmarshal(exported, &v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := sc.Validate(inst); err != nil {
		t.Errorf("%s: the export isn't valid OpenAPI 3.1:\n%v", name, err)
	}
}

func TestExport_IsValidOpenAPI(t *testing.T) {
	t.Parallel()

	sc := oasSchema(t)
	files, _ := filepath.Glob(testkit.Path(t, "specs", "*.yaml"))
	for _, f := range files {
		res := importSpec(t, filepath.Base(f))
		exported, err := openapi.Export(&model.Project{Routes: res.Routes, Schemas: res.Schemas}, openapi.Info{Title: res.Title})
		if err != nil {
			t.Fatal(err)
		}
		assertValid(t, sc, filepath.Base(f), exported)
	}
	for _, dir := range []string{"shop", "flutter_shop"} {
		p, probs, err := config.Load(filepath.Join(filepath.Dir(testkit.Path(t)), "examples", dir, ".mockmachina"))
		if err != nil || probs.HasErrors() {
			t.Fatalf("%s: %v %v", dir, probs, err)
		}
		exported, err := openapi.Export(p, openapi.Info{Title: dir})
		if err != nil {
			t.Fatal(err)
		}
		assertValid(t, sc, dir, exported)
	}
}
