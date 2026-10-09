package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func importInto(t *testing.T, spec string) (string, []string) {
	t.Helper()
	data, err := os.ReadFile(testkit.Path(t, "specs", spec))
	if err != nil {
		t.Fatal(err)
	}
	res, err := openapi.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), config.DirName)
	written, err := config.WriteContract(dir, res.Routes, res.Schemas)
	if err != nil {
		t.Fatal(err)
	}
	return dir, written
}

func TestWriteContract_Golden(t *testing.T) {
	t.Parallel()

	dir, written := importInto(t, "awkward-3.1.yaml")
	if got := strings.Join(written, " "); got != "routes/orders.yaml routes/tree.yaml schemas/api.yaml" {
		t.Errorf("written = %s", got)
	}
	for _, f := range written {
		testkit.Golden(t, readFile(t, filepath.Join(dir, filepath.FromSlash(f))), "import/awkward/"+f)
	}
}

func TestWriteContract_CorpusLoadsClean(t *testing.T) {
	t.Parallel()

	files, _ := filepath.Glob(testkit.Path(t, "specs", "*.yaml"))
	for _, f := range files {
		dir, _ := importInto(t, filepath.Base(f))
		if _, probs, err := config.Load(dir); err != nil || probs.HasErrors() {
			t.Errorf("%s: %v\n%s", filepath.Base(f), err, probs)
		}
	}
}

func TestWriteContract_PostmanLoadsWithReviewWarnings(t *testing.T) {
	t.Parallel()

	dir, _ := importInto(t, "shop.postman_collection.json")
	p, probs, err := config.Load(dir)
	if err != nil || probs.HasErrors() {
		t.Fatalf("%v\n%s", err, probs)
	}
	var got []string
	for _, w := range config.Warnings(p) {
		if strings.Contains(w.Msg, "guessed") {
			got = append(got, w.String())
		}
	}
	want := "routes/users.yaml:2: warning: users.list response 200 has a schema guessed from examples; check its required and nullable fields, then delete x-mockmachina-inferred"
	if !slices.Contains(got, want) {
		t.Errorf("warnings %q\nwant one to be %q", got, want)
	}
}

func TestWriteContract_RefusesToOverwrite(t *testing.T) {
	t.Parallel()

	dir, _ := importInto(t, "petstore-3.0.yaml")
	data, _ := os.ReadFile(testkit.Path(t, "specs", "petstore-3.0.yaml"))
	res, _ := openapi.Import(data)
	_, err := config.WriteContract(dir, res.Routes, res.Schemas)
	if err == nil || err.Error() != "routes/pets.yaml, schemas/api.yaml already exist" {
		t.Errorf("error = %v", err)
	}
}
