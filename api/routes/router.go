package routes

import "github.com/gin-gonic/gin"

func SetupPublicRoutes(r *gin.Engine) {
	g := r.Group("/")

	g.GET("/", getIndex)
	g.GET("/:service", getService)
}
