package config

import "github.com/demola234/tiny-tools/mock_machina/internal/suggest"

func closeMatchOr(got string, candidates []string, fallback string) string {
	if s, ok := suggest.Closest(got, candidates); ok {
		return `did you mean "` + s + `"?`
	}
	return fallback
}
