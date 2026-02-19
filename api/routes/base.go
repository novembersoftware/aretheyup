package routes

import (
	"github.com/gin-gonic/gin"
)

// GET /
func getIndex(c *gin.Context) {
	c.HTML(200, "index.html", gin.H{})
}
