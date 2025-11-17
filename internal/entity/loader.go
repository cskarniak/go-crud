// internal/entity/loader.go
package entity

import (
	"fmt"
	"log"
	"path/filepath"
	"reflect"

	"example.com/go-crud/config/form_codes"
	"example.com/go-crud/config/loader"
	"gopkg.in/yaml.v3"
)

// PopupConfig définit le style des fenêtres modales (popups).
type PopupConfig struct {
	Width                 string   `yaml:"width"`
	MaxWidth              string   `yaml:"maxWidth"`
	MaxHeight             string   `yaml:"maxHeight"`
	HeaderBackgroundColor string   `yaml:"headerBackgroundColor"`
	HeaderTextColor       string   `yaml:"headerTextColor"`
	TitleFontSize         string   `yaml:"titleFontSize"`
	BodyBackgroundColor   string   `yaml:"bodyBackgroundColor"`
	SearchFontSize        string   `yaml:"searchFontSize"`
	ColumnWidths          []string `yaml:"columnWidths"`
	ColumnAlignments      []string `yaml:"columnAlignments"`
	HeaderFontSize        string   `yaml:"headerFontSize"`
	RowHeight             string   `yaml:"rowHeight"`
	RowFontSize           string   `yaml:"rowFontSize"`
	RowFontColor          string   `yaml:"rowFontColor"`
	EvenRowColor          string   `yaml:"evenRowColor"`
	OddRowColor           string   `yaml:"oddRowColor"`
	TableHoverColor       string   `yaml:"tableHoverColor"`
	ShowSelectButton      bool     `yaml:"showSelectButton"`
	SelectButtonLabel     string   `yaml:"selectButtonLabel"`
}

// Variables globales pour stocker les configurations par défaut
var (
	defaultListConfig  ListConfig
	defaultFicheConfig FicheConfig
	DefaultPopupConfig PopupConfig
)

// init est exécuté une seule fois au démarrage pour charger les configurations par défaut.
func init() {
	if err := loader.Load("config/defaults/list.yaml", &defaultListConfig); err != nil {
		log.Printf("Attention : impossible de charger la configuration par défaut pour les listes : %v", err)
	}
	if err := loader.Load("config/defaults/fiche.yaml", &defaultFicheConfig); err != nil {
		log.Printf("Attention : impossible de charger la configuration par défaut pour les fiches : %v", err)
	}
	if err := loader.Load("config/defaults/popup.yaml", &DefaultPopupConfig); err != nil {
		log.Printf("Attention : impossible de charger la configuration par défaut pour les popups : %v", err)
	}
}

// --- Types de config pour les champs de formulaire ---
type ComboFieldConfig struct {
	SQL           string   `yaml:"sql"`
	KeyField      string   `yaml:"keyField"`
	DisplayFields []string `yaml:"displayFields"`
	Separator     string   `yaml:"separator"`
}

// VisionFieldConfig est pour le CHAMP de type vision (popup)
type VisionFieldConfig struct {
	SQL                string   `yaml:"sql"`
	KeyField           string   `yaml:"keyField"`
	DisplayFields      []string `yaml:"displayFields"`
	ReturnField        string   `yaml:"returnField"`
	ReturnFieldDisplay string   `yaml:"returnFieldDisplay"`
	ModalTitle         string   `yaml:"modalTitle"`
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
	// Temporary comment to force re-parsing
}

// FieldDef représente un champ dans un formulaire (fiche)
type FieldDef struct {
	Name               string             `yaml:"name"`
	Type               string             `yaml:"type,omitempty"`
	ReadOnly           bool               `yaml:"readonly,omitempty"`
	ComboConfig        *ComboFieldConfig  `yaml:"comboConfig,omitempty"`
	VisionConfig       *VisionFieldConfig `yaml:"visionConfig,omitempty"`
	VisionButton       string             `yaml:"visionButton,omitempty"`
	MaxLength          int                `yaml:"maxLength,omitempty"`
	Size               int                `yaml:"size,omitempty"`
	Rows               int                `yaml:"rows,omitempty"`
	Decimals           int                `yaml:"decimals,omitempty"`
	DecimalSeparator   string             `yaml:"decimalSeparator,omitempty"`
	ThousandsSeparator string             `yaml:"thousandsSeparator,omitempty"`
	Align              string             `yaml:"align,omitempty"`
}

