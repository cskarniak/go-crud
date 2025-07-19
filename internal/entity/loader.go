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

// --- Types de config combo_base ---
type ComboFieldConfig struct {
    SQL           string   `yaml:"sql"`
    KeyField      string   `yaml:"keyField"`
    DisplayFields []string `yaml:"displayFields"`
    Separator     string   `yaml:"separator"`
}

// --- Types de config vision ---
type VisionConfig struct {
    SQL           string   `yaml:"sql"`
    KeyField      string   `yaml:"keyField"`
    DisplayFields []string `yaml:"displayFields"`
    ReturnField   string   `yaml:"returnField"`
    ModalTitle    string   `yaml:"modalTitle"`
}

// FieldDef représente un champ simple, combo_base ou vision
type FieldDef struct {
    Name         string
    Type         string
    ComboConfig  *ComboFieldConfig
    VisionConfig *VisionConfig
}

// UnmarshalYAML gère à la fois:
//  - le simple "- name: ..." (type string par défaut)
//  - combo_base
//  - vision
func (f *FieldDef) UnmarshalYAML(node *yaml.Node) error {
    // décoder la map brute
    var raw map[string]yaml.Node
    if err := node.Decode(&raw); err != nil {
        return err
    }

    // name (obligatoire)
    if n, ok := raw["name"]; ok {
        if err := n.Decode(&f.Name); err != nil {
            return err
        }
    } else {
        return fmt.Errorf("field entry missing 'name'")
    }

    // type (défaut "string")
    f.Type = "string"
    if t, ok := raw["type"]; ok {
        if err := t.Decode(&f.Type); err != nil {
            return err
        }
    }

    // combo_base ?
    if f.Type == "combo_base" {
        cfg := &ComboFieldConfig{}
        if n, ok := raw["sql"]; ok {
            if err := n.Decode(&cfg.SQL); err != nil {
                return err
            }
        } else {
            return fmt.Errorf("combo_base %s: missing sql", f.Name)
        }
        if n, ok := raw["keyField"]; ok {
            if err := n.Decode(&cfg.KeyField); err != nil {
                return err
            }
        }
        if n, ok := raw["displayFields"]; ok {
            if err := n.Decode(&cfg.DisplayFields); err != nil {
                return err
            }
        }
        if n, ok := raw["separator"]; ok {
            if err := n.Decode(&cfg.Separator); err != nil {
                return err
            }
        }
        f.ComboConfig = cfg
    }

    // vision ?
    if f.Type == "vision" {
        vc := &VisionConfig{}
        if n, ok := raw["sql"]; ok {
            if err := n.Decode(&vc.SQL); err != nil {
                return err
            }
        } else {
            return fmt.Errorf("vision %s: missing sql", f.Name)
        }
        if n, ok := raw["keyField"]; ok {
            if err := n.Decode(&vc.KeyField); err != nil {
                return err
            }
        }
        if n, ok := raw["displayFields"]; ok {
            if err := n.Decode(&vc.DisplayFields); err != nil {
                return err
            }
        }
        if n, ok := raw["returnField"]; ok {
            if err := n.Decode(&vc.ReturnField); err != nil {
                return err
            }
        }
        if n, ok := raw["modalTitle"]; ok {
            if err := n.Decode(&vc.ModalTitle); err != nil {
                return err
            }
        }
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
    Name             string
    PageSize         int
    DefaultSortField string
    DefaultSortOrder string
    PageSizeOptions  []int
    Columns          []string
    SearchableFields []string
    SortableFields   []string
    Labels           map[string]string
}

// Field décrit un champ d'entité stocké en base
type Field struct {
    Name     string
    Label    string
    Type     string
    ReadOnly bool
    Required bool
    // Valeur par défaut si rien n'est saisi
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
    Code            *form_codes.FormCode
}

// yamlEntity reflète vos fichiers config/entities/*.yaml
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
            Labels           map[string]string `yaml:"labels"`
            Groups           []struct {
                Name   string      `yaml:"name"`
                Fields []yaml.Node `yaml:"fields"`
            } `yaml:"groups"`
        } `yaml:"config"`
    } `yaml:"forms"`
}

// LoadEntityConfig lit le YAML d’entité, l’analyse, puis charge le form_code.
func LoadEntityConfig(path string) (*EntityConfig, error) {
    // 1) lire et parse le YAML
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
    }

    // 2) Fields
    for i, f := range y.Fields {
        ec.Fields[i] = Field{
            Name:     f.Name,
            Label:    f.Label,
            Type:     f.Type,
            ReadOnly: f.ReadOnly,
            Required: f.Required,
            Default:  f.Default,
        }
    }

    // 3) Forms
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
                Labels:           form.Config.Labels,
            }
        case "fiche":
            fc := FicheConfig{
                Name:   form.Name,
                Labels: form.Config.Labels,
            }
            for _, grp := range form.Config.Groups {
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
        }
    }

    // 4) Valeurs par défaut génériques
    if ec.DefaultPageSize == 0 {
        ec.DefaultPageSize = 10
    }
    if ec.List.DefaultSortField == "" && len(ec.Fields) > 0 {
        ec.List.DefaultSortField = ec.Fields[0].Name
    }
    if ec.List.DefaultSortOrder == "" {
        ec.List.DefaultSortOrder = "asc"
    }

    // 5) Charger le form_code si existant
    codePath := filepath.Join("config", "form_codes", ec.Fiche.Name+"_code.yaml")
    if fc, err := form_codes.LoadFormCode(codePath); err != nil {
        log.Printf("Aucun form_code pour %s: %v", ec.Fiche.Name, err)
    } else {
        ec.Code = fc
    }

    return ec, nil
}
