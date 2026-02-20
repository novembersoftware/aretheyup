package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/storage"
)

func SetupPageRoutes(r *gin.Engine) {
	r.GET("/", getIndexPage)
	r.GET("/:slug", getServicePage)
}

func SetupAPIRoutes(r *gin.Engine, store *storage.Storage) {
	g := r.Group("/api")
	g.GET("/services", func(c *gin.Context) { getServices(c, store) })
	g.GET("/services/search", func(c *gin.Context) { searchServices(c, store) })
	g.GET("/service/:slug", func(c *gin.Context) { getService(c, store) })
}
