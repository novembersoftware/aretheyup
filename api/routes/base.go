package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/services"
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

func getIndex(c *gin.Context) {
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

	c.HTML(200, "index.html", gin.H{
		"Services": response,
	})
}
