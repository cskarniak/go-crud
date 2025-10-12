// config/config.go

package config

import (
	"example.com/go-crud/config/loader"
)

type ServerConfig struct {
	Port           string   `yaml:"port"`
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type DatabaseConfig struct {
	Directory string `yaml:"directory"`
	Name      string `yaml:"name"`
}

type GeneralConfig struct {
	DefaultPerPage int `yaml:"default_per_page"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	General  GeneralConfig  `yaml:"general"`
	Admin    AdminConfig    `yaml:"admin"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	err := loader.Load(path, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}