func (f *FieldDef) UnmarshalYAML(node *yaml.Node) error {
	// Décodage en map pour gérer les champs simples (string) ou complexes (map)
	var s string
	if err := node.Decode(&s); err == nil {
		f.Name = s
		return nil
	}

	// Si ce n'est pas une simple chaîne, on décode comme une map
	type rawFieldDef FieldDef // Alias pour éviter la récursion infinie
	if err := node.Decode((*rawFieldDef)(f)); err != nil {
		return err
	}
	return nil
}

// Group de champs pour la fiche
type Group struct {
	Name   string     `yaml:"name"`
	Fields []FieldDef `yaml:"fields"`
}

// FicheConfig configuration pour la fiche
type FicheConfig struct {
	Name                       string            `yaml:"name"`
	Groups                     []Group           `yaml:"groups"`
	Titles                     map[string]string `yaml:"titles,omitempty"`      // NOUVEAU
	ButtonLabels               map[string]string `yaml:"buttonLabels,omitempty"` // NOUVEAU
	Width                      string            `yaml:"width,omitempty"`
	MaxWidth                   string            `yaml:"maxWidth,omitempty"`
	FormBackgroundColor        string            `yaml:"formBackgroundColor,omitempty"`
	PageBackgroundColor        string            `yaml:"pageBackgroundColor,omitempty"`
	TabInactiveBackgroundColor string            `yaml:"tabInactiveBackgroundColor,omitempty"`
	TabActiveBackgroundColor   string            `yaml:"tabActiveBackgroundColor,omitempty"`
	TabContentBackgroundColor  string            `yaml:"tabContentBackgroundColor,omitempty"`
	LabelColumnWidth           string            `yaml:"labelColumnWidth,omitempty"`
	TabLabelFontSize           string            `yaml:"tabLabelFontSize,omitempty"`
	ButtonFontSize             string            `yaml:"buttonFontSize,omitempty"`
	FormActionButtonsFontSize  string            `yaml:"formActionButtonsFontSize,omitempty"`
	FormContentMaxHeightAdjustment string        `yaml:"formContentMaxHeightAdjustment,omitempty"`
}

// GetAllFields renvoie une liste plate de tous les champs de tous les groupes.
func (fc *FicheConfig) GetAllFields() []FieldDef {
	var allFields []FieldDef
	for _, group := range fc.Groups {
		allFields = append(allFields, group.Fields...)
	}
	return allFields
}

// ListConfig configuration pour la liste
type ListConfig struct {
	Name                       string            `yaml:"name"`
	Title                      string            `yaml:"title,omitempty"` // NOUVEAU
	PageSize                   int               `yaml:"pageSize"`
	DefaultSortField           string            `yaml:"defaultSortField"`
	DefaultSortOrder           string            `yaml:"defaultSortOrder"`
	PageSizeOptions            []int             `yaml:"pageSizeOptions"`
	Columns                    []string          `yaml:"columns"`
	SearchableFields           []string          `yaml:"searchableFields"`
	SortableFields             []string          `yaml:"sortableFields"`
	Width                      string            `yaml:"width,omitempty"`
	MaxWidth                   string            `yaml:"maxWidth,omitempty"`
	FormBackgroundColor        string            `yaml:"formBackgroundColor,omitempty"`
	PageBackgroundColor        string            `yaml:"pageBackgroundColor,omitempty"`
	ButtonFontSize             string            `yaml:"buttonFontSize,omitempty"`
	PaginationButtonFontSize   string            `yaml:"paginationButtonFontSize,omitempty"`
	PaginationTextFontSize     string            `yaml:"paginationTextFontSize,omitempty"`
	ColumnWidths               []string          `yaml:"columnWidths,omitempty"`
	ColumnHeaderFontSize       string            `yaml:"columnHeaderFontSize,omitempty"`
	ColumnAlignments           []string          `yaml:"columnAlignments,omitempty"`
}

// Field décrit un champ d'entité (modèle de données)
type Field struct {
	Name          string
	Label         string
	Type          string
	ReadOnly      bool
	Required      bool
	Default       interface{}
	DisplayFormat string
	MaxLength     int
}

