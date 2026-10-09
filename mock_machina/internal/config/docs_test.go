package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

var (
	docBlock    = regexp.MustCompile("(?s)```yaml (routes|config)\n(.*?)```")
	docBodyFile = regexp.MustCompile(`body: ([\w.-]+\.\w+)`)
)

func TestFileFormatDoc_ExamplesLoad(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join(filepath.Dir(testkit.Path(t)), "docs", "file-format.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := docBlock.FindAllStringSubmatch(string(data), -1)
	if len(blocks) < 3 {
		t.Fatalf("found %d tested examples in file-format.md, want at least 3", len(blocks))
	}
	for i, b := range blocks {
		kind, src := b[1], b[2]
		files := map[string]string{"config.yaml": src}
		if kind == "routes" {
			name := "example" + strconv.Itoa(i)
			files = map[string]string{"routes/" + name + ".yaml": src}
			for _, m := range docBodyFile.FindAllStringSubmatch(src, -1) {
				files["routes/"+name+"/"+m[1]] = "{}"
			}
		}
		if _, probs := load(t, files); len(probs) > 0 {
			t.Errorf("example %d (%s) in file-format.md has problems:\n%s\n%s", i+1, kind, probs, src)
		}
	}
}
