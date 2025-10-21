// internal/entity/loader.go
package entity

import (
	"fmt"
	"log"
	"path/filepath"

	"example.com/go-crud/config/form_codes"
	"example.com/go-crud/config/loader"
	"gopkg.in/yaml.v3"
)

// --- Types de config pour les champs de formulaire ---
type ComboFieldConfig struct {
	SQL           string   `yaml:"sql"`
	KeyField      string   `yaml:"keyField"`
	DisplayFields []string `yaml:"displayFields"`
	Separator     string   `yaml:"separator"`
}

// VisionFieldConfig est pour le CHAMP de type vision (popup)
type VisionFieldConfig struct {
	SQL           string   `yaml:"sql"`
	KeyField      string   `yaml:"keyField"`
	DisplayFields []string `yaml:"displayFields"`
	ReturnField   string   `yaml:"returnField"`
	ModalTitle    string   `yaml:"modalTitle"`
}

// --- NOUVEAUX types de config pour le FORMULAIRE de type 'vision' ---

// VisionParamConfig définit un paramètre pour une requête de formulaire 'vision'.
type VisionParamConfig struct {
	Name         string `yaml:"name"`
	Source       string `yaml:"source"`       // "context" ou "literal"
	ContextField string `yaml:"contextField"` // si source est "context"
	Value        string `yaml:"value"`        // si source est "literal"
}

// VisionActionsConfig définit les actions autorisées sur un formulaire 'vision'.
type VisionActionsConfig struct {
	AllowCreate     bool  `yaml:"allowCreate"`
	AllowUpdate     bool  `yaml:"allowUpdate"`
	AllowDelete     bool  `yaml:"allowDelete"`
	AllowSelectable *bool `yaml:"allowSelectable,omitempty"` // V29 - Pointeur pour gérer l'absence de valeur
}

// VisionFormConfig est la configuration pour un FORMULAIRE de type 'vision'.
type VisionFormConfig struct {
	Name    string              `yaml:"name"`
	Type    string              `yaml:"type"`
	SQL     string              `yaml:"sql"`
	Params  []VisionParamConfig `yaml:"params"`
	Actions VisionActionsConfig `yaml:"actions"`

	// Champs réutilisés de ListConfig pour l'affichage
	Columns          []string          `yaml:"columns"`
	Labels           map[string]string `yaml:"labels"`
	DefaultSortField string            `yaml:"defaultSortField"`
	DefaultSortOrder string            `yaml:"defaultSortOrder"`
	PageSize         int               `yaml:"pageSize"`
	PageSizeOptions  []int             `yaml:"pageSizeOptions"`
}

// FieldDef représente un champ simple, combo_base ou vision
type FieldDef struct {
	Name         string
	Type         string
	ComboConfig  *ComboFieldConfig
	VisionConfig *VisionFieldConfig
	VisionButton string `yaml:"visionButton,omitempty"` // V28 - Bouton pour ouvrir un formulaire vision
}

func (f *FieldDef) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if n, ok := raw["name"]; ok {
		if err := n.Decode(&f.Name); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("field entry missing 'name'")
	}

	// V28 - Décodage du bouton vision
	if vb, ok := raw["visionButton"]; ok {
		if err := vb.Decode(&f.VisionButton); err != nil {
			return err
		}
	}

	f.Type = "string"
	if t, ok := raw["type"]; ok {
		if err := t.Decode(&f.Type); err != nil {
			return err
		}
	}
	if f.Type == "combo_base" {
		cfg := &ComboFieldConfig{}
		node.Decode(cfg)
		f.ComboConfig = cfg
	} else if f.Type == "vision" {
		vc := &VisionFieldConfig{}
		node.Decode(vc)
		f.VisionConfig = vc
	}
	return nil
}

// Group de champs pour la fiche
type Group struct {
	Name   string
	Fields []FieldDef
}

// FicheConfig configuration pour la fiche
type FicheConfig struct {
	Name   string
	Groups []Group
	Labels map[string]string
}

// ListConfig configuration pour la liste
type ListConfig struct {
	Name             string            `yaml:"name"`
	PageSize         int               `yaml:"pageSize"`
	DefaultSortField string            `yaml:"defaultSortField"`
	DefaultSortOrder string            `yaml:"defaultSortOrder"`
	PageSizeOptions  []int             `yaml:"pageSizeOptions"`
	Columns          []string          `yaml:"columns"`
	SearchableFields []string          `yaml:"searchableFields"`
	SortableFields   []string          `yaml:"sortableFields"`
	Labels           map[string]string `yaml:"labels"`
}

// Field décrit un champ d'entité stocké en base
type Field struct {
	Name     string
	Label    string
	Type     string
	ReadOnly bool
	Required bool
	Default  interface{}
}

