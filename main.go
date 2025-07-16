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

    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "example.com/go-crud/config"
    "example.com/go-crud/internal/crud"
    "example.com/go-crud/internal/entity"
)

func main() {
    // 1) Charger la config
    config.Load()
    cfg := config.Cfg

    // 2) Ouvrir la DB
    dbPath := filepath.Join(cfg.General.DatabaseDir, cfg.General.DatabaseName)
    log.Printf("Connecting SQLite at %s", dbPath)
    db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
    if err != nil {
        log.Fatalf("Impossible d'ouvrir SQLite : %v", err)
    }

    // 3) Configurer le router
    router := setupRouter(&cfg)

    // 4) Redirection racine vers la première entité
    files, _ := filepath.Glob("config/entities/*.yaml")
    if len(files) > 0 {
        if ec0, err := entity.LoadEntityConfig(files[0]); err == nil {
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

func setupRouter(cfg *config.Config) *gin.Engine {
    // On utilise gin.New() pour maîtriser l'ordre
    gin.SetMode(gin.ReleaseMode)
    r := gin.New()
    // Middlewares usuels
    r.Use(gin.Logger(), gin.Recovery())

    // 1) Définir les fonctions de template AVANT de charger les HTML
    r.SetFuncMap(template.FuncMap{
        "add": func(a, b int) int { return a + b },
        "sub": func(a, b int) int { return a - b },
        "marshal": func(v interface{}) template.JS {
            b, _ := json.Marshal(v)
            return template.JS(b)
        },
    })

    // 2) Charger les templates (connait désormais add, sub, marshal)
    r.LoadHTMLGlob("templates/*.html")

    // 3) Servir les assets statiques
    r.Static("/assets", "./assets")

    // 4) Configurer les proxies de confiance
    if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
        log.Fatalf("Erreur config proxies de confiance : %v", err)
    }

    return r
}
