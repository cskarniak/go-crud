// internal/crud/routes.go
package crud

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/go-crud/internal/entity"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// crudHandler détient les dépendances (DB, config d'entité) pour nos handlers.
type crudHandler struct {
	db *gorm.DB
	ec *entity.EntityConfig
}

// RegisterEntity configure les routes CRUD pour une entité en utilisant le crudHandler.
func RegisterEntity(r *gin.Engine, db *gorm.DB, ec *entity.EntityConfig) {
	// On crée une instance de notre handler qui contient la DB et la config.
	h := &crudHandler{db: db, ec: ec}

	listPrefix := "/" + ec.List.Name
	entityPrefix := "/" + ec.Name
	fichePrefix := "/" + ec.Fiche.Name

	// On attache les méthodes du handler aux routes.
	r.GET(listPrefix, h.list)
	r.GET(entityPrefix, h.redirectToList) // Alias /<entity> -> /<entity>_list

	r.GET(fichePrefix+"/new", h.newForm)
	r.POST(fichePrefix, h.create)

	r.GET(fichePrefix+"/edit/:id", h.editForm)
	r.POST(fichePrefix+"/update/:id", h.update)

	r.POST(fichePrefix+"/delete/:id", h.delete)

	r.GET(fichePrefix+"/vision-data/:field", h.visionData)
}

// --- Handlers (Méthodes attachées à crudHandler) ---

// list gère l'affichage de la liste paginée, triée et filtrée.
func (h *crudHandler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.ec.List.PageSize
	if ps := c.Query("pageSize"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			pageSize = v
		}
	}

	sortField := c.DefaultQuery("sort", h.ec.List.DefaultSortField)
	sortOrder := c.DefaultQuery("order", h.ec.List.DefaultSortOrder)

	search := strings.TrimSpace(c.Query("search"))
	highlightID, _ := strconv.Atoi(c.Query("highlight"))

	query := h.db.Table(h.ec.Table).Select(h.ec.List.Columns)
	countQ := h.db.Table(h.ec.Table)

	if search != "" && len(h.ec.List.SearchableFields) > 0 {
		var conds []string
		var args []interface{}
		for _, f := range h.ec.List.SearchableFields {
			conds = append(conds, f+" LIKE ?")
			args = append(args, "%"+search+"%")
		}
		where := strings.Join(conds, " OR ")
		query = query.Where(where, args...)
		countQ = countQ.Where(where, args...)
	}

	var data []map[string]interface{}
	query.Order(sortField + " " + sortOrder).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&data)

	// Gestion du surlignage
	for _, row := range data {
		if idVal, ok := row["id"]; ok {
			if idInt, err := strconv.Atoi(fmt.Sprint(idVal)); err == nil && idInt == highlightID {
				row["_highlight"] = true
			}
		}
	}

	var total int64
	countQ.Count(&total)
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Entity":          h.ec,
		"Columns":         h.ec.List.Columns,
		"Data":            data,
		"Page":            page,
		"PageSize":        pageSize,
		"PageSizeOptions": h.ec.List.PageSizeOptions,
		"SortField":       sortField,
		"SortOrder":       sortOrder,
		"Search":          search,
		"Total":           total,
		"TotalPages":      totalPages,
	})
}

// newForm affiche le formulaire de création.
func (h *crudHandler) newForm(c *gin.Context) {
	dataRow := make(map[string]interface{})
	if h.ec.Code != nil {
		for field, spec := range h.ec.Code.Prepopulate {
			if spec.Type == "now" {
				dataRow[field] = time.Now().Format(spec.Format)
			}
		}
	}

	c.HTML(http.StatusOK, "form.html", gin.H{
		"Entity":    h.ec,
		"Code":      h.ec.Code,
		"Mode":      "new",
		"DataRow":   dataRow,
		"Errors":    map[string]string{},
		"ComboData": h.prepareComboData(),
		"Page":      c.Query("page"),
		"PageSize":  c.Query("pageSize"),
		"SortField": c.Query("sort"),
		"SortOrder": c.Query("order"),
	})
}

