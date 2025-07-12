package crud

import (
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "example.com/go-crud/internal/entity"
)

// RegisterEntity génère les routes CRUD pour une entité donnée.
func RegisterEntity(r *gin.Engine, db *gorm.DB, ec *entity.EntityConfig) {
    listPrefix   := "/" + ec.List.Name    // ex: /categorieList
    entityPrefix := "/" + ec.Name         // ex: /categorie
    fichePrefix  := "/" + ec.Fiche.Name   // ex: /categorieFiche

    // --- 1) LIST via /<formListName> ---
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
        if sortField == "" && len(ec.Fields) > 0 {
            sortField = ec.Fields[0].Name
        }
        sortOrder := c.Query("order")
        if sortOrder == "" {
            sortOrder = ec.List.DefaultSortOrder
        }
        if sortOrder == "" {
            sortOrder = "asc"
        }

        var data []map[string]interface{}
        db.Table(ec.Table).
            Order(sortField + " " + sortOrder).
            Offset((page-1)*pageSize).
            Limit(pageSize).
            Find(&data)

        var total int64
        db.Table(ec.Table).Count(&total)
        totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

        c.HTML(http.StatusOK, "index.html", gin.H{
            "Entity":     ec,
            "Columns":    ec.List.Columns,
            "Data":       data,
            "Page":       page,
            "PageSize":   pageSize,
            "SortField":  sortField,
            "SortOrder":  sortOrder,
            "Total":      total,
            "TotalPages": totalPages,
        })
    })

    // --- 2) Alias : /<entityName> redirige vers la liste ---
    r.GET(entityPrefix, func(c *gin.Context) {
        c.Redirect(http.StatusSeeOther, listPrefix+"?"+c.Request.URL.RawQuery)
    })

    // --- 3) NEW ---
    r.GET(fichePrefix+"/new", func(c *gin.Context) {
        c.HTML(http.StatusOK, "form.html", gin.H{
            "Entity":    ec,
            "Mode":      "new",
            "DataRow":   map[string]interface{}{},
            "Page":      c.Query("page"),
            "PageSize":  c.Query("pageSize"),
            "SortField": c.Query("sort"),
            "SortOrder": c.Query("order"),
        })
    })

    // --- 4) CREATE ---
    r.POST(fichePrefix, func(c *gin.Context) {
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
        c.Redirect(http.StatusSeeOther, listPrefix)
    })

    // --- 5) EDIT ---
    r.GET(fichePrefix+"/edit/:id", func(c *gin.Context) {
        id := c.Param("id")
        var dataRow map[string]interface{}
        if err := db.Table(ec.Table).
            Where("id = ?", id).
            Take(&dataRow).Error; err != nil {
            c.String(http.StatusNotFound, "Enregistrement non trouvé")
            return
        }

        // Formater les champs de type date pour préremplir le form
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

        c.HTML(http.StatusOK, "form.html", gin.H{
            "Entity":    ec,
            "Mode":      "edit",
            "DataRow":   dataRow,
            "Page":      c.Query("page"),
            "PageSize":  c.Query("pageSize"),
            "SortField": c.Query("sort"),
            "SortOrder": c.Query("order"),
        })
    })

    // --- 6) UPDATE ---
    r.POST(fichePrefix+"/update/:id", func(c *gin.Context) {
        id := c.Param("id")
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
        if err := db.Table(ec.Table).
            Where("id = ?", id).
            Updates(updates).Error; err != nil {
            c.String(http.StatusBadRequest, "Erreur de mise à jour : %v", err)
            return
        }

        // Conserver le contexte de pagination/tri
        page := c.Query("page")
        pageSize := c.Query("pageSize")
        sort := c.Query("sort")
        order := c.Query("order")
        redirectURL := listPrefix +
            "?page=" + page +
            "&pageSize=" + pageSize +
            "&sort=" + sort +
            "&order=" + order
        c.Redirect(http.StatusSeeOther, redirectURL)
    })

    // --- 7) DELETE ---
    r.POST(fichePrefix+"/delete/:id", func(c *gin.Context) {
        db.Table(ec.Table).Where("id = ?", c.Param("id")).Delete(nil)
        c.Redirect(http.StatusSeeOther, listPrefix)
    })
}
