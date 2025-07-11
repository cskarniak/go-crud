package entity

import (
	"fmt"
	"io/ioutil"
	"gopkg.in/yaml.v3"
)

// EntityConfig représente la configuration d'une entité métier.
type EntityConfig struct {
	Name             string
	Table            string
	Label            string
	LabelPlural      string
	DefaultPageSize  int
	DefaultSortField string
	DefaultSortOrder string
	Fields           []Field
	List             ListConfig
	Fiche            FicheConfig
}

// Field décrit un champ d'une entité.
type Field struct {
	Name     string
	Label    string
	Type     string
	ReadOnly bool
	Required bool
}

// ListConfig contient la configuration pour la vue "list".
type ListConfig struct {
	PageSize         int
	DefaultSortField string
	DefaultSortOrder string
	PageSizeOptions  []int
	Columns          []string
	SearchableFields []string
	SortableFields   []string
}

// FicheConfig contient la configuration pour la vue "fiche".
type FicheConfig struct {
	Groups []Group
	Labels map[string]string
}

// Group permet de regrouper plusieurs champs dans la fiche.
type Group struct {
	Name   string
	Fields []string
}

// helper correspondant à la structure du YAML.
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

// LoadEntityConfig lit un fichier YAML et retourne la configuration de l'entité.
func LoadEntityConfig(path string) (*EntityConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lecture du fichier %s échouée: %w", path, err)
	}

	var y yamlEntity
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, fmt.Errorf("parsing YAML %s échoué: %w", path, err)
	}

	// Construire EntityConfig
	ec := EntityConfig{
		Name:            y.Entity.Name,
		Table:           y.Entity.Table,
		Label:           y.Entity.Label,
		LabelPlural:     y.Entity.LabelPlural,
		DefaultPageSize: y.Entity.DefaultPageSize,
		Fields:          make([]Field, len(y.Fields)),
	}

	// Copier les champs
	for i, f := range y.Fields {
		ec.Fields[i] = Field{f.Name, f.Label, f.Type, f.ReadOnly, f.Required}
	}

	// Parcourir les forms
	for _, form := range y.Forms {
		switch form.Type {
		case "list":
			ec.List = ListConfig{
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
				Groups: form.Config.Groups,
				Labels: form.Config.Labels,
			}
		}
	}

	// Valeurs par défaut si nécessaire
	if ec.DefaultPageSize == 0 {
		ec.DefaultPageSize = 10
	}
	if ec.List.DefaultSortField == "" {
		ec.List.DefaultSortField = "id"
	}
	if ec.List.DefaultSortOrder == "" {
		ec.List.DefaultSortOrder = "asc"
	}

	return &ec, nil
}
