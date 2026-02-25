package api

import (
	"html/template"
	"strings"
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
	if err := configureTrustedProxies(r, config.C.TrustedProxies); err != nil {
		log.Fatal().Err(err).Msg("Invalid TRUSTED_PROXIES configuration")
	}

	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Logger)

	templ := template.Must(template.New("").Funcs(lucide.FuncMap()).ParseGlob("templates/*.html"))
	templ = template.Must(templ.ParseGlob("templates/components/*.html"))
	r.SetHTMLTemplate(templ)

	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.Static("/static", "./static")

	routes.SetupPageRoutes(
		r,
		store,
		middleware.RequireAllowedPageOrigin(config.C.AllowedPageOrigins),
	)
	routes.SetupAPIRoutes(
		r,
		store,
		middleware.ReportRouteRateLimit(
			store.Redis(),
			config.C.ReportRateLimitMaxRequests,
			time.Duration(config.C.ReportRateLimitWindowSeconds)*time.Second,
		),
		middleware.RequireWebsiteAPIOrigin(config.C.AllowedPageOrigins),
	)

	run(r)
}

func configureTrustedProxies(r *gin.Engine, trustedProxiesCSV string) error {
	trustedProxies := parseTrustedProxiesCSV(trustedProxiesCSV)
	if len(trustedProxies) == 0 {
		return r.SetTrustedProxies(nil)
	}

	return r.SetTrustedProxies(trustedProxies)
}

func parseTrustedProxiesCSV(raw string) []string {
	trustedProxies := make([]string, 0)
	for value := range strings.SplitSeq(raw, ",") {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}

		trustedProxies = append(trustedProxies, normalized)
	}

	return trustedProxies
}

func run(r *gin.Engine) {
	log.Info().Str("port", config.C.APIPort).Msg("Server started")
	err := r.Run(":" + config.C.APIPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
