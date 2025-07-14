// internal/entity/loader.go
package entity

import (
    "fmt"
    "log"
    "path/filepath"

    "example.com/go-crud/config/form_codes"
    "example.com/go-crud/config/loader"
)

// Field décrit un champ d'entité
type Field struct {
    Name     string
    Label    string
    Type     string
    ReadOnly bool
    Required bool
}

// ListConfig configuration pour la liste
type ListConfig struct {
    Name             string
    PageSize         int
    DefaultSortField string
    DefaultSortOrder string
    PageSizeOptions  []int
    Columns          []string
    SearchableFields []string
    SortableFields   []string
}

// Group de champs dans la fiche
type Group struct {
    Name   string
    Fields []string
}

// FicheConfig configuration pour la fiche
type FicheConfig struct {
    Name   string
    Groups []Group
    Labels map[string]string
}

// EntityConfig structure riche basée sur votre YAML d’entité
type EntityConfig struct {
    Name            string
    Table           string
    Label           string
    LabelPlural     string
    DefaultPageSize int
    Fields          []Field
    List            ListConfig
    Fiche           FicheConfig

    // Code de pré‐remplissage et validations (optionnel)
    Code *form_codes.FormCode
}

// yamlEntity reflète la structure de votre YAML categories.yaml
type yamlEntity struct {
    Entity struct {
        Name            string `yaml:"name"`
        Table           string `yaml:"table"`
        Label           string `yaml:"label"`
        LabelPlural     string `yaml:"labelPlural"`
        DefaultPageSize int    `yaml:"defaultPageSize"`
    } `yaml:"entity"`
    Fields []struct {
        Name     string `yaml:"name"`
        Type     string `yaml:"type"`
        Label    string `yaml:"label"`
        ReadOnly bool   `yaml:"readonly,omitempty"`
        Required bool   `yaml:"required,omitempty"`
    } `yaml:"fields"`
    Forms []struct {
        Name   string `yaml:"name"`
        Type   string `yaml:"type"`
        Config struct {
            PageSize         int               `yaml:"pageSize"`
            DefaultSortField string            `yaml:"defaultSortField"`
            DefaultSortOrder string            `yaml:"defaultSortOrder"`
            PageSizeOptions  []int             `yaml:"pageSizeOptions"`
            Columns          []string          `yaml:"columns"`
            SearchableFields []string          `yaml:"searchableFields"`
            SortableFields   []string          `yaml:"sortableFields"`
            Groups           []Group           `yaml:"groups"`
            Labels           map[string]string `yaml:"labels"`
        } `yaml:"config"`
    } `yaml:"forms"`
}

// LoadEntityConfig lit le YAML d’entité, construit EntityConfig,
// puis tente de charger le form‐code associé (pré‐remplissage & validations).
func LoadEntityConfig(path string) (*EntityConfig, error) {
    // 1) Charger le YAML d’entité
    var y yamlEntity
    if err := loader.Load(path, &y); err != nil {
        return nil, fmt.Errorf("échec chargement entité %s: %w", path, err)
    }

    // 2) Transformer en EntityConfig
    ec := &EntityConfig{
        Name:            y.Entity.Name,
        Table:           y.Entity.Table,
        Label:           y.Entity.Label,
        LabelPlural:     y.Entity.LabelPlural,
        DefaultPageSize: y.Entity.DefaultPageSize,
        Fields:          make([]Field, len(y.Fields)),
    }
    for i, f := range y.Fields {
        ec.Fields[i] = Field{
            Name:     f.Name,
            Label:    f.Label,
            Type:     f.Type,
            ReadOnly: f.ReadOnly,
            Required: f.Required,
        }
    }

    // 3) Initialiser les forms list et fiche
    for _, form := range y.Forms {
        switch form.Type {
        case "list":
            ec.List = ListConfig{
                Name:             form.Name,
                PageSize:         form.Config.PageSize,
                DefaultSortField: form.Config.DefaultSortField,
                DefaultSortOrder: form.Config.DefaultSortOrder,
                PageSizeOptions:  form.Config.PageSizeOptions,
                Columns:          form.Config.Columns,
                SearchableFields: form.Config.SearchableFields,
                SortableFields:   form.Config.SortableFields,
            }
        case "fiche":
            ec.Fiche = FicheConfig{
                Name:   form.Name,
                Groups: form.Config.Groups,
                Labels: form.Config.Labels,
            }
        }
    }

    // 4) Valeurs par défaut si manquantes
    if ec.DefaultPageSize == 0 {
        ec.DefaultPageSize = 10
    }
    if ec.List.DefaultSortField == "" && len(ec.Fields) > 0 {
        ec.List.DefaultSortField = ec.Fields[0].Name
    }
    if ec.List.DefaultSortOrder == "" {
        ec.List.DefaultSortOrder = "asc"
    }

    // 5) Charger le form_code associé (s’il existe)
    codePath := filepath.Join("config", "form_codes", ec.Fiche.Name+"_code.yaml")
    if fc, err := form_codes.LoadFormCode(codePath); err != nil {
        log.Printf("Aucun form code pour %s : %v", ec.Fiche.Name, err)
    } else {
        ec.Code = fc
    }

    return ec, nil
}
