package model

import (
	"regexp"
	"slices"
	"strings"
)

var stateNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var yamlKeywords = []string{"true", "false", "null", "yes", "no", "on", "off", "y", "n"}

func IsYAMLKeyword(s string) bool { return slices.Contains(yamlKeywords, s) }

func ValidStateName(s string) bool {
	return stateNamePattern.MatchString(s) && !IsYAMLKeyword(s)
}

var notNameChars = regexp.MustCompile(`[^a-z0-9]+`)

func RouteNames(method, routePath string) (resource, name string) {
	segments := strings.Split(strings.Trim(routePath, "/"), "/")
	endsWithParam := strings.HasPrefix(segments[len(segments)-1], "{")
	resource = "root"
	for _, s := range segments {
		if s != "" && !strings.HasPrefix(s, "{") {
			resource = strings.Trim(notNameChars.ReplaceAllString(strings.ToLower(s), "-"), "-")
		}
	}
	names := map[Method]string{
		MethodPost: "create", MethodPut: "replace", MethodPatch: "update",
		MethodDelete: "delete", MethodHead: "head", MethodOptions: "options",
	}
	name, ok := names[Method(method)]
	switch {
	case ok:
	case endsWithParam:
		name = "get"
	default:
		name = "list"
	}
	return resource, name
}
