package lib

import "github.com/gin-gonic/gin"

// Respond will respond with HTML or JSON depending on the request
func Respond(c *gin.Context, status int, component string, data interface{}) {
	responseType := c.GetHeader("Accept")
	if responseType == "application/json" {
		c.JSON(status, data)
		return
	}

	c.HTML(status, component, data)
}
