package routes

import (
	"github.com/gin-gonic/gin"
)

// GET /
func getIndexPage(c *gin.Context) {
	c.HTML(200, "index.html", gin.H{})
}

// GET /:slug
func getServicePage(c *gin.Context) {
	slug := c.Param("slug")
	c.HTML(200, "service.html", gin.H{
		"serviceSlug": slug,
	})
}
