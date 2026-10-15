package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

const petsSpec = `openapi: 3.1.0
info: { title: Pets, version: "1" }
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pets" }
components:
  schemas:
    Pets: { type: array, items: { type: object, properties: { name: { type: string } } } }
`

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImport_RenamesSchemasTheProjectAlreadyHas(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	put(t, filepath.Join(dir, "routes", "health.yaml"), "get:\n  route: GET /health\n  states:\n    up: {}\n")
	put(t, filepath.Join(dir, "schemas", "orders.yaml"), "Pets: { type: object }\n")
	spec := filepath.Join(root, "pets-spec-2.0.yaml")
	put(t, spec, petsSpec)

	var out strings.Builder
	if err := runImport(&out, spec, dir, false); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "renamed schema Pets → PetsSpecPets (schemas/orders.yaml already has Pets)") {
		t.Errorf("output doesn't explain the rename:\n%s", out.String())
	}
	proj, probs, err := config.Load(dir)
	if err != nil || len(probs) > 0 {
		t.Fatalf("project after import: %v %v", probs, err)
	}
	r, ok := proj.Route("pets.list")
	if !ok || r.Responses[0].Schema.Name != "PetsSpecPets" {
		t.Errorf("imported route should use the renamed schema: %+v", r)
	}

	out.Reset()
	if err := runImport(&out, spec, dir, false); err != nil || !strings.Contains(out.String(), "already up to date") {
		t.Errorf("importing the same spec again should change nothing: %v\n%s", err, out.String())
	}
}

func TestSchemaPrefix(t *testing.T) {
	t.Parallel()

	for file, want := range map[string]string{
		"swagger-petstore-3.0.yaml":    "SwaggerPetstore",
		"/specs/payments_api.v2.json":  "PaymentsApi",
		"openapi.yaml":                 "Openapi",
		"3.0.yaml":                     "Imported",
		"shop.postman_collection.json": "ShopPostmanCollection",
	} {
		if got := schemaPrefix(file); got != want {
			t.Errorf("schemaPrefix(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestFindSpecs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	put(t, filepath.Join(root, "api", "openapi.yaml"), "openapi: 3.1.0\ninfo: {}\n")
	put(t, filepath.Join(root, "old", "swagger.json"), `{"swagger": "2.0", "info": {}}`)
	put(t, filepath.Join(root, "shop.postman_collection.json"), `{"info": {"_postman_id": "x", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"}}`)
	put(t, filepath.Join(root, "docker-compose.yml"), "services: {}\n")
	put(t, filepath.Join(root, "node_modules", "pkg", "openapi.yaml"), "openapi: 3.0.0\n")
	put(t, filepath.Join(root, ".mockmachina", "schemas", "api.yaml"), "openapi: 3.0.0\n")

	want := []string{"api/openapi.yaml", "old/swagger.json", "shop.postman_collection.json"}
	if diff := cmp.Diff(want, findSpecs(root)); diff != "" {
		t.Errorf("findSpecs (-want +got):\n%s", diff)
	}
}

func TestImport_NoFileOutsideATerminal(t *testing.T) {
	t.Parallel()

	err := noFileError([]string{"api/openapi.yaml", "petstore.json"})
	for _, want := range []string{"import needs a FILE", "api/openapi.yaml", "petstore.json"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v; want %q", err, want)
		}
	}
	if err := noFileError(nil); err == nil || !strings.Contains(err.Error(), "no OpenAPI, Swagger or Postman file") {
		t.Errorf("err = %v", err)
	}
}
