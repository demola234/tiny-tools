package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

const specV1 = `openapi: 3.1.0
info: { title: Pets, version: "1" }
paths:
  /pets:
    get:
      summary: List pets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pets" }
              example: [{ id: 1, name: Rex }]
  /pets/{id}:
    delete:
      responses:
        "204": { description: gone }
components:
  schemas:
    Pets: { type: array, items: { type: object, properties: { id: { type: integer }, name: { type: string } } } }
`

const specV2 = `openapi: 3.1.0
info: { title: Pets, version: "2" }
paths:
  /pets:
    get:
      summary: List all the pets
      parameters:
        - { name: limit, in: query, schema: { type: integer, minimum: 1 } }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pets" }
              example: [{ id: 1, name: Rex }]
        "500":
          description: broken
          content:
            application/json:
              schema: { type: object, properties: { error: { type: string } } }
  /pets/{id}:
    get:
      responses:
        "200":
          description: one
          content:
            application/json:
              schema: { type: object, properties: { id: { type: integer } } }
components:
  schemas:
    Pets: { type: array, items: { type: object, required: [id], properties: { id: { type: integer }, name: { type: string } } } }
`

func importText(t *testing.T, spec string) *openapi.Result {
	t.Helper()
	res, err := openapi.Import([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestMergeContract(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), config.DirName)
	v1 := importText(t, specV1)
	if _, err := config.WriteContract(dir, v1.Routes, v1.Schemas); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "routes", "pets.yaml")
	edited := strings.Replace(string(readFile(t, file)), "list:\n  route: GET /pets\n", "list:\n  # hand edited\n  route: GET /pets\n  owners: { backend: [ademola] }\n", 1)
	edited = strings.Replace(edited, "    success: {body: [{id: 1, name: Rex}]}\n", "    success: {body: [{id: 1, name: Rex}]}\n    slow: {latency: 2s, body: []}\n", 1)
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	v2 := importText(t, specV2)
	report, err := config.MergeContract(dir, v2.Routes, v2.Schemas, false)
	if err != nil {
		t.Fatal(err)
	}
	want := config.MergeReport{
		Added:     []string{"pets.get"},
		Updated:   []string{"pets.list"},
		NewStates: []string{"pets.list.server_error"},
		NotInSpec: []string{"pets.delete"},
		Files:     []string{"routes/pets.yaml", "schemas/api.yaml"},
	}
	if diff := cmp.Diff(want, report); diff != "" {
		t.Errorf("report (-want +got):\n%s", diff)
	}
	testkit.Golden(t, readFile(t, file), "import/merged-pets.yaml")
	if _, probs, err := config.Load(dir); err != nil || probs.HasErrors() {
		t.Errorf("merged project: %v\n%s", err, probs)
	}
}

func TestMergeContract_DryRunWritesNothing(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), config.DirName)
	v1 := importText(t, specV1)
	if _, err := config.WriteContract(dir, v1.Routes, v1.Schemas); err != nil {
		t.Fatal(err)
	}
	before := string(readFile(t, filepath.Join(dir, "routes", "pets.yaml")))
	report, err := config.MergeContract(dir, importText(t, specV2).Routes, importText(t, specV2).Schemas, true)
	if err != nil || len(report.Added) != 1 {
		t.Fatalf("report %+v, %v", report, err)
	}
	if after := string(readFile(t, filepath.Join(dir, "routes", "pets.yaml"))); after != before {
		t.Error("--dry-run changed the file")
	}
}

func TestMergeContract_SameSpecChangesNothing(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), config.DirName)
	v1 := importText(t, specV1)
	if _, err := config.WriteContract(dir, v1.Routes, v1.Schemas); err != nil {
		t.Fatal(err)
	}
	report, err := config.MergeContract(dir, importText(t, specV1).Routes, importText(t, specV1).Schemas, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added)+len(report.Updated)+len(report.NewStates)+len(report.Files) != 0 {
		t.Errorf("re-importing the same spec reported changes: %+v", report)
	}
}
