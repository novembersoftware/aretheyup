package api

import (
	"html/template"
	"time"

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
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Logger)
	r.Use(getRateLimiter("global", store))

	templ := template.Must(template.New("").Funcs(lucide.FuncMap()).ParseGlob("templates/*.html"))
	templ = template.Must(templ.ParseGlob("templates/components/*.html"))
	r.SetHTMLTemplate(templ)

	r.Static("/static", "./static")

	routes.SetupPageRoutes(
		r,
		middleware.RequireAllowedPageOrigin(config.C.AllowedPageOrigins),
		getRateLimiter("public", store),
	)
	routes.SetupAPIRoutes(
		r,
		store,
		getRateLimiter("public", store),
		getRateLimiter("report", store),
		middleware.RequireWebsiteWriteOrigin(config.C.AllowedPageOrigins),
	)

	run(r)
}

func run(r *gin.Engine) {
	log.Info().Str("port", config.C.APIPort).Msg("Server started")
	err := r.Run(":" + config.C.APIPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}

func getRateLimiter(group string, store *storage.Storage) gin.HandlerFunc {
	switch group {
	case "global":
		return middleware.GlobalRateLimit(
			store.Redis(),
			config.C.GlobalRateLimitMaxRequests,
			time.Duration(config.C.GlobalRateLimitWindowSeconds)*time.Second,
		)
	case "public":
		return middleware.PublicRouteRateLimit(
			store.Redis(),
			config.C.PublicRateLimitMaxRequests,
			time.Duration(config.C.PublicRateLimitWindowSeconds)*time.Second,
		)
	case "report":
		return middleware.ReportRouteRateLimit(
			store.Redis(),
			config.C.ReportRateLimitMaxRequests,
			time.Duration(config.C.ReportRateLimitWindowSeconds)*time.Second,
		)
	default:
		panic("invalid rate limiter group: " + group)
	}
}
