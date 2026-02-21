package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/storage"
)

func SetupPageRoutes(r *gin.Engine, publicRouteLimiter gin.HandlerFunc) {
	r.GET("/", publicRouteLimiter, getIndexPage)
	r.GET("/:slug", publicRouteLimiter, getServicePage)
}

func SetupAPIRoutes(r *gin.Engine, store *storage.Storage, publicRouteLimiter, reportRouteLimiter gin.HandlerFunc) {
	g := r.Group("/api")
	g.GET("/services", publicRouteLimiter, func(c *gin.Context) { getServices(c, store) })
	g.GET("/services/search", publicRouteLimiter, func(c *gin.Context) { searchServices(c, store) })
	g.GET("/service/:slug", publicRouteLimiter, func(c *gin.Context) { getService(c, store) })
	g.POST("/service/:slug/report", reportRouteLimiter, func(c *gin.Context) { createServiceReport(c, store) })
}
