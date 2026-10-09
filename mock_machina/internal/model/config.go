package model

import "slices"

const ConfigVersion = 1

type Config struct {
	Version int
	Host    string
	Ports   Ports
	Proxy   string
	Seed    uint64
	Locale  string
}

var locales = []string{"en", "en_NG"}

func Locales() []string { return slices.Clone(locales) }

type Ports struct {
	Mock int
}

func DefaultConfig() Config {
	return Config{Version: ConfigVersion, Host: "127.0.0.1", Ports: Ports{Mock: 4001}, Locale: "en"}
}
