package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// Respond will respond with HTML or JSON depending on the request
func Respond(c *gin.Context, status int, component string, data interface{}) {
	responseType := strings.ToLower(c.GetHeader("Accept"))
	if strings.Contains(responseType, "application/json") {
		c.JSON(status, data)
		return
	}

	c.HTML(status, component, data)
}