// create traite la soumission du formulaire de création.
func (h *crudHandler) create(c *gin.Context) {
	errors := h.validate(c)
	if len(errors) > 0 {
		h.repopulateFormOnError(c, "new", errors)
		return
	}

	vals := h.bindAndConvertForm(c)

	if err := h.db.Table(h.ec.Table).Create(vals).Error; err != nil {
		c.String(http.StatusBadRequest, "Erreur de création : %v", err)
		return
	}

	// Redirection vers la bonne page avec surlignage
	var newID int
	h.db.Table(h.ec.Table).Select("id").Order("id DESC").Limit(1).Scan(&newID)
	var countBefore int64
	h.db.Table(h.ec.Table).Where("id <= ?", newID).Count(&countBefore)
	page := int((countBefore + int64(h.ec.List.PageSize) - 1) / int64(h.ec.List.PageSize))

	redirectURL := fmt.Sprintf(
		"/%s?page=%d&pageSize=%d&sort=%s&order=%s&highlight=%d",
		h.ec.List.Name, page, h.ec.List.PageSize, h.ec.List.DefaultSortField, h.ec.List.DefaultSortOrder, newID,
	)
	c.Redirect(http.StatusSeeOther, redirectURL)
}

// editForm affiche le formulaire de modification avec les données pré-remplies.
func (h *crudHandler) editForm(c *gin.Context) {
	id := c.Param("id")
	dataRow := make(map[string]interface{})
	if err := h.db.Table(h.ec.Table).Where("id = ?", id).Take(&dataRow).Error; err != nil {
		c.String(http.StatusNotFound, "Enregistrement non trouvé : %v", err)
		return
	}

	// Formater les dates pour l'affichage dans le formulaire
	for _, f := range h.ec.Fields {
		if f.Type == "date" {
			if raw, ok := dataRow[f.Name]; ok && raw != nil {
				if t, ok := raw.(time.Time); ok {
					dataRow[f.Name] = t.Format("2006-01-02")
				} else if str, ok := raw.(string); ok {
					// GORM peut retourner des dates comme string
					if t, err := time.Parse(time.RFC3339, str); err == nil {
						dataRow[f.Name] = t.Format("2006-01-02")
					}
				}
			}
		}
	}

	c.HTML(http.StatusOK, "form.html", gin.H{
		"Entity":    h.ec,
		"Code":      h.ec.Code,
		"Mode":      "edit",
		"DataRow":   dataRow,
		"Errors":    map[string]string{},
		"ComboData": h.prepareComboData(),
		"Page":      c.Query("page"),
		"PageSize":  c.Query("pageSize"),
		"SortField": c.Query("sort"),
		"SortOrder": c.Query("order"),
	})
}

// update traite la soumission du formulaire de modification.
func (h *crudHandler) update(c *gin.Context) {
	id := c.Param("id")
	errors := h.validate(c)
	if len(errors) > 0 {
		h.repopulateFormOnError(c, "edit", errors)
		return
	}

	updates := h.bindAndConvertForm(c)

	if err := h.db.Table(h.ec.Table).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.String(http.StatusBadRequest, "Erreur de mise à jour : %v", err)
		return
	}

	redirectURL := fmt.Sprintf(
		"/%s?page=%s&pageSize=%s&sort=%s&order=%s&highlight=%s",
		h.ec.List.Name, c.Query("page"), c.Query("pageSize"), c.Query("sort"), c.Query("order"), id,
	)
	c.Redirect(http.StatusSeeOther, redirectURL)
}

// delete gère la suppression d'un enregistrement.
func (h *crudHandler) delete(c *gin.Context) {
	h.db.Table(h.ec.Table).Where("id = ?", c.Param("id")).Delete(nil)
	c.Redirect(http.StatusSeeOther, "/"+h.ec.List.Name)
}

// --- Fonctions utilitaires (helpers) privées ---

// validate exécute les règles de validation définies dans le _code.yaml.
func (h *crudHandler) validate(c *gin.Context) map[string]string {
	errors := make(map[string]string)
	if h.ec.Code == nil {
		return errors
	}

	for field, rule := range h.ec.Code.BackValidations {
		raw := c.PostForm(field)
		if rule.Required && raw == "" {
			errors[field] = rule.RequiredMessage
			continue // Si c'est requis et vide, pas la peine de vérifier le reste
		}
		if raw != "" { // On ne valide min/max que si le champ n'est pas vide
			if rule.Min > 0 && len(raw) < rule.Min {
				errors[field] = rule.MinMessage
			}
			if rule.Max > 0 && len(raw) > rule.Max {
				errors[field] = rule.MaxMessage
			}
		}
	}
	return errors
}

