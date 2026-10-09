package config

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestPairs_StopsWhenTheLoopBreaks(t *testing.T) {
	t.Parallel()

	var node yaml.Node
	if err := yaml.Unmarshal([]byte("a: 1\nb: 2\nc: 3\n"), &node); err != nil {
		t.Fatal(err)
	}
	var seen []string
	for key := range pairs(node.Content[0]) {
		seen = append(seen, key.Value)
		if key.Value == "b" {
			break
		}
	}
	if len(seen) != 2 {
		t.Errorf("visited %v, want to stop after b", seen)
	}
}
