// internal/crud/routes.go
package crud

import (
	"database/sql"
	"fmt"
	"log"
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
	h := &crudHandler{db: db, ec: ec}

	// Routes standard (liste, fiche)
	r.GET("/"+ec.List.Name, h.list)
	r.GET("/"+ec.Name, h.redirectToList)
	r.GET("/"+ec.Fiche.Name+"/new", h.newForm)
	r.POST("/"+ec.Fiche.Name, h.create)
	r.GET("/"+ec.Fiche.Name+"/edit/:id", h.editForm)
	r.POST("/"+ec.Fiche.Name+"/update/:id", h.update)
	r.POST("/"+ec.Fiche.Name+"/delete/:id", h.delete)
	r.GET("/"+ec.Fiche.Name+"/vision-data/:field", h.visionData)

	// NOUVEAU : Enregistrer les routes pour les formulaires 'vision'
	for name := range ec.VisionForms {
		r.GET("/vision/"+name, h.vision)
	}
}

// --- Handlers ---

// vision est le NOUVEAU handler pour les formulaires de type 'vision'.
func (h *crudHandler) vision(c *gin.Context) {
	visionName := strings.TrimPrefix(c.FullPath(), "/vision/")
	visionCfg, ok := h.ec.VisionForms[visionName]
	if !ok {
		c.String(http.StatusNotFound, "Formulaire 'vision' non trouvé: %s", visionName)
		return
	}

	var args []interface{}
	for _, param := range visionCfg.Params {
		var value string
		if param.Source == "context" {
			// Le nom du champ est maintenant dynamique, basé sur ce que le bouton envoie.
			value = c.Query(param.ContextField)
		} else if param.Source == "literal" {
			value = param.Value
		}
		args = append(args, sql.Named(param.Name, value))
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := visionCfg.PageSize
	if ps, err := strconv.Atoi(c.Query("pageSize")); err == nil && ps > 0 {
		pageSize = ps
	}
	if pageSize == 0 {
		pageSize = 10
	}

	sortField := c.DefaultQuery("sort", visionCfg.DefaultSortField)
	sortOrder := c.DefaultQuery("order", visionCfg.DefaultSortOrder)

	var data []map[string]interface{}
	query := h.db.Raw(visionCfg.SQL, args...)
	err := query.Scan(&data).Error

	if err != nil {
		log.Printf("[VISION] Erreur SQL pour '%s': %v", visionName, err)
	} else {
		log.Printf("[VISION] Requête pour '%s' a retourné %d enregistrements", visionName, len(data))
	}

	// V29 - Logique pour le mode sélectionnable
	allowSelectable := true // Par défaut, la sélection est autorisée
	if visionCfg.Actions.AllowSelectable != nil {
		allowSelectable = *visionCfg.Actions.AllowSelectable
	}

	isVisionReturn := c.Query("return_to") != ""
	returnTo := c.Query("return_to")

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Entity":          h.ec,
		"VisionConfig":    visionCfg,
		"IsVisionReturn":  isVisionReturn,
		"ReturnTo":        returnTo,
		"AllowSelectable": allowSelectable, // On passe la valeur au template
		"Columns":         visionCfg.Columns,
		"Data":            data,
		"Page":            page,
		"PageSize":        pageSize,
		"PageSizeOptions": visionCfg.PageSizeOptions,
		"SortField":       sortField,
		"SortOrder":       sortOrder,
		"Search":          "",
		"Total":           len(data),
		"TotalPages":      1,
	})
}


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

	// Formater les nombres pour l'affichage
	for _, row := range data {
		for _, f := range h.ec.Fields {
			if f.Type == "number" {
				if ficheDef, ok := h.ec.FicheFieldsByName[f.Name]; ok && ficheDef.DecimalSeparator != "" {
					if raw, ok := row[f.Name]; ok && raw != nil {
						if num, err := strconv.ParseFloat(fmt.Sprint(raw), 64); err == nil {
							row[f.Name] = formatNumber(num, ficheDef.Decimals, ficheDef.DecimalSeparator, ficheDef.ThousandsSeparator)
						}
					}
				}
			}
		}
	}

	var total int64
	countQ.Count(&total)

	// Correction du bug de division par zéro (Force Write)
	var totalPages int
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	} else {
		totalPages = 0 // Évite le plantage si pageSize est 0
	}

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
	// On sélectionne toutes les colonnes
	if err := h.db.Table(h.ec.Table).Select("*").Where("id = ?", id).Take(&dataRow).Error; err != nil {
		c.String(http.StatusNotFound, "Enregistrement non trouvé : %v", err)
		return
	}

	// Formater les dates et les nombres pour l'affichage dans le formulaire
	const dbFormat = "2006-01-02 15:04:05" // Format de stockage

	for _, f := range h.ec.Fields {
		raw, ok := dataRow[f.Name]
		if !ok || raw == nil {
			continue
		}

		switch f.Type {
		case "datetime":
			if dateStr, ok := raw.(string); ok {
				if t, err := time.Parse(dbFormat, dateStr); err == nil {
					dataRow[f.Name] = t.Format(f.DisplayFormat)
				}
			}
		case "number":
			if ficheDef, ok := h.ec.FicheFieldsByName[f.Name]; ok && ficheDef.DecimalSeparator != "" {
				if num, err := strconv.ParseFloat(fmt.Sprint(raw), 64); err == nil {
					dataRow[f.Name] = formatNumber(num, ficheDef.Decimals, ficheDef.DecimalSeparator, ficheDef.ThousandsSeparator)
				}
			}
		case "boolean": // Nouvelle logique pour les booléens
			// GORM peut renvoyer des booléens comme int64 (0 ou 1) ou bool
			if val, ok := raw.(int64); ok {
				dataRow[f.Name] = val != 0 // Convertit 0 en false, tout le reste en true
			} else if val, ok := raw.(bool); ok {
				dataRow[f.Name] = val
			} else {
				// Si ce n'est ni int64 ni bool, on essaie de parser la chaîne si c'est une chaîne
				if s, ok := raw.(string); ok {
					dataRow[f.Name] = (s == "true" || s == "1" || s == "on")
				} else {
					// Par défaut, si on ne sait pas, on considère false
					dataRow[f.Name] = false
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
	
	// Créer une map pour un accès rapide aux propriétés des champs
	fieldProps := make(map[string]entity.Field)
	for _, f := range h.ec.Fields {
		fieldProps[f.Name] = f
	}

	for _, grp := range h.ec.Fiche.Groups {
		for _, fd := range grp.Fields {
			props, ok := fieldProps[fd.Name]
			// Ignorer les champs qui ne sont pas dans la config globale, l'ID, ou les champs readonly
			if !ok || props.Name == "id" || props.ReadOnly {
				continue
			}

			raw := c.PostForm(fd.Name)
			var finalValue interface{}

			if raw == "" {
				if props.Type == "boolean" {
					finalValue = false
				} else {
					finalValue = nil
				}
			} else {
				isSpecialType := fd.ComboConfig != nil || fd.VisionConfig != nil
				if isSpecialType {
					finalValue = raw
				} else {
					switch props.Type {
					case "uint", "int":
						if i, err := strconv.Atoi(raw); err == nil {
							finalValue = i
						}
					case "number":
						cleanRaw := strings.Replace(raw, ",", ".", -1)
						if f, err := strconv.ParseFloat(cleanRaw, 64); err == nil {
							finalValue = f
						}
					case "boolean":
						if raw == "on" || raw == "true" || raw == "1" {
							finalValue = true
						} else {
							finalValue = false
						}
					case "date":
						if raw == "" { // Si le champ date est vide
							finalValue = time.Now() // Remplir avec la date/heure actuelle
						} else if t, err := time.Parse("2006-01-02", raw); err == nil {
							finalValue = t
						}
					case "datetime":
						if raw == "" { // Si le champ datetime est vide
							finalValue = time.Now().Format("2006-01-02 15:04:05") // Remplir avec la date/heure actuelle formatée
						} else if props.DisplayFormat != "" {
							t, err := time.Parse(props.DisplayFormat, raw)
							if err == nil {
								finalValue = t.Format("2006-01-02 15:04:05")
							} else {
								finalValue = nil
							}
						} else {
							finalValue = raw
						}
					default:
						finalValue = raw
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
	var vc *entity.VisionFieldConfig
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

// formatNumber formate un nombre en chaîne de caractères avec les séparateurs et décimales spécifiés.
func formatNumber(n float64, decimals int, decSep, thouSep string) string {
	// Formate d'abord avec le bon nombre de décimales et un point comme séparateur.
	s := strconv.FormatFloat(n, 'f', decimals, 64)

	parts := strings.Split(s, ".")
	integerPart := parts[0]
	fractionalPart := ""
	if len(parts) > 1 {
		fractionalPart = parts[1]
	}

	// Ajoute le séparateur de milliers.
	n_len := len(integerPart)
	if n_len > 3 && thouSep != "" {
		// Calcule le nombre de séparateurs nécessaires.
		sep_count := (n_len - 1) / 3
		// Crée une nouvelle chaîne avec assez d'espace.
		new_integer_part := make([]byte, n_len+sep_count)
		// Remplit la nouvelle chaîne de droite à gauche.
		j := len(new_integer_part) - 1
		for i := n_len - 1; i >= 0; i-- {
			if (n_len-i-1)%3 == 0 && i != n_len-1 {
				new_integer_part[j] = thouSep[0]
				j--
			}
			new_integer_part[j] = integerPart[i]
			j--
		}
		integerPart = string(new_integer_part)
	}

	if fractionalPart != "" {
		return integerPart + decSep + fractionalPart
	}
	return integerPart
}
