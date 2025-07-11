package crud

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"example.com/go-crud/internal/entity"
)

// RegisterEntity génère les routes CRUD pour une entité donnée.
func RegisterEntity(r *gin.Engine, db *gorm.DB, ec *entity.EntityConfig) {
	prefix := "/" + ec.Name

	// LIST
	r.GET(prefix, func(c *gin.Context) {
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

		// Récupération des données
		var data []map[string]interface{}
		db.Table(ec.Table).
			Order(sortField + " " + sortOrder).
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&data)

		// Rendu
		c.HTML(http.StatusOK, "index.html", gin.H{
			"Entity":    ec,
			"Columns":   ec.List.Columns,
			"Data":      data,
			"Page":      page,
			"PageSize":  pageSize,
			"SortField": sortField,
			"SortOrder": sortOrder,
		})
	})

	// NEW
	r.GET(prefix+"/new", func(c *gin.Context) {
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

	// CREATE
	r.POST(prefix, func(c *gin.Context) {
		vals := make(map[string]interface{})
		for _, f := range ec.Fields {
			vals[f.Name] = c.PostForm(f.Name)
		}
		db.Table(ec.Table).Create(vals)
		c.Redirect(http.StatusSeeOther, prefix)
	})

	// EDIT
	r.GET(prefix+"/edit/:id", func(c *gin.Context) {
		id := c.Param("id")

		var dataRow map[string]interface{}
		if err := db.Table(ec.Table).
			Where("id = ?", id).
			Take(&dataRow).
			Error; err != nil {
			c.String(http.StatusNotFound, "Enregistrement non trouvé")
			return
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

	// UPDATE
	r.POST(prefix+"/update/:id", func(c *gin.Context) {
		id := c.Param("id")
		updates := make(map[string]interface{})
		for _, f := range ec.Fields {
			if f.Name == "id" {
				continue
			}
			updates[f.Name] = c.PostForm(f.Name)
		}
		db.Table(ec.Table).
			Where("id = ?", id).
			Updates(updates)

		// Redirection en conservant le contexte
		redirect := prefix + "?page=" + c.Query("page") +
			"&pageSize=" + c.Query("pageSize") +
			"&sort=" + c.Query("sort") +
			"&order=" + c.Query("order")
		c.Redirect(http.StatusSeeOther, redirect)
	})

	// DELETE
	r.POST(prefix+"/delete/:id", func(c *gin.Context) {
		db.Table(ec.Table).
			Where("id = ?", c.Param("id")).
			Delete(nil)
		c.Redirect(http.StatusSeeOther, prefix)
	})
}
