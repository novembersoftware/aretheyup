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
	var rows []storage.ServiceRow
	var err error
	if config.StatusSnapshotAPIReadsEnabled() {
		rows, err = store.ListServicesFromStatusSnapshots(c.Request.Context(), servicesPerPage+1, offset)
	} else {
		rows, err = store.ListServices(c.Request.Context(), servicesPerPage+1, offset)
	}
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to fetch services"})
		return
	}

	hasMore := len(rows) > servicesPerPage
	if hasMore {
		rows = rows[:servicesPerPage]
	}

	response, err := buildServiceListResponses(c, store, rows)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate service status"})
		return
	}

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

	var rows []storage.ServiceRow
	var err error
	if config.StatusSnapshotAPIReadsEnabled() {
		rows, err = store.SearchServicesFromStatusSnapshots(c.Request.Context(), q)
	} else {
		rows, err = store.SearchServices(c.Request.Context(), q)
	}
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to search services"})
		return
	}

	response, err := buildServiceListResponses(c, store, rows)
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

	if config.StatusSnapshotAPIReadsEnabled() {
		if _, err := store.RefreshServiceStatus(c.Request.Context(), service.ID, time.Now().UTC()); err != nil {
			utils.Respond(c, 500, "error", gin.H{"error": "Failed to refresh service status"})
			return
		}
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

func buildServiceListResponses(c *gin.Context, store *storage.Storage, rows []storage.ServiceRow) ([]structs.ServiceResponse, error) {
	if config.StatusSnapshotAPIReadsEnabled() {
		return utils.BuildServiceResponsesFromSnapshots(rows), nil
	}
	return utils.BuildServiceResponses(c, store, rows)
}

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

	reportWindowStart := now.Add(-algorithm.ReportWindow)
	var status algorithm.Status
	var recentReports int64
	var baseline *structs.ServiceBaseline
	var recentProbeTotal int64
	var recentProbeFailures int64
	var computedAt time.Time
	if config.StatusSnapshotAPIReadsEnabled() {
		statusSnapshot, err := store.GetServiceStatus(ctx, service.ID)
		if err != nil {
			utils.Respond(c, 500, "error", gin.H{"error": "Failed to load service status"})
			return
		}
		if statusSnapshot == nil {
			utils.Respond(c, 503, "error", gin.H{"error": "Service status is not ready"})
			return
		}

		status = statusFromSnapshot(statusSnapshot)
		recentReports = statusSnapshot.RecentReports
		recentProbeTotal = statusSnapshot.RecentProbeTotal
		recentProbeFailures = statusSnapshot.RecentProbeFailures
		baseline = baselineFromStatusSnapshot(statusSnapshot)
		computedAt = statusSnapshot.ComputedAt
	} else {
		var err error
		recentReports, err = store.CountRecentReports(ctx, service.ID, reportWindowStart)
		if err != nil {
			utils.Respond(c, 500, "error", gin.H{"error": "Failed to count recent reports"})
			return
		}

		hourOfWeek := utils.ToHourOfWeek(now)
		baseline, err = store.GetBaselineForServiceHour(ctx, service.ID, hourOfWeek)
		if err != nil {
			utils.Respond(c, 500, "error", gin.H{"error": "Failed to load baseline"})
			return
		}

		recentProbeTotal, recentProbeFailures, err = store.GetRecentProbeStats(ctx, service.ID, algorithm.RecentProbeWindow)
		if err != nil {
			utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate probe data"})
			return
		}

		status = utils.DetermineStatus(recentReports, baseline, recentProbeTotal, recentProbeFailures)
	}

	probeDetail, err := store.GetProbeServiceDetail(ctx, service.ID, 50)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load probe detail"})
		return
	}

	probePresentation := utils.BuildProbePresentation(probeDetail)

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

	probeRecentTotal := probePresentation.RecentTotal
	probeRecentFailures := probePresentation.RecentFailures
	probeRecentSuccesses := probePresentation.RecentSuccesses
	if config.StatusSnapshotAPIReadsEnabled() {
		probeRecentTotal = int(recentProbeTotal)
		probeRecentFailures = int(recentProbeFailures)
		probeRecentSuccesses = probeRecentTotal - probeRecentFailures
		if probeRecentSuccesses < 0 {
			probeRecentSuccesses = 0
		}
	}
	computedAtLabel := ""
	if !computedAt.IsZero() {
		computedAtLabel = computedAt.UTC().Format("Jan 2, 3:04 PM MST")
	}

	response := structs.ServiceDetailResponse{
		ID:                    service.ID,
		Slug:                  service.Slug,
		Name:                  service.Name,
		URL:                   service.HomepageURL,
		IconURL:               fmt.Sprintf("https://s2.googleusercontent.com/s2/favicons?sz=64&domain=%s", service.HomepageURL),
		Category:              service.Category,
		Status:                string(status),
		ComputedAt:            computedAt,
		ComputedAtLabel:       computedAtLabel,
		RecentReports:         recentReports,
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

func statusFromSnapshot(snapshot *structs.ServiceStatus) algorithm.Status {
	if snapshot == nil {
		return algorithm.StatusOperational
	}

	switch algorithm.Status(snapshot.Status) {
	case algorithm.StatusOperational, algorithm.StatusDegraded, algorithm.StatusOutage:
		return algorithm.Status(snapshot.Status)
	default:
		return algorithm.StatusOperational
	}
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
