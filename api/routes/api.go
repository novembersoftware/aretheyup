package routes

import (
	"fmt"
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/api/middleware"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/novembersoftware/aretheyup/utils"
)

// GET /api/services
// Returns the top 48 services ordered by recent report count (last 30 minutes)
func getServices(c *gin.Context, store *storage.Storage) {
	rows, err := store.ListServices(c.Request.Context())
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to fetch services"})
		return
	}

	response, err := utils.BuildServiceResponses(c, store, rows)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate service status"})
		return
	}

	utils.Respond(c, 200, "service-list", gin.H{
		"services": response,
	})
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

	response, err := utils.BuildServiceResponses(c, store, rows)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate service status"})
		return
	}

	utils.Respond(c, 200, "service-list", gin.H{
		"services": response,
	})
}

// POST /api/service/:slug/report
func createServiceReport(c *gin.Context, store *storage.Storage) {
	slug := c.Param("slug")

	service, err := store.GetServiceBySlug(c.Request.Context(), slug)
	if err != nil {
		utils.Respond(c, 404, "service-not-found", gin.H{
			"error": "Service not found",
		})
		return
	}

	report := structs.UserReport{
		ServiceID:   service.ID,
		Fingerprint: utils.RequestFingerprint(c),
		Region:      utils.RequestRegion(c),
	}

	if err := store.CreateUserReport(c.Request.Context(), &report); err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to create report"})
		return
	}

	respondServiceCard(c, store, service, true)
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

	respondServiceCard(c, store, service, false)
}

func respondServiceCard(c *gin.Context, store *storage.Storage, service *structs.Service, reported bool) {
	ctx := c.Request.Context()
	now := time.Now().UTC()

	rateLimitState, err := middleware.GetReportRateLimitState(
		c,
		store.Redis(),
		time.Duration(config.C.ReportRateLimitWindowSeconds)*time.Second,
	)
	if err != nil {
		rateLimitState = middleware.ReportRateLimitState{CanReport: true}
	}

	reportWindowStart := now.Add(-algorithm.ReportWindow)
	recentReports, err := store.CountRecentReports(ctx, service.ID, reportWindowStart)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to count recent reports"})
		return
	}

	hourOfWeek := utils.ToHourOfWeek(now)
	baseline, err := store.GetBaselineForServiceHour(ctx, service.ID, hourOfWeek)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load baseline"})
		return
	}

	recentProbeTotal, recentProbeFailures, err := store.GetRecentProbeStats(ctx, service.ID, algorithm.RecentProbeWindow)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate probe data"})
		return
	}

	status := utils.DetermineStatus(recentReports, baseline, recentProbeTotal, recentProbeFailures)

	histogramSince := now.Truncate(30 * time.Minute).Add(-47 * 30 * time.Minute)
	reportBuckets, err := store.GetReportBucketsForService(ctx, service.ID, histogramSince, 30*time.Minute)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load report timeline"})
		return
	}
	histogram := utils.BuildReportHistogram(now, reportBuckets, baseline, status)

	regionalCounts, err := store.GetRegionalReportCountsForService(ctx, service.ID, reportWindowStart, 8)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load regional report data"})
		return
	}
	regionalReports := utils.BuildRegionalReportBreakdown(regionalCounts, recentReports)

	windowStartDay := now.Truncate(24*time.Hour).AddDate(0, 0, -89)
	windowEnd := now
	windowIncidents, err := store.GetIncidentsOverlappingWindow(ctx, service.ID, windowStartDay, windowEnd)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load uptime history"})
		return
	}

	dailyReports, err := store.GetDailyReportCountsForService(ctx, service.ID, windowStartDay)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load report history"})
		return
	}

	uptimeDays, uptimePercent, outageDays, elevatedDays := utils.BuildUptimeDays(windowStartDay, 90, windowEnd, windowIncidents, dailyReports, status)

	incidents, err := store.GetRecentIncidentsForService(ctx, service.ID, windowStartDay, 20)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load incident timeline"})
		return
	}

	timeline := utils.BuildIncidentTimeline(incidents, now)

	baselineMean := 0.0
	alertThreshold := math.Max(1, float64(recentReports))
	if baseline != nil {
		baselineMean = baseline.MeanReports
		alertThreshold = math.Max(1, baseline.MeanReports+(2*baseline.StdDevReports))
	}
	windowUsage := int(math.Min(100, math.Round((float64(recentReports)/alertThreshold)*100)))

	response := structs.ServiceDetailResponse{
		ID:                  service.ID,
		Slug:                service.Slug,
		Name:                service.Name,
		URL:                 service.HomepageURL,
		IconURL:             fmt.Sprintf("https://s2.googleusercontent.com/s2/favicons?sz=64&domain=%s", service.HomepageURL),
		Category:            service.Category,
		Status:              string(status),
		RecentReports:       recentReports,
		CanReport:           rateLimitState.CanReport,
		ReportRetryAfterSec: rateLimitState.RetryAfterSeconds,
		ReportWindowLabel:   fmt.Sprintf("last %d min", int(algorithm.ReportWindow.Minutes())),
		BaselineMeanReports: baselineMean,
		WindowUsagePercent:  windowUsage,
		UptimePercent:       uptimePercent,
		UptimeDays:          uptimeDays,
		OutageDayCount:      outageDays,
		ElevatedDayCount:    elevatedDays,
		ReportBuckets:       histogram,
		RegionalReports:     regionalReports,
		IncidentTimeline:    timeline,
	}

	utils.Respond(c, 200, "service-card", gin.H{
		"service":  response,
		"reported": reported,
	})
}
