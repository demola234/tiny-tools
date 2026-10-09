package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func TestLoadFS_ConfigFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file *string
		want model.Config
	}{
		{"no config.yaml", nil, model.DefaultConfig()},
		{"empty config.yaml", ptr(""), model.DefaultConfig()},
		{
			"every field",
			ptr("version: 1\nhost: 0.0.0.0\nports:\n  mock: 5000\n"),
			model.Config{Version: 1, Host: "0.0.0.0", Ports: model.Ports{Mock: 5000}, Locale: "en"},
		},
		{
			"only the port",
			ptr("ports: { mock: 8080 }\n"),
			model.Config{Version: 1, Host: "127.0.0.1", Ports: model.Ports{Mock: 8080}, Locale: "en"},
		},
		{"localhost", ptr("host: localhost\n"), model.Config{Version: 1, Host: "localhost", Ports: model.Ports{Mock: 4001}, Locale: "en"}},
		{
			"seed and locale",
			ptr("seed: 42\nlocale: en_NG\n"),
			model.Config{Version: 1, Host: "127.0.0.1", Ports: model.Ports{Mock: 4001}, Seed: 42, Locale: "en_NG"},
		},
		{
			"proxy",
			ptr("proxy: https://staging.example.com/api\n"),
			model.Config{Version: 1, Host: "127.0.0.1", Ports: model.Ports{Mock: 4001}, Proxy: "https://staging.example.com/api", Locale: "en"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{}
			if tc.file != nil {
				files["config.yaml"] = *tc.file
			}
			p := loadClean(t, files)
			if diff := cmp.Diff(tc.want, p.Config); diff != "" {
				t.Errorf("config (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadFS_ConfigFileProblems(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, file, want string }{
		{"unknown field", "prots: { mock: 1 }\n", `config.yaml:1: unknown field "prots" (did you mean "ports"?)`},
		{"newer version", "version: 2\n", `config.yaml:1: config version 2 needs a newer mockmachina (this one reads version 1)`},
		{"version zero", "version: 0\n", `config.yaml:1: version must be a whole number from 1, like 1`},
		{"version text", "version: one\n", `config.yaml:1: version must be a whole number from 1, like 1`},
		{"port too big", "ports: { mock: 70000 }\n", `config.yaml:1: ports.mock 70000 isn't a port (1-65535)`},
		{"port text", "ports:\n  mock: abc\n", `config.yaml:2: ports.mock must be a number, like 4001`},
		{"ports not a mapping", "ports: 4001\n", `config.yaml:1: ports must map names to numbers, like { mock: 4001 }`},
		{"ports typo", "ports: { mok: 1 }\n", `config.yaml:1: unknown field "mok" (did you mean "mock"?)`},
		{"negative seed", "seed: -1\n", `config.yaml:1: seed must be a whole number from 0, like 42`},
		{"seed text", "seed: lucky\n", `config.yaml:1: seed must be a whole number from 0, like 42`},
		{"unknown locale", "locale: fr_FR\n", `config.yaml:1: locale "fr_FR" isn't available (locales: en, en_NG)`},
		{"locale case", "locale: en_ng\n", `config.yaml:1: locale "en_ng" isn't available (did you mean "en_NG"?)`},
		{"proxy not a URL", "proxy: staging\n", `config.yaml:1: proxy "staging" must be an http or https URL, like http://localhost:8080`},
		{"proxy with another scheme", "proxy: ftp://x\n", `config.yaml:1: proxy "ftp://x" must be an http or https URL, like http://localhost:8080`},
		{"bad host", "host: not a host!\n", `config.yaml:1: host "not a host!" must be an IP address or localhost, like 127.0.0.1`},
		{"not a mapping", "- a\n", `config.yaml:1: expected config fields (version, host, ports, proxy, seed, locale)`},
		{"invalid YAML", "host: [x\n", `config.yaml:1: invalid YAML: did not find expected ',' or ']'`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, probs := load(t, map[string]string{"config.yaml": tc.file})
			if len(probs) != 1 || probs[0].String() != tc.want {
				t.Errorf("problems = %v, want [%s]", probs, tc.want)
			}
			if p.Config.Ports.Mock == 0 || p.Config.Host == "" {
				t.Errorf("config after a problem = %+v, want defaults filled in", p.Config)
			}
		})
	}
}

func ptr(s string) *string { return &s }
