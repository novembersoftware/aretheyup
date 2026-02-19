package routes

import "github.com/gin-gonic/gin"

func SetupPublicRoutes(r *gin.Engine) {
	r.GET("/", getIndex)
	r.GET("/:slug", getService)
}
