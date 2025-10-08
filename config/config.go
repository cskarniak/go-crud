package config

import (
    "io/ioutil"
    "log"

    "gopkg.in/yaml.v3"
)

type ServerConfig struct {
    Port           string   `yaml:"port"`
    TrustedProxies []string `yaml:"trusted_proxies"`
}

type DatabaseConfig struct {
    Path string `yaml:"path"`
}

type GeneralConfig struct {
    DatabaseDir    string `yaml:"DATABASE_DIR"`
    DatabaseName   string `yaml:"DATABASE_NAME"`
    APIPrefix      string `yaml:"API_PREFIX"`
    DefaultPerPage int    `yaml:"DEFAULT_PER_PAGE"`
    SecretKey      string `yaml:"SECRET_KEY"`
}

type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    General  GeneralConfig  `yaml:",inline"`
}

var Cfg Config

func Load() {
    data, err := ioutil.ReadFile("config/config.yaml")
    if err != nil {
        log.Fatalf("Impossible de lire config/config.yaml : %v", err)
    }
    if err := yaml.Unmarshal(data, &Cfg); err != nil {
        log.Fatalf("Impossible de parser config/config.yaml : %v", err)
    }
}
