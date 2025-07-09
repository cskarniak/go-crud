// config/config.go
package config

import (
    "io/ioutil"
    "log"

    "gopkg.in/yaml.v3"
)

// Server contient la config du serveur
type Server struct {
    Port           string   `yaml:"port"`
    TrustedProxies []string `yaml:"trusted_proxies"`
}

// Database contient la config de la base de données
type Database struct {
    Path string `yaml:"path"`
}

// Config regroupe l’ensemble des paramètres
type Config struct {
    Server   Server   `yaml:"server"`
    Database Database `yaml:"database"`
}

// Conf est la config chargée en mémoire
var Conf Config

// Init charge config/config.yaml dans Conf
func Init() {
    data, err := ioutil.ReadFile("config/config.yaml")
    if err != nil {
        log.Fatalf("⚠️ Impossible de lire config/config.yaml : %v", err)
    }
    if err := yaml.Unmarshal(data, &Conf); err != nil {
        log.Fatalf("⚠️ Impossible de parser config/config.yaml : %v", err)
    }
}
