// internal/crud/routes.go
package crud

import (
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "example.com/go-crud/internal/entity"
)

// RegisterEntity génère les routes CRUD pour une entité donnée,
// avec pagination, tri, recherche, combo_base, vision, pré‑remplissage,
// validations, highlight et affichage inline des erreurs.
func RegisterEntity(r *gin.Engine, db *gorm.DB, ec *entity.EntityConfig) {
    listPrefix   := "/" + ec.List.Name
    entityPrefix := "/" + ec.Name
    fichePrefix  := "/" + ec.Fiche.Name

    // --- LIST (GET) ---
    r.GET(listPrefix, func(c *gin.Context) {
        page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
        pageSize := ec.List.PageSize
        if ps := c.Query("pageSize"); ps != "" {
            if v, err := strconv.Atoi(ps); err == nil {
                pageSize = v
            }
        }

        sortField := c.Query("sort")
        if sortField == "" {
            sortField = ec.List.DefaultSortField
        }
        sortOrder := c.Query("order")
        if sortOrder == "" {
            sortOrder = ec.List.DefaultSortOrder
        }

        search := strings.TrimSpace(c.Query("search"))
        highlightID, _ := strconv.Atoi(c.Query("highlight"))

        query := db.Table(ec.Table)
        // On ne récupère que les colonnes spécifiées dans le YAML
        query = db.Table(ec.Table).
            Select(ec.List.Columns)
        countQ := db.Table(ec.Table)
        if search != "" && len(ec.List.SearchableFields) > 0 {
            var conds []string
            var args []interface{}
            for _, f := range ec.List.SearchableFields {
                conds = append(conds, f+" LIKE ?")
                args = append(args, "%"+search+"%")
            }
            where := strings.Join(conds, " OR ")
            query = query.Where(where, args...)
            countQ = countQ.Where(where, args...)
        }

        var data []map[string]interface{}
        query.Order(sortField + " " + sortOrder).
            Offset((page-1)*pageSize).
            Limit(pageSize).
            Find(&data)

        // highlight
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
            "Entity":          ec,
            "Columns":         ec.List.Columns,
            "Data":            data,
            "Page":            page,
            "PageSize":        pageSize,
            "PageSizeOptions": ec.List.PageSizeOptions,
            "SortField":       sortField,
            "SortOrder":       sortOrder,
            "Search":          search,
            "Total":           total,
            "TotalPages":      totalPages,
        })
    })

    // --- ALIAS /<entity> → liste ---
    r.GET(entityPrefix, func(c *gin.Context) {
        c.Redirect(http.StatusSeeOther, listPrefix+"?"+c.Request.URL.RawQuery)
    })

    // --- NEW (GET) avec pré‑remplissage, combo_base et vision ---
    r.GET(fichePrefix+"/new", func(c *gin.Context) {
        dataRow := map[string]interface{}{}
        if ec.Code != nil {
            for field, spec := range ec.Code.Prepopulate {
                if spec.Type == "now" {
                    dataRow[field] = time.Now().Format(spec.Format)
                }
            }
        }

        // préparer combo_base
        comboData := make(map[string][]map[string]interface{})
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                if fd.Type == "combo_base" && fd.ComboConfig != nil {
                    var rows []map[string]interface{}
                    db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                    opts := make([]map[string]interface{}, 0, len(rows))
                    for _, row := range rows {
                        parts := []string{}
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

        c.HTML(http.StatusOK, "form.html", gin.H{
            "Entity":    ec,
            "Code":      ec.Code,
            "Mode":      "new",
            "DataRow":   dataRow,
            "Errors":    map[string]string{},
            "ComboData": comboData,
            "Page":      c.Query("page"),
            "PageSize":  c.Query("pageSize"),
            "SortField": c.Query("sort"),
            "SortOrder": c.Query("order"),
        })
    })

    // --- VISION DATA (GET) ---
    r.GET(fichePrefix+"/vision-data/:field", func(c *gin.Context) {
        fieldName := c.Param("field")
        var vc *entity.VisionConfig
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                if fd.Name == fieldName && fd.VisionConfig != nil {
                    vc = fd.VisionConfig
                    break
                }
            }
            if vc != nil {
                break
            }
        }
        if vc == nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "vision field not found"})
            return
        }

        var rows []map[string]interface{}
        db.Raw(vc.SQL).Scan(&rows)
        c.JSON(http.StatusOK, rows)
    })

    // --- CREATE (POST) ---
    r.POST(fichePrefix, func(c *gin.Context) {
        // validation back
        errors := map[string]string{}
        if ec.Code != nil {
            for field, rule := range ec.Code.BackValidations {
                raw, exists := c.GetPostForm(field)
                if !exists {
                    continue
                }
                if rule.Required && raw == "" {
                    if rule.RequiredMessage != "" {
                        errors[field] = rule.RequiredMessage
                    } else {
                        errors[field] = fmt.Sprintf("%s est obligatoire", field)
                    }
                }
                if rule.Min > 0 && len(raw) < rule.Min {
                    if rule.MinMessage != "" {
                        errors[field] = rule.MinMessage
                    } else {
                        errors[field] = fmt.Sprintf("%s doit faire au moins %d caractères", field, rule.Min)
                    }
                }
                if rule.Max > 0 && len(raw) > rule.Max {
                    if rule.MaxMessage != "" {
                        errors[field] = rule.MaxMessage
                    } else {
                        errors[field] = fmt.Sprintf("%s ne doit pas dépasser %d caractères", field, rule.Max)
                    }
                }
            }
        }
        if len(errors) > 0 {
            // reconstruire DataRow
            dataRow := map[string]interface{}{}
            for _, grp := range ec.Fiche.Groups {
                for _, fd := range grp.Fields {
                    dataRow[fd.Name] = c.PostForm(fd.Name)
                }
            }
            // reconstruire comboData
            comboData := make(map[string][]map[string]interface{})
            for _, grp := range ec.Fiche.Groups {
                for _, fd := range grp.Fields {
                    if fd.Type == "combo_base" && fd.ComboConfig != nil {
                        var rows []map[string]interface{}
                        db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                        opts := []map[string]interface{}{}
                        for _, row := range rows {
                            parts := []string{}
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
            c.HTML(http.StatusBadRequest, "form.html", gin.H{
                "Entity":    ec,
                "Code":      ec.Code,
                "Mode":      "new",
                "DataRow":   dataRow,
                "Errors":    errors,
                "ComboData": comboData,
                "Page":      c.Query("page"),
                "PageSize":  c.Query("pageSize"),
                "SortField": c.Query("sort"),
                "SortOrder": c.Query("order"),
            })
            return
        }

        // préparation des valeurs
        vals := map[string]interface{}{}
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                // skip readonly
                var ro bool
                for _, f := range ec.Fields {
                    if f.Name == fd.Name && f.ReadOnly {
                        ro = true
                        break
                    }
                }
                if ro {
                    continue
                }

                raw := c.PostForm(fd.Name)
                var v interface{}

                if fd.ComboConfig != nil || fd.VisionConfig != nil {
                    v = raw
                } else {
                    for _, f := range ec.Fields {
                        if f.Name == fd.Name {
                            switch f.Type {
                            case "uint", "int", "number":
                                if i, err := strconv.Atoi(raw); err == nil {
                                    v = i
                                }
                            case "date":
                                if t, err := time.Parse("2006-01-02", raw); err == nil {
                                    v = t
                                }
                            default:
                                v = raw
                            }
                            break
                        }
                    }
                }
                vals[fd.Name] = v
            }
        }

        if err := db.Table(ec.Table).Create(vals).Error; err != nil {
            c.String(http.StatusBadRequest, "Erreur de création : %v", err)
            return
        }

        // highlight du nouvel id
        var newID int
        db.Table(ec.Table).Select("id").Order("id DESC").Limit(1).Scan(&newID)
        var countBefore int64
        db.Table(ec.Table).Where("id <= ?", newID).Count(&countBefore)
        page := int((countBefore + int64(ec.List.PageSize) - 1) / int64(ec.List.PageSize))

        redirectURL := fmt.Sprintf(
            "%s?page=%d&pageSize=%d&sort=%s&order=%s&highlight=%d",
            listPrefix, page, ec.List.PageSize, ec.List.DefaultSortField, ec.List.DefaultSortOrder, newID,
        )
        c.Redirect(http.StatusSeeOther, redirectURL)
    })

    // --- EDIT (GET) avec combo_base & vision préchargés ---
    r.GET(fichePrefix+"/edit/:id", func(c *gin.Context) {
        id := c.Param("id")
        dataRow := map[string]interface{}{}
        if err := db.Table(ec.Table).Where("id = ?", id).Take(&dataRow).Error; err != nil {
            c.String(http.StatusNotFound, "Enregistrement non trouvé")
            return
        }
        // formater les dates
        for _, f := range ec.Fields {
            if f.Type == "date" {
                if raw, ok := dataRow[f.Name]; ok && raw != nil {
                    switch x := raw.(type) {
                    case time.Time:
                        dataRow[f.Name] = x.Format("2006-01-02")
                    case []byte:
                        dataRow[f.Name] = string(x)
                    }
                }
            }
        }

        // combo_base reload
        comboData := make(map[string][]map[string]interface{})
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                if fd.Type == "combo_base" && fd.ComboConfig != nil {
                    var rows []map[string]interface{}
                    db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                    opts := []map[string]interface{}{}
                    for _, row := range rows {
                        parts := []string{}
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

        c.HTML(http.StatusOK, "form.html", gin.H{
            "Entity":    ec,
            "Code":      ec.Code,
            "Mode":      "edit",
            "DataRow":   dataRow,
            "Errors":    map[string]string{},
            "ComboData": comboData,
            "Page":      c.Query("page"),
            "PageSize":  c.Query("pageSize"),
            "SortField": c.Query("sort"),
            "SortOrder": c.Query("order"),
        })
    })

    // --- UPDATE (POST) ---
    r.POST(fichePrefix+"/update/:id", func(c *gin.Context) {
        id := c.Param("id")
        // validation back
        errors := map[string]string{}
        if ec.Code != nil {
            for field, rule := range ec.Code.BackValidations {
                raw, exists := c.GetPostForm(field)
                if !exists {
                    continue
                }
                if rule.Required && raw == "" {
                    if rule.RequiredMessage != "" {
                        errors[field] = rule.RequiredMessage
                    } else {
                        errors[field] = fmt.Sprintf("%s est obligatoire", field)
                    }
                }
                if rule.Min > 0 && len(raw) < rule.Min {
                    if rule.MinMessage != "" {
                        errors[field] = rule.MinMessage
                    } else {
                        errors[field] = fmt.Sprintf("%s doit faire au moins %d caractères", field, rule.Min)
                    }
                }
                if rule.Max > 0 && len(raw) > rule.Max {
                    if rule.MaxMessage != "" {
                        errors[field] = rule.MaxMessage
                    } else {
                        errors[field] = fmt.Sprintf("%s ne doit pas dépasser %d caractères", field, rule.Max)
                    }
                }
            }
        }
        if len(errors) > 0 {
            dataRow := map[string]interface{}{"id": id}
            for _, grp := range ec.Fiche.Groups {
                for _, fd := range grp.Fields {
                    dataRow[fd.Name] = c.PostForm(fd.Name)
                }
            }
            // rebuild comboData as above…
            comboData := make(map[string][]map[string]interface{})
            for _, grp := range ec.Fiche.Groups {
                for _, fd := range grp.Fields {
                    if fd.Type == "combo_base" && fd.ComboConfig != nil {
                        var rows []map[string]interface{}
                        db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                        opts := []map[string]interface{}{}
                        for _, row := range rows {
                            parts := []string{}
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
            c.HTML(http.StatusBadRequest, "form.html", gin.H{
                "Entity":    ec,
                "Code":      ec.Code,
                "Mode":      "edit",
                "DataRow":   dataRow,
                "Errors":    errors,
                "ComboData": comboData,
                "Page":      c.Query("page"),
                "PageSize":  c.Query("pageSize"),
                "SortField": c.Query("sort"),
                "SortOrder": c.Query("order"),
            })
            return
        }

        // préparer updates
        updates := map[string]interface{}{}
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                if fd.Name == "id" {
                    continue
                }
                var ro bool
                for _, f := range ec.Fields {
                    if f.Name == fd.Name && f.ReadOnly {
                        ro = true
                        break
                    }
                }
                if ro {
                    continue
                }

                raw := c.PostForm(fd.Name)
                var v interface{}
                if fd.ComboConfig != nil || fd.VisionConfig != nil {
                    v = raw
                } else {
                    for _, f := range ec.Fields {
                        if f.Name == fd.Name {
                            switch f.Type {
                            case "uint", "int", "number":
                                if i, err := strconv.Atoi(raw); err == nil {
                                    v = i
                                }
                            case "date":
                                if t, err := time.Parse("2006-01-02", raw); err == nil {
                                    v = t
                                }
                            default:
                                v = raw
                            }
                            break
                        }
                    }
                }
                updates[fd.Name] = v
            }
        }

        if err := db.Table(ec.Table).Where("id = ?", id).Updates(updates).Error; err != nil {
            c.String(http.StatusBadRequest, "Erreur de mise à jour : %v", err)
            return
        }

        redirectURL := fmt.Sprintf(
            "%s?page=%s&pageSize=%s&sort=%s&order=%s&highlight=%s",
            listPrefix,
            c.Query("page"),
            c.Query("pageSize"),
            c.Query("sort"),
            c.Query("order"),
            id,
        )
        c.Redirect(http.StatusSeeOther, redirectURL)
    })

    // --- DELETE (POST) ---
    r.POST(fichePrefix+"/delete/:id", func(c *gin.Context) {
        db.Table(ec.Table).Where("id = ?", c.Param("id")).Delete(nil)
        c.Redirect(http.StatusSeeOther, listPrefix)
    })
}
