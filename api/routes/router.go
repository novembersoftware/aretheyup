package routes

import "github.com/gin-gonic/gin"

func SetupPageRoutes(r *gin.Engine) {
	r.GET("/", getIndexPage)
	r.GET("/:slug", getServicePage)
}

func SetupAPIRoutes(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/services", getServices)
	g.GET("/service/:slug", getService)
}
