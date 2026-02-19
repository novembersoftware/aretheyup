package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/lib"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/novembersoftware/aretheyup/structs"
)

type ServiceResponse struct {
	ID            uint   `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	RecentReports int64  `json:"recent_reports"`
}

// GET /api/services
func getServices(c *gin.Context) {
	const html = "service-list"

	var serviceList []struct {
		ID          uint
		Slug        string
		Name        string
		HomepageURL string
		Category    string
	}
	services.DB.Table("services").
		Select("services.id, services.slug, services.name, services.homepage_url, services.category").
		Find(&serviceList)

	response := make([]ServiceResponse, len(serviceList))
	for i, s := range serviceList {
		status, recentReports := algorithm.GetServiceStatus(s.ID)
		response[i] = ServiceResponse{
			ID:            s.ID,
			Slug:          s.Slug,
			Name:          s.Name,
			URL:           s.HomepageURL,
			Category:      s.Category,
			Status:        string(status),
			RecentReports: recentReports,
		}
	}

	lib.Respond(c, 200, html, gin.H{
		"services": response,
	})
}

type ServiceDetailResponse struct {
	ID            uint   `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	RecentReports int64  `json:"recent_reports"`
}

// GET /api/service/:slug
func getService(c *gin.Context) {
	slug := c.Param("slug")

	var service structs.Service
	if err := services.DB.Where("slug = ?", slug).First(&service).Error; err != nil {
		lib.Respond(c, 404, "service-not-found", gin.H{
			"error": "Service not found",
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

	lib.Respond(c, 200, "service-card", gin.H{
		"service": response,
	})
}
