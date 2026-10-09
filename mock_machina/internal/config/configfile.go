package config

import (
	"io/fs"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

const (
	configFile = "config.yaml"
	maxPort    = 65535
)

var (
	configFields = []string{"version", "host", "ports", "proxy", "seed", "locale"}
	portFields   = []string{"mock"}
)

func (l *loader) config() model.Config {
	cfg := model.DefaultConfig()
	data, err := fs.ReadFile(l.fsys, configFile)
	if err != nil {
		return cfg
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		l.yamlError(configFile, err)
		return cfg
	}
	if len(doc.Content) == 0 {
		return cfg
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		l.add(configFile, root.Line, "expected config fields (%s)", strings.Join(configFields, ", "))
		return cfg
	}
	f := l.fields(configFile, root, configFields, nil)
	if v := f["version"]; v != nil {
		l.version(v, &cfg)
	}
	if v := f["host"]; v != nil {
		l.host(v, &cfg)
	}
	if v := f["ports"]; v != nil {
		l.ports(v, &cfg)
	}
	if v := f["proxy"]; v != nil {
		l.proxy(v, &cfg)
	}
	if v := f["seed"]; v != nil {
		l.seed(v, &cfg)
	}
	if v := f["locale"]; v != nil {
		l.locale(v, &cfg)
	}
	return cfg
}

func (l *loader) version(v *yaml.Node, cfg *model.Config) {
	var n int
	switch {
	case v.Decode(&n) != nil || n < 1:
		l.add(configFile, v.Line, "version must be a whole number from 1, like 1")
	case n > model.ConfigVersion:
		l.add(configFile, v.Line, "config version %d needs a newer mockmachina (this one reads version %d)", n, model.ConfigVersion)
	default:
		cfg.Version = n
	}
}

func (l *loader) host(v *yaml.Node, cfg *model.Config) {
	if v.Value != "localhost" && net.ParseIP(v.Value) == nil {
		l.add(configFile, v.Line, "host %q must be an IP address or localhost, like 127.0.0.1", v.Value)
		return
	}
	cfg.Host = v.Value
}

func (l *loader) ports(v *yaml.Node, cfg *model.Config) {
	if v.Kind != yaml.MappingNode {
		l.add(configFile, v.Line, "ports must map names to numbers, like { mock: 4001 }")
		return
	}
	mock := l.fields(configFile, v, portFields, nil)["mock"]
	if mock == nil {
		return
	}
	var port int
	switch {
	case mock.Decode(&port) != nil:
		l.add(configFile, mock.Line, "ports.mock must be a number, like 4001")
	case port < 1 || port > maxPort:
		l.add(configFile, mock.Line, "ports.mock %d isn't a port (1-%d)", port, maxPort)
	default:
		cfg.Ports.Mock = port
	}
}

func (l *loader) proxy(v *yaml.Node, cfg *model.Config) {
	if problem := ProxyProblem(v.Value); problem != "" {
		l.add(configFile, v.Line, "proxy %s", problem)
		return
	}
	cfg.Proxy = v.Value
}

func ProxyProblem(target string) string {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return strconv.Quote(target) + " must be an http or https URL, like http://localhost:8080"
	}
	return ""
}

func (l *loader) seed(v *yaml.Node, cfg *model.Config) {
	n, err := strconv.ParseUint(v.Value, 10, 64)
	if v.Kind != yaml.ScalarNode || err != nil {
		l.add(configFile, v.Line, "seed must be a whole number from 0, like 42")
		return
	}
	cfg.Seed = n
}

func (l *loader) locale(v *yaml.Node, cfg *model.Config) {
	locales := model.Locales()
	if slices.Contains(locales, v.Value) {
		cfg.Locale = v.Value
		return
	}
	l.add(configFile, v.Line, "locale %q isn't available (%s)", v.Value, closeMatchOr(v.Value, locales, "locales: "+strings.Join(locales, ", ")))
}
