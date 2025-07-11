package main

import (
    "context"
    "fmt"
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
    // Charger la configuration
    config.Load()

    // Initialiser la base de données
    db, err := setupDatabase()
    if err != nil {
        log.Fatalf("DB init error: %v", err)
    }

    // Configurer le routeur
    router := setupRouter()

    // Handler racine : redirection vers la première entité
    files, _ := filepath.Glob("config/entities/*.yaml")
    if len(files) > 0 {
        if ec0, err := entity.LoadEntityConfig(files[0]); err == nil && ec0.Name != "" {
            router.GET("/", func(c *gin.Context) {
                c.Redirect(http.StatusSeeOther, "/"+ec0.Name)
            })
        }
    }

    // Charger et enregistrer les routes CRUD
    for _, file := range files {
        ec, err := entity.LoadEntityConfig(file)
        if err != nil || ec.Name == "" {
            log.Printf("skip entity %s: %v", file, err)
            continue
        }
        crud.RegisterEntity(router, db, ec)
    }

    // Démarrer le serveur avec graceful shutdown
    startServer(router)
}

func setupDatabase() (*gorm.DB, error) {
    dbPath := config.Cfg.Database.Path
    log.Printf("Connecting SQLite at %s", dbPath)
    return gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
}

func setupRouter() *gin.Engine {
    gin.SetMode(gin.ReleaseMode)
    r := gin.Default()
    // Charger templates et assets
    r.LoadHTMLGlob("templates/*.html")
    r.Static("/assets", "./assets")

    // Configurer proxies de confiance
    if err := r.SetTrustedProxies(config.Cfg.Server.TrustedProxies); err != nil {
        log.Fatalf("trusted proxies error: %v", err)
    }
    return r
}

func startServer(r *gin.Engine) {
    addr := fmt.Sprintf(":%s", config.Cfg.Server.Port)
    srv := &http.Server{Addr: addr, Handler: r}

    go func() {
        log.Printf("Server running at %s", addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down server...")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("shutdown error: %v", err)
    }
    log.Println("Server stopped")
}
