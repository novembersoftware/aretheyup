package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/lib"
	"github.com/novembersoftware/aretheyup/storage"
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
// Returns the top 48 services ordered by recent report count (last 10 minutes)
func getServices(c *gin.Context, store *storage.Storage) {
	rows, err := store.ListServices(c.Request.Context())
	if err != nil {
		lib.Respond(c, 500, "error", gin.H{"error": "Failed to fetch services"})
		return
	}

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
func searchServices(c *gin.Context, store *storage.Storage) {
	q := c.Query("q")

	if q == "" {
		getServices(c, store)
		return
	}

	rows, err := store.SearchServices(c.Request.Context(), q)
	if err != nil {
		lib.Respond(c, 500, "error", gin.H{"error": "Failed to search services"})
		return
	}

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
func getService(c *gin.Context, store *storage.Storage) {
	slug := c.Param("slug")

	service, err := store.GetServiceBySlug(c.Request.Context(), slug)
	if err != nil {
		lib.Respond(c, 404, "service-not-found", gin.H{
			"error": "Service not found",
		})
		return
	}

	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	count, _ := store.CountRecentReports(c.Request.Context(), service.ID, tenMinutesAgo)

	response := ServiceDetailResponse{
		ID:            service.ID,
		Slug:          service.Slug,
		Name:          service.Name,
		URL:           service.HomepageURL,
		Category:      service.Category,
		Status:        string(algorithm.StatusFromCount(count)),
		RecentReports: count,
	}

	lib.Respond(c, 200, "service-card", gin.H{
		"service": response,
	})
}
