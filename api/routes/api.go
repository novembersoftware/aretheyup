package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/novembersoftware/aretheyup/utils"
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
// Returns the top 48 services ordered by recent report count (last 30 minutes)
func getServices(c *gin.Context, store *storage.Storage) {
	rows, err := store.ListServices(c.Request.Context())
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to fetch services"})
		return
	}

	response, err := buildServiceResponses(c, store, rows)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate service status"})
		return
	}

	utils.Respond(c, 200, "service-list", gin.H{
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
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to search services"})
		return
	}

	response, err := buildServiceResponses(c, store, rows)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate service status"})
		return
	}

	utils.Respond(c, 200, "service-list", gin.H{
		"services": response,
	})
}

// GET /api/service/:slug
func getService(c *gin.Context, store *storage.Storage) {
	slug := c.Param("slug")

	service, err := store.GetServiceBySlug(c.Request.Context(), slug)
	if err != nil {
		utils.Respond(c, 404, "service-not-found", gin.H{
			"error": "Service not found",
		})
		return
	}

	reportWindowStart := time.Now().Add(-algorithm.ReportWindow)
	recentReports, err := store.CountRecentReports(c.Request.Context(), service.ID, reportWindowStart)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to count recent reports"})
		return
	}

	now := time.Now().UTC()
	hourOfWeek := toHourOfWeek(now)
	baseline, err := store.GetBaselineForServiceHour(c.Request.Context(), service.ID, hourOfWeek)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load baseline"})
		return
	}

	recentProbeTotal, recentProbeFailures, err := store.GetRecentProbeStats(c.Request.Context(), service.ID, algorithm.RecentProbeWindow)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate probe data"})
		return
	}

	status := determineStatus(recentReports, baseline, recentProbeTotal, recentProbeFailures)

	response := ServiceDetailResponse{
		ID:            service.ID,
		Slug:          service.Slug,
		Name:          service.Name,
		URL:           service.HomepageURL,
		Category:      service.Category,
		Status:        string(status),
		RecentReports: recentReports,
	}

	utils.Respond(c, 200, "service-card", gin.H{
		"service": response,
	})
}

func buildServiceResponses(c *gin.Context, store *storage.Storage, rows []storage.ServiceRow) ([]ServiceResponse, error) {
	// Pull all baselines/probe stats in batches so list and search stay fast.
	serviceIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		serviceIDs = append(serviceIDs, row.ID)
	}

	hourOfWeek := toHourOfWeek(time.Now().UTC())
	baselines, err := store.GetBaselinesForServicesHour(c.Request.Context(), serviceIDs, hourOfWeek)
	if err != nil {
		return nil, err
	}

	probeStats, err := store.GetRecentProbeStatsForServices(c.Request.Context(), serviceIDs, algorithm.RecentProbeWindow)
	if err != nil {
		return nil, err
	}

	response := make([]ServiceResponse, len(rows))
	for i, row := range rows {
		baseline := baselines[row.ID]
		probe := probeStats[row.ID]
		// Same decision path used by the detail endpoint.
		status := determineStatus(row.RecentReportCount, &baseline, probe.RecentProbeTotal, probe.RecentProbeFailures)

		response[i] = ServiceResponse{
			ID:            row.ID,
			Slug:          row.Slug,
			Name:          row.Name,
			URL:           row.HomepageURL,
			Category:      row.Category,
			Status:        string(status),
			RecentReports: row.RecentReportCount,
		}
	}

	return response, nil
}

func determineStatus(recentReports int64, baseline *structs.ServiceBaseline, recentProbeTotal, recentProbeFailures int64) algorithm.Status {
	// Build the signal payload once, then let algorithm.DetermineStatus handle thresholds.
	signals := algorithm.Signals{
		RecentReports:       recentReports,
		RecentProbeTotal:    recentProbeTotal,
		RecentProbeFailures: recentProbeFailures,
	}

	if baseline != nil {
		signals.ReportBaselineMean = baseline.MeanReports
		signals.ReportBaselineStdDev = baseline.StdDevReports
		signals.ReportBaselineWeeks = baseline.SampleCount
		signals.ProbeBaselineFailureRate = baseline.ProbeFailureRate
		signals.ProbeBaselineSamples = baseline.ProbeFailureSamples
	}

	return algorithm.DetermineStatus(signals)
}

func toHourOfWeek(t time.Time) int {
	// 0..167 so every service has a baseline bucket for each weekday/hour slot.
	return int(t.Weekday())*24 + t.Hour()
}
