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
// avec prise en charge de la mise en évidence d’un enregistrement créé ou édité.
func RegisterEntity(r *gin.Engine, db *gorm.DB, ec *entity.EntityConfig) {
    listPrefix   := "/" + ec.List.Name    // ex: /categorieList
    entityPrefix := "/" + ec.Name         // ex: /categorie
    fichePrefix  := "/" + ec.Fiche.Name   // ex: /categorieFiche

    // --- LIST (GET) avec pagination, tri, recherche et highlight ---
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
        if sortField == "" && len(ec.List.SortableFields) > 0 {
            sortField = ec.List.SortableFields[0]
        }
        sortOrder := c.Query("order")
        if sortOrder == "" {
            sortOrder = ec.List.DefaultSortOrder
        }
        if sortOrder == "" {
            sortOrder = "asc"
        }

        // Recherche
        search := strings.TrimSpace(c.Query("search"))

        // Highlight ID (paramètre facultatif)
        highlightID, _ := strconv.Atoi(c.Query("highlight"))

        // Construction de la requête principale et du comptage
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

        // Récupération des données paginées
        var data []map[string]interface{}
        query.
            Order(sortField + " " + sortOrder).
            Offset((page-1)*pageSize).
            Limit(pageSize).
            Find(&data)

        // Marquer la ligne à surligner
        for _, row := range data {
            if idVal, ok := row["id"]; ok {
                idStr := fmt.Sprintf("%v", idVal)
                if idInt, err := strconv.Atoi(idStr); err == nil && idInt == highlightID {
                    row["_highlight"] = true
                }
            }
        }

        // Comptage total pour pagination
        var total int64
        countQ.Count(&total)
        totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

        // Rendu du template
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

    // --- Alias : /<entity> → liste ---
    r.GET(entityPrefix, func(c *gin.Context) {
        c.Redirect(http.StatusSeeOther, listPrefix+"?"+c.Request.URL.RawQuery)
    })

    // --- NEW (GET formulaire vide) ---
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

    // --- CREATE (POST) ---
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

        // Récupérer le nouvel ID
        var newID int
        db.Table(ec.Table).
            Select("id").
            Order("id DESC").
            Limit(1).
            Scan(&newID)

        // Calculer la page du nouvel enregistrement
        var countBefore int64
        db.Table(ec.Table).
            Where("id <= ?", newID).
            Count(&countBefore)
        page := int((countBefore + int64(ec.List.PageSize) - 1) / int64(ec.List.PageSize))

        // Rediriger vers la page avec highlight
        redirectURL := fmt.Sprintf(
            "%s?page=%d&pageSize=%d&sort=%s&order=%s&highlight=%d",
            listPrefix, page, ec.List.PageSize, ec.List.DefaultSortField, ec.List.DefaultSortOrder, newID,
        )
        c.Redirect(http.StatusSeeOther, redirectURL)
    })

    // --- EDIT (GET pré-remplit form) ---
    r.GET(fichePrefix+"/edit/:id", func(c *gin.Context) {
        id := c.Param("id")
        var dataRow map[string]interface{}
        if err := db.Table(ec.Table).
            Where("id = ?", id).
            Take(&dataRow).Error; err != nil {
            c.String(http.StatusNotFound, "Enregistrement non trouvé")
            return
        }
        // Formater les champs date
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

    // --- UPDATE (POST) ---
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

        // Redirection avec highlight
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
        db.Table(ec.Table).
            Where("id = ?", c.Param("id")).
            Delete(nil)
        c.Redirect(http.StatusSeeOther, listPrefix)
    })
}
