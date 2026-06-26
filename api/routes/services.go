package routes

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/api/middleware"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/novembersoftware/aretheyup/utils"
)

const servicesPerPage = 48

// GET /api/services?page=1
// Returns services ordered by recent report count (last 30 minutes) in paginated chunks.
func getServices(c *gin.Context, store *storage.Storage) {
	page := 1
	if rawPage := c.Query("page"); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	offset := (page - 1) * servicesPerPage
	rows, err := store.ListServices(c.Request.Context(), servicesPerPage+1, offset)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to fetch services"})
		return
	}

	hasMore := len(rows) > servicesPerPage
	if hasMore {
		rows = rows[:servicesPerPage]
	}

	response := utils.BuildServiceResponses(rows)

	utils.Respond(c, 200, "service-list", gin.H{
		"services": response,
		"append":   page > 1,
		"hasMore":  hasMore,
		"nextPage": page + 1,
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

	response := utils.BuildServiceResponses(rows)

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

	if _, err := store.RefreshServiceStatus(c.Request.Context(), service.ID, time.Now().UTC()); err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to refresh service status"})
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

// GET /api/services/count
func getServiceCount(c *gin.Context, store *storage.Storage) {
	count, err := store.GetServiceCount(c.Request.Context())
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to get service count"})
		return
	}
	utils.Respond(c, 200, "service-count", gin.H{"count": count})
}

// ----- HELPERS -----

// Respond with the service card for a given service
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

	statusSnapshot, err := store.GetServiceStatus(ctx, service.ID)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load service status"})
		return
	}
	if statusSnapshot == nil {
		statusSnapshot, err = store.RefreshServiceStatus(ctx, service.ID, now)
		if err != nil {
			utils.Respond(c, 500, "error", gin.H{"error": "Failed to refresh service status"})
			return
		}
	}

	probeDetail, err := store.GetProbeServiceDetail(ctx, service.ID, 50)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load probe detail"})
		return
	}

	status := algorithm.Status(statusSnapshot.Status)
	baseline := baselineFromStatusSnapshot(statusSnapshot)
	probePresentation := utils.BuildProbePresentation(probeDetail)

	histogramSince := now.Truncate(30 * time.Minute).Add(-47 * 30 * time.Minute)
	reportBuckets, err := store.GetReportBucketsForService(ctx, service.ID, histogramSince, 30*time.Minute)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load report timeline"})
		return
	}
	histogram := utils.BuildReportHistogram(now, reportBuckets, baseline, status)

	reportWindowStart := now.Add(-algorithm.ReportWindow)
	regionalCounts, err := store.GetRegionalReportCountsForService(ctx, service.ID, reportWindowStart, 8)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load regional report data"})
		return
	}
	regionalReports := utils.BuildRegionalReportBreakdown(regionalCounts, statusSnapshot.RecentReports)

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
	alertThreshold := math.Max(1, float64(statusSnapshot.RecentReports))
	if baseline != nil {
		baselineMean = baseline.MeanReports
		alertThreshold = math.Max(1, baseline.MeanReports+(2*baseline.StdDevReports))
	}
	windowUsage := int(math.Min(100, math.Round((float64(statusSnapshot.RecentReports)/alertThreshold)*100)))

	probeRecentFailures := int(statusSnapshot.RecentProbeFailures)
	probeRecentTotal := int(statusSnapshot.RecentProbeTotal)
	probeRecentSuccesses := probeRecentTotal - probeRecentFailures
	if probeRecentSuccesses < 0 {
		probeRecentSuccesses = 0
	}

	response := structs.ServiceDetailResponse{
		ID:                    service.ID,
		Slug:                  service.Slug,
		Name:                  service.Name,
		URL:                   service.HomepageURL,
		IconURL:               fmt.Sprintf("https://s2.googleusercontent.com/s2/favicons?sz=64&domain=%s", service.HomepageURL),
		Category:              service.Category,
		Status:                string(status),
		ComputedAt:            statusSnapshot.ComputedAt,
		ComputedAtLabel:       statusSnapshot.ComputedAt.UTC().Format("Jan 2, 3:04 PM MST"),
		RecentReports:         statusSnapshot.RecentReports,
		CanReport:             rateLimitState.CanReport,
		ReportRetryAfterSec:   rateLimitState.RetryAfterSeconds,
		ReportWindowLabel:     fmt.Sprintf("last %d min", int(algorithm.ReportWindow.Minutes())),
		BaselineMeanReports:   baselineMean,
		WindowUsagePercent:    windowUsage,
		UptimePercent:         uptimePercent,
		UptimeDays:            uptimeDays,
		OutageDayCount:        outageDays,
		ElevatedDayCount:      elevatedDays,
		ReportBuckets:         histogram,
		RegionalReports:       regionalReports,
		IncidentTimeline:      timeline,
		ProbeConfigured:       probeDetail.HasConfig,
		ProbeEnabled:          probeDetail.Enabled,
		ProbeRecentTotal:      probeRecentTotal,
		ProbeRecentSuccesses:  probeRecentSuccesses,
		ProbeRecentFailures:   probeRecentFailures,
		LastProbeCheckedLabel: probePresentation.LastCheckedLabel,
		LastProbeSuccessLabel: probePresentation.LastSuccessLabel,
		LastProbeFailureLabel: probePresentation.LastFailureLabel,
		LastProbeOutcome:      probePresentation.LastOutcome,
		LastProbeStatusCode:   probePresentation.LastStatusCode,
		LastProbeLatencyMs:    probePresentation.LastResponseTimeMs,
		ProbeLatencyAverageMs: probePresentation.LatencyAverageMs,
		ProbeLatencyMaxMs:     probePresentation.LatencyMaxMs,
		ProbeHistory:          probePresentation.History,
	}

	utils.Respond(c, 200, "service-card", gin.H{
		"service":  response,
		"reported": reported,
	})
}

func baselineFromStatusSnapshot(snapshot *structs.ServiceStatus) *structs.ServiceBaseline {
	if snapshot == nil || snapshot.BaselineSampleCount == 0 {
		return nil
	}

	return &structs.ServiceBaseline{
		ServiceID:            snapshot.ServiceID,
		HourOfWeek:           snapshot.HourOfWeek,
		MeanReports:          snapshot.BaselineMeanReports,
		StdDevReports:        snapshot.BaselineStdDevReports,
		SampleCount:          snapshot.BaselineSampleCount,
		ProbeFailureRate:     snapshot.ProbeBaselineFailureRate,
		ProbeFailureSamples:  snapshot.ProbeBaselineSamples,
		ProbeLatencyMedianMs: 0,
		ProbeLatencySamples:  0,
	}
}
