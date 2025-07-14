// config/form_codes/loader.go
package form_codes

import (
    "fmt"
    "io/ioutil"

    "gopkg.in/yaml.v3"
)

// PrepopulateSpec décrit comment pré‐remplir un champ.
type PrepopulateSpec struct {
    Type   string `yaml:"type"`
    Format string `yaml:"format"`
}

// FrontValidation règle HTML5/JS à injecter sur <input>.
type FrontValidation struct {
    Required bool   `yaml:"required"`
    Pattern  string `yaml:"pattern"`
    Title    string `yaml:"title"`
}

// BackValidation règle à appliquer côté serveur, avec messages.
type BackValidation struct {
    Required        bool   `yaml:"required"`
    RequiredMessage string `yaml:"required_message"`

    Min        int    `yaml:"min"`
    MinMessage string `yaml:"min_message"`

    Max        int    `yaml:"max"`
    MaxMessage string `yaml:"max_message"`
}

// FormCode regroupe la config de pré‐remplissage et de validation.
type FormCode struct {
    Form             string                         `yaml:"form"`
    Prepopulate      map[string]PrepopulateSpec     `yaml:"prepopulate"`
    FrontValidations map[string]FrontValidation     `yaml:"front_validations"`
    BackValidations  map[string]BackValidation      `yaml:"back_validations"`
}

// LoadFormCode lit et parse le YAML du code de formulaire.
func LoadFormCode(path string) (*FormCode, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("impossible de lire %s : %w", path, err)
    }
    var cfg FormCode
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("impossible de parser %s : %w", path, err)
    }
    return &cfg, nil
}
