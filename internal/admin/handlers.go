// internal/admin/handlers.go
package admin

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"example.com/go-crud/config"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// GetSettingsHandler affiche le formulaire avec les configurations actuelles.
func GetSettingsHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "admin_settings.html", gin.H{
			"Title":  "Configuration du Site",
			"Config": cfg,
		})
	}
}

// PostSettingsHandler traite la soumission du formulaire et met à jour la configuration.
func PostSettingsHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mise à jour de la struct de configuration en mémoire
		cfg.Server.Port = c.PostForm("port")
		cfg.Database.Directory = c.PostForm("db_directory")
		cfg.Database.Name = c.PostForm("db_name")
		if perPage, err := strconv.Atoi(c.PostForm("default_per_page")); err == nil {
			cfg.General.DefaultPerPage = perPage
		}

		// Conversion de la struct Go en YAML
		yamlData, err := yaml.Marshal(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, "Erreur : impossible de générer le YAML.")
			return
		}

		// Écriture du nouveau contenu dans le fichier config.yaml
		err = os.WriteFile("config/config.yaml", yamlData, 0644)
		if err != nil {
			c.String(http.StatusInternalServerError, "Erreur : impossible d'écrire dans le fichier de configuration.")
			return
		}

		log.Println("Fichier config.yaml mis à jour avec succès.")
		// Redirection pour que le navigateur recharge la page avec les nouvelles valeurs
		c.Redirect(http.StatusFound, "/admin/settings")
	}
}

// AuthMiddleware est un middleware simple pour protéger nos routes admin.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Le login/mot de passe est codé en dur pour cet exemple.
		user, pass, hasAuth := c.Request.BasicAuth()
		if hasAuth && user == "admin" && pass == "password" {
			c.Next()
		} else {
			c.Writer.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}
