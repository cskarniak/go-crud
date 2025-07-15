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
// avec pagination, tri, recherche, combo_base, pré‑remplissage, validations,
// highlight et affichage inline des erreurs.
func RegisterEntity(r *gin.Engine, db *gorm.DB, ec *entity.EntityConfig) {
    listPrefix := "/" + ec.List.Name
    entityPrefix := "/" + ec.Name
    fichePrefix := "/" + ec.Fiche.Name

    // --- LIST (GET) ---
    r.GET(listPrefix, func(c *gin.Context) {
        // Pagination
        page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
        pageSize := ec.List.PageSize
        if ps := c.Query("pageSize"); ps != "" {
            if v, err := strconv.Atoi(ps); err == nil {
                pageSize = v
            }
        }

        // Tri
        sortField := c.Query("sort")
        if sortField == "" {
            sortField = ec.List.DefaultSortField
        }
        sortOrder := c.Query("order")
        if sortOrder == "" {
            sortOrder = ec.List.DefaultSortOrder
        }

        // Recherche
        search := strings.TrimSpace(c.Query("search"))

        // Highlight
        highlightID, _ := strconv.Atoi(c.Query("highlight"))

        // Build queries
        query := db.Table(ec.Table)
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

        // Fetch data
        var data []map[string]interface{}
        query.Order(sortField+" "+sortOrder).
            Offset((page-1)*pageSize).
            Limit(pageSize).
            Find(&data)

        // Mark highlight
        for _, row := range data {
            if idVal, ok := row["id"]; ok {
                if idInt, err := strconv.Atoi(fmt.Sprint(idVal)); err == nil && idInt == highlightID {
                    row["_highlight"] = true
                }
            }
        }

        // Count total
        var total int64
        countQ.Count(&total)
        totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

        // Render
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

    // --- NEW (GET) avec pré‑remplissage et combo_base ---
    r.GET(fichePrefix+"/new", func(c *gin.Context) {
        // Pré-remplissage
        dataRow := map[string]interface{}{}
        if ec.Code != nil {
            for field, spec := range ec.Code.Prepopulate {
                if spec.Type == "now" {
                    dataRow[field] = time.Now().Format(spec.Format)
                }
            }
        }

        // Préparer comboData
        comboData := map[string][]map[string]interface{}{}
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                if fd.Type != "combo_base" || fd.ComboConfig == nil {
                    continue
                }
                var rows []map[string]interface{}
                if err := db.Raw(fd.ComboConfig.SQL).Scan(&rows).Error; err != nil {
                    continue
                }
                opts := make([]map[string]interface{}, 0, len(rows))
                for _, row := range rows {
                    parts := make([]string, 0, len(fd.ComboConfig.DisplayFields))
                    for _, col := range fd.ComboConfig.DisplayFields {
                        if v, ok := row[col]; ok {
                            parts = append(parts, fmt.Sprint(v))
                        }
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

        c.HTML(http.StatusOK, "form.html", gin.H{
            "Entity":    ec,
            "Code":      ec.Code,
            "Mode":      "new",
            "DataRow":   dataRow,
            "Errors":    map[string]string{},  // toujours non-nil
            "ComboData": comboData,
            "Page":      c.Query("page"),
            "PageSize":  c.Query("pageSize"),
            "SortField": c.Query("sort"),
            "SortOrder": c.Query("order"),
        })
    })

    // --- CREATE (POST) avec validation et erreurs inline ---
    r.POST(fichePrefix, func(c *gin.Context) {
        // Validation back
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
            // Reconstituer dataRow et comboData
            dataRow := map[string]interface{}{}
            for _, f := range ec.Fields {
                dataRow[f.Name] = c.PostForm(f.Name)
            }
            comboData := map[string][]map[string]interface{}{}
            for _, grp := range ec.Fiche.Groups {
                for _, fd := range grp.Fields {
                    if fd.Type != "combo_base" || fd.ComboConfig == nil {
                        continue
                    }
                    var rows []map[string]interface{}
                    db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                    opts := make([]map[string]interface{}, 0, len(rows))
                    for _, row := range rows {
                        parts := make([]string, 0, len(fd.ComboConfig.DisplayFields))
                        for _, col := range fd.ComboConfig.DisplayFields {
                            if v, ok := row[col]; ok {
                                parts = append(parts, fmt.Sprint(v))
                            }
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

        // Inserer en base
        vals := map[string]interface{}{}
        for _, f := range ec.Fields {
            if f.ReadOnly {
                continue
            }
            raw := c.PostForm(f.Name)
            var v interface{}
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
            vals[f.Name] = v
        }
        if err := db.Table(ec.Table).Create(vals).Error; err != nil {
            c.String(http.StatusBadRequest, "Erreur de création : %v", err)
            return
        }

        // Highlight nouvel enregistrement
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

    // --- EDIT (GET) avec combo_base ---
    r.GET(fichePrefix+"/edit/:id", func(c *gin.Context) {
        id := c.Param("id")
        var dataRow map[string]interface{}
        if err := db.Table(ec.Table).Where("id = ?", id).Take(&dataRow).Error; err != nil {
            c.String(http.StatusNotFound, "Enregistrement non trouvé")
            return
        }
        // format date
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
        // Préparer comboData
        comboData := map[string][]map[string]interface{}{}
        for _, grp := range ec.Fiche.Groups {
            for _, fd := range grp.Fields {
                if fd.Type != "combo_base" || fd.ComboConfig == nil {
                    continue
                }
                var rows []map[string]interface{}
                db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                opts := make([]map[string]interface{}, 0, len(rows))
                for _, row := range rows {
                    parts := make([]string, 0, len(fd.ComboConfig.DisplayFields))
                    for _, col := range fd.ComboConfig.DisplayFields {
                        if v, ok := row[col]; ok {
                            parts = append(parts, fmt.Sprint(v))
                        }
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

    // --- UPDATE (POST) avec validation et erreurs inline ---
    r.POST(fichePrefix+"/update/:id", func(c *gin.Context) {
        id := c.Param("id")
        // Validation back
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
            // Reconstituer dataRow et comboData puis réafficher
            dataRow := map[string]interface{}{"id": id}
            for _, f := range ec.Fields {
                dataRow[f.Name] = c.PostForm(f.Name)
            }
            comboData := map[string][]map[string]interface{}{}
            for _, grp := range ec.Fiche.Groups {
                for _, fd := range grp.Fields {
                    if fd.Type != "combo_base" || fd.ComboConfig == nil {
                        continue
                    }
                    var rows []map[string]interface{}
                    db.Raw(fd.ComboConfig.SQL).Scan(&rows)
                    opts := make([]map[string]interface{}, 0, len(rows))
                    for _, row := range rows {
                        parts := make([]string, 0, len(fd.ComboConfig.DisplayFields))
                        for _, col := range fd.ComboConfig.DisplayFields {
                            if v, ok := row[col]; ok {
                                parts = append(parts, fmt.Sprint(v))
                            }
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

        // Appliquer update
        updates := map[string]interface{}{}
        for _, f := range ec.Fields {
            if f.ReadOnly || f.Name == "id" {
                continue
            }
            raw := c.PostForm(f.Name)
            var v interface{}
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
            updates[f.Name] = v
        }
        if err := db.Table(ec.Table).Where("id = ?", id).Updates(updates).Error; err != nil {
            c.String(http.StatusBadRequest, "Erreur de mise à jour : %v", err)
            return
        }

        redirectURL := fmt.Sprintf("%s?page=%s&pageSize=%s&sort=%s&order=%s&highlight=%s",
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
