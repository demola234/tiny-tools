package live

import (
	"fmt"
	"maps"
	"slices"

	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
)

const (
	maxFindings = 20
	maxDepth    = 32
)

type finding struct {
	severity diff.Severity
	message  string
}

func compareShape(expected, actual any) []finding {
	w := &walker{}
	w.walk("", expected, actual, 0)
	if extra := w.total - maxFindings; extra > 0 {
		w.out = append(w.out, finding{diff.Info, fmt.Sprintf("and %d more differences", extra)})
	}
	return w.out
}

type walker struct {
	out   []finding
	total int
}

func (w *walker) add(s diff.Severity, format string, args ...any) {
	w.total++
	if len(w.out) < maxFindings {
		w.out = append(w.out, finding{s, fmt.Sprintf(format, args...)})
	}
}

func (w *walker) walk(path string, expected, actual any, depth int) {
	if depth > maxDepth || expected == nil {
		return
	}
	if want, got := kind(expected), kind(actual); want != got {
		if actual == nil {
			w.add(diff.Warning, "%s is null live, %s in the contract", subject(path), want)
			return
		}
		w.add(diff.Breaking, "%s is %s live, %s in the contract", subject(path), got, want)
		return
	}
	switch e := expected.(type) {
	case map[string]any:
		w.object(path, e, actual.(map[string]any), depth)
	case []any:
		if a := actual.([]any); len(e) > 0 && len(a) > 0 {
			w.walk(path+"[]", e[0], a[0], depth+1)
		}
	}
}

func (w *walker) object(path string, expected, actual map[string]any, depth int) {
	for _, k := range slices.Sorted(maps.Keys(expected)) {
		child := join(path, k)
		if v, ok := actual[k]; ok {
			w.walk(child, expected[k], v, depth+1)
			continue
		}
		w.add(diff.Breaking, "field %q is missing live", child)
	}
	for _, k := range slices.Sorted(maps.Keys(actual)) {
		if _, ok := expected[k]; !ok {
			w.add(diff.Info, "live response has extra field %q", join(path, k))
		}
	}
}

func kind(v any) string {
	switch v.(type) {
	case map[string]any:
		return "an object"
	case []any:
		return "a list"
	case string:
		return "a string"
	case float64:
		return "a number"
	case bool:
		return "a boolean"
	default:
		return "null"
	}
}

func subject(path string) string {
	if path == "" {
		return "the response"
	}
	return fmt.Sprintf("field %q", path)
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