// EntityConfig regroupe tout le config d’une entité
type EntityConfig struct {
	Name            string
	Table           string
	Label           string
	LabelPlural     string
	DefaultPageSize int
	Fields          []Field
	List            ListConfig
	Fiche           FicheConfig
	VisionForms     map[string]VisionFormConfig // Map pour stocker les formulaires 'vision'
	Code            *form_codes.FormCode
}

// yamlEntity reflète la structure des fichiers YAML
type yamlEntity struct {
	Entity struct {
		Name            string `yaml:"name"`
		Table           string `yaml:"table"`
		Label           string `yaml:"label"`
		LabelPlural     string `yaml:"labelPlural"`
		DefaultPageSize int    `yaml:"defaultPageSize,omitempty"`
	} `yaml:"entity"`
	Fields []struct {
		Name     string      `yaml:"name"`
		Type     string      `yaml:"type,omitempty"`
		Label    string      `yaml:"label"`
		ReadOnly bool        `yaml:"readonly,omitempty"`
		Required bool        `yaml:"required,omitempty"`
		Default  interface{} `yaml:"default,omitempty"`
	} `yaml:"fields"`
	Forms []struct {
		Name   string    `yaml:"name"`
		Type   string    `yaml:"type"`
		Config yaml.Node `yaml:"config"` // Utiliser yaml.Node pour un décodage flexible
	} `yaml:"forms"`
}

// LoadEntityConfig lit et analyse la configuration d'une entité.
func LoadEntityConfig(path string) (*EntityConfig, error) {
	var y yamlEntity
	if err := loader.Load(path, &y); err != nil {
		return nil, fmt.Errorf("échec chargement %s : %w", path, err)
	}

	ec := &EntityConfig{
		Name:            y.Entity.Name,
		Table:           y.Entity.Table,
		Label:           y.Entity.Label,
		LabelPlural:     y.Entity.LabelPlural,
		DefaultPageSize: y.Entity.DefaultPageSize,
		Fields:          make([]Field, len(y.Fields)),
		VisionForms:     make(map[string]VisionFormConfig), // Initialiser la map
	}

	for i, f := range y.Fields {
		ec.Fields[i] = Field{f.Name, f.Label, f.Type, f.ReadOnly, f.Required, f.Default}
	}

	for _, form := range y.Forms {
		switch form.Type {
		case "list":
			var listCfg ListConfig
			if err := form.Config.Decode(&listCfg); err != nil {
				return nil, fmt.Errorf("erreur décodage 'list' form %s: %w", form.Name, err)
			}
			listCfg.Name = form.Name
			ec.List = listCfg
		case "fiche":
			var ficheNode struct { // Structure temporaire pour le décodage
				Labels map[string]string `yaml:"labels"`
				Groups []struct {
					Name   string      `yaml:"name"`
					Fields []yaml.Node `yaml:"fields"`
				} `yaml:"groups"`
			}
			if err := form.Config.Decode(&ficheNode); err != nil {
				return nil, fmt.Errorf("erreur décodage 'fiche' form %s: %w", form.Name, err)
			}
			fc := FicheConfig{Name: form.Name, Labels: ficheNode.Labels}
			for _, grp := range ficheNode.Groups {
				g := Group{Name: grp.Name}
				for _, node := range grp.Fields {
					var fd FieldDef
					if err := node.Decode(&fd); err != nil {
						return nil, fmt.Errorf("fields decoding: %w", err)
					}
					g.Fields = append(g.Fields, fd)
				}
				fc.Groups = append(fc.Groups, g)
			}
			ec.Fiche = fc
		case "vision":
			var visionCfg VisionFormConfig
			if err := form.Config.Decode(&visionCfg); err != nil {
				return nil, fmt.Errorf("erreur décodage 'vision' form %s: %w", form.Name, err)
			}
			visionCfg.Name = form.Name
			visionCfg.Type = form.Type
			ec.VisionForms[form.Name] = visionCfg
		}
	}

	// Valeurs par défaut génériques
	if ec.List.PageSize == 0 {
		if ec.DefaultPageSize != 0 {
			ec.List.PageSize = ec.DefaultPageSize
		} else {
			ec.List.PageSize = 10
		}
	}
	if ec.List.DefaultSortField == "" && len(ec.Fields) > 0 {
			ec.List.DefaultSortField = ec.Fields[0].Name
	}
	if ec.List.DefaultSortOrder == "" {
		ec.List.DefaultSortOrder = "asc"
	}

	// Charger le form_code si existant
	codePath := filepath.Join("config", "form_codes", ec.Fiche.Name+"_code.yaml")
	if fc, err := form_codes.LoadFormCode(codePath); err != nil {
		log.Printf("Aucun form_code pour %s: %v", ec.Fiche.Name, err)
	} else {
		ec.Code = fc
	}

	return ec, nil
}
