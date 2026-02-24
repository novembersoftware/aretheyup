package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/storage"
)

func SetupPageRoutes(r *gin.Engine, store *storage.Storage, pageOriginGuard gin.HandlerFunc) {
	group := r.Group("")
	group.Use(pageOriginGuard)
	group.GET("/", getIndexPage)
	group.GET("/robots.txt", getRobotsTxt)
	group.GET("/sitemap.xml", func(c *gin.Context) { getSitemapXML(c, store) })
	group.GET("/:slug", func(c *gin.Context) { getServicePage(c, store) })
}

func SetupAPIRoutes(r *gin.Engine, store *storage.Storage, reportRouteLimiter, websiteAPIGuard gin.HandlerFunc) {
	g := r.Group("/api")
	g.Use(websiteAPIGuard)
	g.Use(func(c *gin.Context) {
		c.Header("X-Robots-Tag", "noindex, nofollow")
		c.Next()
	})
	g.GET("/services", func(c *gin.Context) { getServices(c, store) })
	g.GET("/services/search", func(c *gin.Context) { searchServices(c, store) })
	g.GET("/service/:slug", func(c *gin.Context) { getService(c, store) })
	g.POST("/service/:slug/report", reportRouteLimiter, func(c *gin.Context) { createServiceReport(c, store) })
	g.GET("/services/count", func(c *gin.Context) { getServiceCount(c, store) })
}
