// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"example.com/go-crud/config"
	"example.com/go-crud/internal/admin" // Import du nouveau package admin
	"example.com/go-crud/internal/crud"
	"example.com/go-crud/internal/entity"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 1) Charger la config en appelant la NOUVELLE fonction Load
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("Erreur lors du chargement de la configuration : %v", err)
	}

	// 2) Ouvrir la DB (utilise la variable `cfg` locale)
	dbPath := filepath.Join(cfg.Database.Directory, cfg.Database.Name)
	log.Printf("Connecting SQLite at %s", dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Impossible d'ouvrir SQLite : %v", err)
	}

	// 3) Configurer le router (on passe `cfg` en paramètre)
	router := setupRouter(cfg)

	// --- ROUTES ADMIN ---
	// On déclare le groupe admin UNE SEULE FOIS
	adminRoutes := router.Group("/admin")
	{
		// On applique le middleware de sécurité à tout le groupe /admin
		adminRoutes.Use(admin.AuthMiddleware())

		// On définit ensuite les routes du groupe
		adminRoutes.GET("/settings", admin.GetSettingsHandler(cfg))
		adminRoutes.POST("/settings", admin.PostSettingsHandler())
	}

	// 4) Redirection racine vers la première entité
	files, _ := filepath.Glob("config/entities/*.yaml")
	if len(files) > 0 {
		if ec0, err := entity.LoadEntityConfig(files[0]); err != nil {
			log.Printf("Attention : impossible de créer la redirection racine car le fichier %s n'a pas pu être chargé : %v", files[0], err)
		} else {
			router.GET("/", func(c *gin.Context) {
				c.Redirect(http.StatusSeeOther, "/"+ec0.List.Name+"?"+c.Request.URL.RawQuery)
			})
		}
	}

	// 5) Enregistrer chaque entité CRUD
	for _, file := range files {
		log.Printf("load entity: %s", file)
		ec, err := entity.LoadEntityConfig(file)
		if err != nil {
			log.Printf("skip entity %s: %v", file, err)
			continue
		}
		crud.RegisterEntity(router, db, ec)
	}

	// 6) Démarrer le serveur avec graceful shutdown
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("Server running at %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped cleanly.")
}

// setupRouter prend maintenant un pointeur vers la config et est complète.
func setupRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 1) Définir les fonctions de template AVANT de charger les HTML
	r.SetFuncMap(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"marshal": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				log.Printf("Erreur de 'marshal' dans le template : %v", err)
				return template.JS("null")
			}
			return template.JS(b)
		},
	})

	// 2) Charger les templates (y compris le nouveau)
	r.LoadHTMLGlob("templates/*.html")

	// 3) Servir les assets statiques
	r.Static("/assets", "./assets")

	// 4) Configurer les proxies de confiance en utilisant la config passée en paramètre
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Fatalf("Erreur config proxies de confiance : %v", err)
	}

	return r
}