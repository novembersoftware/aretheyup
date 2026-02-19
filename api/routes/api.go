package routes

import (
	"time"

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
// Returns the top 50 services ordered by recent report count (last 10 minutes (for now)).
func getServices(c *gin.Context) {
	var rows []struct {
		ID                uint
		Slug              string
		Name              string
		HomepageURL       string
		Category          string
		RecentReportCount int64
	}

	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	services.DB.Raw(`
		SELECT s.id, s.slug, s.name, s.homepage_url, s.category,
		       COUNT(ur.id) AS recent_report_count
		FROM services s
		LEFT JOIN user_reports ur ON ur.service_id = s.id AND ur.timestamp > ?
		GROUP BY s.id
		ORDER BY recent_report_count DESC
		LIMIT 48
	`, tenMinutesAgo).Scan(&rows)

	response := make([]ServiceResponse, len(rows))
	for i, s := range rows {
		response[i] = ServiceResponse{
			ID:            s.ID,
			Slug:          s.Slug,
			Name:          s.Name,
			URL:           s.HomepageURL,
			Category:      s.Category,
			Status:        string(algorithm.StatusFromCount(s.RecentReportCount)),
			RecentReports: s.RecentReportCount,
		}
	}

	lib.Respond(c, 200, "service-list", gin.H{
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

// GET /api/services/search?q=...
func searchServices(c *gin.Context) {
	q := c.Query("q")

	if q == "" {
		getServices(c)
		return
	}

	var rows []struct {
		ID                uint
		Slug              string
		Name              string
		HomepageURL       string
		Category          string
		RecentReportCount int64
	}

	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	services.DB.Raw(`
		SELECT s.id, s.slug, s.name, s.homepage_url, s.category,
		       COUNT(ur.id) AS recent_report_count
		FROM services s
		LEFT JOIN user_reports ur ON ur.service_id = s.id AND ur.timestamp > ?
		WHERE LOWER(s.name) LIKE LOWER(?)
		GROUP BY s.id
		ORDER BY recent_report_count DESC
		LIMIT 48
	`, tenMinutesAgo, "%"+q+"%").Scan(&rows)

	response := make([]ServiceResponse, len(rows))
	for i, s := range rows {
		response[i] = ServiceResponse{
			ID:            s.ID,
			Slug:          s.Slug,
			Name:          s.Name,
			URL:           s.HomepageURL,
			Category:      s.Category,
			Status:        string(algorithm.StatusFromCount(s.RecentReportCount)),
			RecentReports: s.RecentReportCount,
		}
	}

	lib.Respond(c, 200, "service-list", gin.H{
		"services": response,
	})
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