// EntityConfig regroupe tout le config d’une entité
type EntityConfig struct {
	Name              string
	Table             string
	Label             string
	LabelPlural       string
	DefaultPageSize   int
	Fields            []Field
	FieldsByName      map[string]Field
	FicheFieldsByName map[string]FieldDef // Map pour un accès rapide aux champs de la fiche
	List              ListConfig
	Fiche             FicheConfig
	VisionForms       map[string]VisionFormConfig
	Code              *form_codes.FormCode
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
		Name          string      `yaml:"name"`
		Type          string      `yaml:"type,omitempty"`
		Label         string      `yaml:"label"`
		ReadOnly      bool        `yaml:"readonly,omitempty"`
		Required      bool        `yaml:"required,omitempty"`
		Default       interface{} `yaml:"default,omitempty"`
		DisplayFormat string      `yaml:"displayFormat,omitempty"`
		MaxLength     int         `yaml:"maxLength,omitempty"`
	} `yaml:"fields"`
	Forms []struct {
		Name   string    `yaml:"name"`
		Type   string    `yaml:"type"`
		Config yaml.Node `yaml:"config"` // Utiliser yaml.Node pour un décodage flexible
	} `yaml:"forms"`
}

// mergeWithDefaults fusionne une configuration spécifique avec une configuration par défaut.
func mergeWithDefaults(specific, defaults interface{}) {
	specVal := reflect.ValueOf(specific).Elem()
	defVal := reflect.ValueOf(defaults)

	for i := 0; i < specVal.NumField(); i++ {
		specField := specVal.Field(i)
		defField := defVal.Field(i)

		// Gestion spéciale pour les maps
		if specField.Kind() == reflect.Map {
			if specField.IsNil() {
				specField.Set(defField)
			} else {
				for _, key := range defField.MapKeys() {
					// Si la clé du défaut n'existe pas dans le spécifique, on l'ajoute.
					if specField.MapIndex(key).IsZero() {
						specField.SetMapIndex(key, defField.MapIndex(key))
					}
				}
			}
			continue // Passer au champ suivant
		}

		// Comportement standard pour les autres champs
		if specField.IsZero() {
			specField.Set(defField)
		}
	}
}

// LoadEntityConfig lit et analyse la configuration d'une entité.
func LoadEntityConfig(path string) (*EntityConfig, error) {
	var y yamlEntity
	if err := loader.Load(path, &y); err != nil {
		return nil, fmt.Errorf("échec chargement %s : %w", path, err)
	}

	ec := &EntityConfig{
		Name:              y.Entity.Name,
		Table:             y.Entity.Table,
		Label:             y.Entity.Label,
		LabelPlural:       y.Entity.LabelPlural,
		DefaultPageSize:   y.Entity.DefaultPageSize,
		Fields:            make([]Field, len(y.Fields)),
		FieldsByName:      make(map[string]Field),
		FicheFieldsByName: make(map[string]FieldDef), // Initialisation
		VisionForms:       make(map[string]VisionFormConfig),
	}

	for i, f := range y.Fields {
		field := Field{f.Name, f.Label, f.Type, f.ReadOnly, f.Required, f.Default, f.DisplayFormat, f.MaxLength}
		ec.Fields[i] = field
		ec.FieldsByName[f.Name] = field
	}

	for _, form := range y.Forms {
		switch form.Type {
		case "list":
			var listCfg ListConfig
			if err := form.Config.Decode(&listCfg); err != nil {
				return nil, fmt.Errorf("erreur décodage 'list' form %s: %w", form.Name, err)
			}
			listCfg.Name = form.Name
			// Fusionner avec les valeurs par défaut
			mergeWithDefaults(&listCfg, defaultListConfig)
			ec.List = listCfg
		case "fiche":
			var ficheCfg FicheConfig
			if err := form.Config.Decode(&ficheCfg); err != nil {
				return nil, fmt.Errorf("erreur décodage 'fiche' form %s: %w", form.Name, err)
			}
			ficheCfg.Name = form.Name
			// Fusionner avec les valeurs par défaut
			mergeWithDefaults(&ficheCfg, defaultFicheConfig)
			ec.Fiche = ficheCfg
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

	// Remplir la map FicheFieldsByName après avoir chargé la fiche
	for _, group := range ec.Fiche.Groups {
		for _, fieldDef := range group.Fields {
			ec.FicheFieldsByName[fieldDef.Name] = fieldDef
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
