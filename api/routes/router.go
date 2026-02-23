package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/api/middleware"
	"github.com/novembersoftware/aretheyup/storage"
)

func SetupPageRoutes(r *gin.Engine, store *storage.Storage, pageOriginGuard, publicRouteLimiter gin.HandlerFunc) {
	group := r.Group("")
	group.Use(pageOriginGuard)
	group.GET("/", publicRouteLimiter, getIndexPage)
	group.GET("/robots.txt", publicRouteLimiter, getRobotsTxt)
	group.GET("/sitemap.xml", publicRouteLimiter, func(c *gin.Context) { getSitemapXML(c, store) })
	group.GET("/:slug", publicRouteLimiter, func(c *gin.Context) { getServicePage(c, store) })
}

func SetupAPIRoutes(r *gin.Engine, store *storage.Storage, publicRouteLimiter, reportRouteLimiter, websiteWriteGuard gin.HandlerFunc) {
	g := r.Group("/api")
	g.Use(middleware.OpenAPICORS())
	g.Use(func(c *gin.Context) {
		c.Header("X-Robots-Tag", "noindex, nofollow")
		c.Next()
	})
	g.GET("/services", publicRouteLimiter, func(c *gin.Context) { getServices(c, store) })
	g.GET("/services/search", publicRouteLimiter, func(c *gin.Context) { searchServices(c, store) })
	g.GET("/service/:slug", publicRouteLimiter, func(c *gin.Context) { getService(c, store) })
	g.POST("/service/:slug/report", websiteWriteGuard, reportRouteLimiter, func(c *gin.Context) { createServiceReport(c, store) })
	g.GET("/services/count", publicRouteLimiter, func(c *gin.Context) { getServiceCount(c, store) })
}
