package routes

import "github.com/gin-gonic/gin"

func SetupPageRoutes(r *gin.Engine) {
	r.GET("/", getIndex)
	r.GET("/:slug", getService)
}

func SetupAPIRoutes(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/services", getServices)
}
