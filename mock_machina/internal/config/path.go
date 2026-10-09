package config

import (
	"fmt"
	"regexp"
	"strings"
)

var paramName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func pathProblem(path string) string {
	if !strings.HasPrefix(path, "/") {
		return fmt.Sprintf("path %q must start with \"/\"", path)
	}
	if strings.ContainsAny(path, "?#") {
		return fmt.Sprintf("path %q can't contain \"?\" or \"#\"; query parameters aren't part of the path", path)
	}
	seen := map[string]bool{}
	for segment := range strings.SplitSeq(path, "/") {
		if msg := segmentProblem(path, segment, seen); msg != "" {
			return msg
		}
	}
	return ""
}

func segmentProblem(path, segment string, seen map[string]bool) string {
	opens, closes := strings.Count(segment, "{"), strings.Count(segment, "}")
	switch {
	case opens == 0 && closes == 0:
		return ""
	case opens > closes:
		return fmt.Sprintf("path %q has an unclosed \"{\"", path)
	case closes > opens:
		return fmt.Sprintf("path %q has a \"}\" without a \"{\"", path)
	case opens > 1 || !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}"):
		return fmt.Sprintf("path %q: a parameter must be a whole segment, like /users/{id}", path)
	}
	name := segment[1 : len(segment)-1]
	switch {
	case name == "":
		return fmt.Sprintf("path %q has a parameter with no name", path)
	case !paramName.MatchString(name):
		return fmt.Sprintf("path %q: parameter %q must use letters, digits and underscores", path, name)
	case seen[name]:
		return fmt.Sprintf("path %q uses parameter %q twice", path, name)
	}
	seen[name] = true
	return ""
}
