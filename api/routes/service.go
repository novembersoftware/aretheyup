package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/novembersoftware/aretheyup/structs"
)

type ServiceDetailResponse struct {
	ID            uint   `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	RecentReports int64  `json:"recent_reports"`
}

func getService(c *gin.Context) {
	slug := c.Param("slug")

	var service structs.Service
	if err := services.DB.Where("slug = ?", slug).First(&service).Error; err != nil {
		c.HTML(404, "service.html", gin.H{
			"Service": nil,
			"Error":   "Service not found",
		})
		return
	}

	status, recentReports := algorithm.GetServiceStatus(service.ID)
	response := ServiceDetailResponse{
		ID:            service.ID,
		Slug:          service.Slug,
		Name:          service.Name,
		URL:           service.HomepageURL,
		Category:      service.Category,
		Status:        string(status),
		RecentReports: recentReports,
	}

	responseType := c.GetHeader("Accept")
	if responseType == "application/json" {
		c.JSON(200, response)
		return
	}

	c.HTML(200, "service.html", gin.H{
		"PageTitle": response.Name,
		"Service":   response,
	})
}
