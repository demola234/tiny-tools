package model_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func checkNames(t *testing.T, name string, fn func(string) bool, valid, invalid []string) {
	t.Helper()
	for _, s := range valid {
		if !fn(s) {
			t.Errorf("%s(%q) = false, want true", name, s)
		}
	}
	for _, s := range invalid {
		if fn(s) {
			t.Errorf("%s(%q) = true, want false", name, s)
		}
	}
}

func TestValidStateName(t *testing.T) {
	t.Parallel()
	checkNames(t, "ValidStateName", model.ValidStateName,
		[]string{"success", "server_error", "e2", "none", "nothing", "yes_please"},
		[]string{
			"", "Server", "1x", "a-b", "_x", "server error",
			"true", "false", "null", "yes", "no", "on", "off", "y", "n",
		})
}

func TestIsYAMLKeyword(t *testing.T) {
	t.Parallel()
	checkNames(t, "IsYAMLKeyword", model.IsYAMLKeyword,
		[]string{"true", "false", "null", "yes", "no", "on", "off", "y", "n"},
		[]string{"", "none", "nope", "True", "ok"})
}
