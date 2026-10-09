package model_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	want := model.Config{Version: 1, Host: "127.0.0.1", Ports: model.Ports{Mock: 4001}, Locale: "en"}
	if got := model.DefaultConfig(); got != want {
		t.Errorf("DefaultConfig() = %+v, want %+v", got, want)
	}
	if model.ConfigVersion != 1 {
		t.Errorf("ConfigVersion = %d, want 1", model.ConfigVersion)
	}
}
