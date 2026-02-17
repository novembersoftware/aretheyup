package routes

import "github.com/gin-gonic/gin"

// GET /:service
func getService(c *gin.Context) {
	service := c.Param("service")

	// see if the request is a json
	responseType := c.GetHeader("Accept")
	if responseType == "application/json" {
		c.JSON(200, gin.H{
			"service": service,
		})
		return
	}

	c.HTML(200, "service.html", gin.H{
		"Service": service,
	})
}
