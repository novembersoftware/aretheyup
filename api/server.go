package api

import (
	"html/template"

	"github.com/gin-gonic/gin"
	"github.com/kaugesaar/lucide-go"
	"github.com/novembersoftware/aretheyup/api/middleware"
	"github.com/novembersoftware/aretheyup/api/routes"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

func Start(store *storage.Storage) {
	if config.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.Logger)

	templ := template.Must(template.New("").Funcs(lucide.FuncMap()).ParseGlob("templates/*.html"))
	templ = template.Must(templ.ParseGlob("templates/components/*.html"))
	r.SetHTMLTemplate(templ)

	r.Static("/static", "./static")

	routes.SetupPageRoutes(r)
	routes.SetupAPIRoutes(r, store)

	run(r)
}

func run(r *gin.Engine) {
	log.Info().Str("port", config.C.APIPort).Msg("Server started")
	err := r.Run(":" + config.C.APIPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