// bindAndConvertForm lit les données du formulaire POST, les convertit aux bons types
// et gère les valeurs vides pour les transformer en nil (corrige le bug).
func (h *crudHandler) bindAndConvertForm(c *gin.Context) map[string]interface{} {
	values := make(map[string]interface{})

	for _, grp := range h.ec.Fiche.Groups {
		for _, fd := range grp.Fields {
			// Ignorer les champs readonly et l'ID
			if fd.Name == "id" { continue }
			var isReadOnly bool
			for _, f := range h.ec.Fields {
				if f.Name == fd.Name && f.ReadOnly {
					isReadOnly = true
					break
				}
			}
			if isReadOnly { continue }

			raw := c.PostForm(fd.Name)
			var finalValue interface{}

			// Cas spécial pour les champs vides : on veut `nil` pour insérer `NULL`.
			if raw == "" {
				finalValue = nil
			} else {
				// On ne fait la conversion que si la valeur n'est pas vide.
				isSpecialType := fd.ComboConfig != nil || fd.VisionConfig != nil
				if isSpecialType {
					finalValue = raw
				} else {
					for _, f := range h.ec.Fields {
						if f.Name == fd.Name {
							switch f.Type {
							case "uint", "int", "number":
								if i, err := strconv.Atoi(raw); err == nil {
									finalValue = i
								}
							case "date":
								if t, err := time.Parse("2006-01-02", raw); err == nil {
									finalValue = t
								}
							default:
								finalValue = raw
							}
							break
						}
					}
				}
			}
			values[fd.Name] = finalValue
		}
	}
	return values
}

// prepareComboData exécute les requêtes SQL pour tous les combo_base du formulaire.
func (h *crudHandler) prepareComboData() map[string][]map[string]interface{} {
	comboData := make(map[string][]map[string]interface{})
	for _, grp := range h.ec.Fiche.Groups {
		for _, fd := range grp.Fields {
			if fd.Type == "combo_base" && fd.ComboConfig != nil {
				var rows []map[string]interface{}
				h.db.Raw(fd.ComboConfig.SQL).Scan(&rows)
				opts := make([]map[string]interface{}, 0, len(rows))
				for _, row := range rows {
					var parts []string
					for _, col := range fd.ComboConfig.DisplayFields {
						parts = append(parts, fmt.Sprint(row[col]))
					}
					label := strings.Join(parts, fd.ComboConfig.Separator)
					opts = append(opts, map[string]interface{}{
						"Value": row[fd.ComboConfig.KeyField],
						"Label": label,
					})
				}
				comboData[fd.Name] = opts
			}
		}
	}
	return comboData
}

// repopulateFormOnError ré-affiche le formulaire en cas d'erreur de validation.
func (h *crudHandler) repopulateFormOnError(c *gin.Context, mode string, errors map[string]string) {
	dataRow := make(map[string]interface{})
	for _, grp := range h.ec.Fiche.Groups {
		for _, fd := range grp.Fields {
			dataRow[fd.Name] = c.PostForm(fd.Name)
		}
	}
	// Si on est en mode édition, il faut conserver l'ID.
	if mode == "edit" {
		dataRow["id"] = c.Param("id")
	}

	c.HTML(http.StatusBadRequest, "form.html", gin.H{
		"Entity":    h.ec,
		"Code":      h.ec.Code,
		"Mode":      mode,
		"DataRow":   dataRow,
		"Errors":    errors,
		"ComboData": h.prepareComboData(),
		"Page":      c.Query("page"),
		"PageSize":  c.Query("pageSize"),
		"SortField": c.Query("sort"),
		"SortOrder": c.Query("order"),
	})
}

// visionData fournit les données JSON pour les popups de type 'vision'.
func (h *crudHandler) visionData(c *gin.Context) {
	fieldName := c.Param("field")
	var vc *entity.VisionConfig
	for _, grp := range h.ec.Fiche.Groups {
		for _, fd := range grp.Fields {
			if fd.Name == fieldName && fd.VisionConfig != nil {
				vc = fd.VisionConfig
				break
			}
		}
		if vc != nil { break }
	}

	if vc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "configuration 'vision' non trouvée"})
		return
	}

	var rows []map[string]interface{}
	h.db.Raw(vc.SQL).Scan(&rows)
	c.JSON(http.StatusOK, rows)
}

// redirectToList gère l'alias de /<entity> vers la page de liste.
func (h *crudHandler) redirectToList(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/"+h.ec.List.Name+"?"+c.Request.URL.RawQuery)
}