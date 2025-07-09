package main

import (
    "log"
    "net/http"

    "example.com/go-crud/config"
    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

// Article est notre modèle de données
type Article struct {
    ID      uint   `gorm:"primaryKey"`
    Title   string `form:"title" binding:"required"`
    Content string `form:"content" binding:"required"`
}

func main() {
    // 1️⃣ Charge la config
    config.Init()

    // 2️⃣ En mode release (moins verbeux)
    gin.SetMode(gin.ReleaseMode)

    // 3️⃣ Crée l’engine Gin avec Logger et Recovery
    r := gin.New()
    r.Use(gin.Logger(), gin.Recovery())

    // 4️⃣ Sécurise les proxies
    if err := r.SetTrustedProxies(config.Conf.Server.TrustedProxies); err != nil {
        log.Fatalf("⚠️ Erreur proxies : %v", err)
    }

    // 5️⃣ Ouvre la base SQLite dont le chemin vient de la config
    db, err := gorm.Open(sqlite.Open(config.Conf.Database.Path), &gorm.Config{})
    if err != nil {
        panic("Échec de la connexion à la base de données")
    }
    db.AutoMigrate(&Article{})

    // 6️⃣ Statics & Templates
    r.Static("/assets", "./assets")
    r.LoadHTMLGlob("templates/*")

    // 7️⃣ Routes CRUD

    // – Liste des articles
    r.GET("/", func(c *gin.Context) {
        var articles []Article
        db.Find(&articles)
        c.HTML(http.StatusOK, "layout.html", gin.H{
            "articles": articles,
        })
    })

    // – Formulaire de création
    r.GET("/articles/new", func(c *gin.Context) {
        c.HTML(http.StatusOK, "layout.html", gin.H{
            "showCreate": true,
        })
    })
    r.POST("/articles", func(c *gin.Context) {
        var form Article
        if err := c.ShouldBind(&form); err != nil {
            c.HTML(http.StatusBadRequest, "layout.html", gin.H{
                "showCreate": true,
                "error":      err.Error(),
            })
            return
        }
        db.Create(&form)
        c.Redirect(http.StatusFound, "/")
    })

    // – Formulaire d’édition
    r.GET("/articles/edit/:id", func(c *gin.Context) {
        var article Article
        if err := db.First(&article, c.Param("id")).Error; err != nil {
            c.String(http.StatusNotFound, "Article non trouvé")
            return
        }
        c.HTML(http.StatusOK, "layout.html", gin.H{
            "showEdit": true,
            "article":  article,
        })
    })
    r.POST("/articles/update/:id", func(c *gin.Context) {
        var article Article
        if err := db.First(&article, c.Param("id")).Error; err != nil {
            c.String(http.StatusNotFound, "Article non trouvé")
            return
        }
        if err := c.ShouldBind(&article); err != nil {
            c.HTML(http.StatusBadRequest, "layout.html", gin.H{
                "showEdit": true,
                "error":    err.Error(),
                "article":  article,
            })
            return
        }
        db.Save(&article)
        c.Redirect(http.StatusFound, "/")
    })

    // – Suppression
    r.POST("/articles/delete/:id", func(c *gin.Context) {
        db.Delete(&Article{}, c.Param("id"))
        c.Redirect(http.StatusFound, "/")
    })

    // 8️⃣ Démarrage du serveur
    addr := ":" + config.Conf.Server.Port
    log.Printf("🚀 Serveur démarré sur http://localhost%s", addr)
    if err := r.Run(addr); err != nil {
        log.Fatalf("⚠️ Échec démarrage serveur : %v", err)
    }
}
