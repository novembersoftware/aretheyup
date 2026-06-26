package routes

import (
	"context"
	"errors"
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
	"gorm.io/gorm"
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
	limit := servicesPerPage + 1
	if response, ok := store.GetCachedServiceListResponses(c.Request.Context(), limit, offset); ok {
		respondServiceList(c, response, page)
		return
	}

	rows, err := store.ListServices(c.Request.Context(), limit, offset)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to fetch services"})
		return
	}

	response, err := utils.BuildServiceResponses(c, store, rows)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to evaluate service status"})
		return
	}

	store.SetCachedServiceListResponses(c.Request.Context(), limit, offset, response)
	respondServiceList(c, response, page)
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

	response, err := buildNeutralServiceDetailResponse(c.Request.Context(), store, service, time.Now().UTC())
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load service detail"})
		return
	}

	store.SetCachedServiceDetailResponse(c.Request.Context(), service.Slug, response)
	respondServiceCard(c, store, response, true)
}

// GET /api/service/:slug
func getService(c *gin.Context, store *storage.Storage) {
	slug := c.Param("slug")

	response, err := getCachedOrBuildServiceDetailResponse(c, store, slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Respond(c, 404, "service-not-found", gin.H{
				"error": "Service not found",
			})
			return
		}

		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load service detail"})
		return
	}

	respondServiceCard(c, store, response, false)
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

func respondServiceList(c *gin.Context, response []structs.ServiceResponse, page int) {
	hasMore := len(response) > servicesPerPage
	if hasMore {
		response = response[:servicesPerPage]
	}

	utils.Respond(c, 200, "service-list", gin.H{
		"services": response,
		"append":   page > 1,
		"hasMore":  hasMore,
		"nextPage": page + 1,
	})
}

func getCachedOrBuildServiceDetailResponse(c *gin.Context, store *storage.Storage, slug string) (structs.ServiceDetailResponse, error) {
	ctx := c.Request.Context()
	if response, ok := store.GetCachedServiceDetailResponse(ctx, slug); ok {
		return response, nil
	}

	service, err := store.GetServiceBySlug(ctx, slug)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	response, err := buildNeutralServiceDetailResponse(ctx, store, service, time.Now().UTC())
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	store.SetCachedServiceDetailResponse(ctx, slug, response)
	return response, nil
}

func reportRateLimitState(c *gin.Context, store *storage.Storage) middleware.ReportRateLimitState {
	rateLimitState, err := middleware.GetReportRateLimitState(
		c,
		store.Redis(),
		time.Duration(config.C.ReportRateLimitWindowSeconds)*time.Second,
	)
	if err != nil {
		return middleware.ReportRateLimitState{CanReport: true}
	}

	return rateLimitState
}

func mergeServiceDetailRequesterState(response structs.ServiceDetailResponse, rateLimitState middleware.ReportRateLimitState) structs.ServiceDetailResponse {
	response.CanReport = rateLimitState.CanReport
	response.ReportRetryAfterSec = rateLimitState.RetryAfterSeconds
	return response
}

// Respond with the service card for a given service
func respondServiceCard(c *gin.Context, store *storage.Storage, response structs.ServiceDetailResponse, reported bool) {
	response = mergeServiceDetailRequesterState(response, reportRateLimitState(c, store))

	utils.Respond(c, 200, "service-card", gin.H{
		"service":  response,
		"reported": reported,
	})
}

func buildNeutralServiceDetailResponse(ctx context.Context, store *storage.Storage, service *structs.Service, now time.Time) (structs.ServiceDetailResponse, error) {
	now = now.UTC()

	reportWindowStart := now.Add(-algorithm.ReportWindow)
	recentReports, err := store.CountRecentReports(ctx, service.ID, reportWindowStart)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	hourOfWeek := utils.ToHourOfWeek(now)
	baseline, err := store.GetBaselineForServiceHour(ctx, service.ID, hourOfWeek)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	recentProbeTotal, recentProbeFailures, err := store.GetRecentProbeStats(ctx, service.ID, algorithm.RecentProbeWindow)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	probeDetail, err := store.GetProbeServiceDetail(ctx, service.ID, 50)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	status := utils.DetermineStatus(recentReports, baseline, recentProbeTotal, recentProbeFailures)
	probePresentation := utils.BuildProbePresentation(probeDetail)

	histogramSince := now.Truncate(30 * time.Minute).Add(-47 * 30 * time.Minute)
	reportBuckets, err := store.GetReportBucketsForService(ctx, service.ID, histogramSince, 30*time.Minute)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}
	histogram := utils.BuildReportHistogram(now, reportBuckets, baseline, status)

	regionalCounts, err := store.GetRegionalReportCountsForService(ctx, service.ID, reportWindowStart, 8)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}
	regionalReports := utils.BuildRegionalReportBreakdown(regionalCounts, recentReports)

	windowStartDay := now.Truncate(24*time.Hour).AddDate(0, 0, -89)
	windowEnd := now
	windowIncidents, err := store.GetIncidentsOverlappingWindow(ctx, service.ID, windowStartDay, windowEnd)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	dailyReports, err := store.GetDailyReportCountsForService(ctx, service.ID, windowStartDay)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	uptimeDays, uptimePercent, outageDays, elevatedDays := utils.BuildUptimeDays(windowStartDay, 90, windowEnd, windowIncidents, dailyReports, status)

	incidents, err := store.GetRecentIncidentsForService(ctx, service.ID, windowStartDay, 20)
	if err != nil {
		return structs.ServiceDetailResponse{}, err
	}

	timeline := utils.BuildIncidentTimeline(incidents, now)

	baselineMean := 0.0
	alertThreshold := math.Max(1, float64(recentReports))
	if baseline != nil {
		baselineMean = baseline.MeanReports
		alertThreshold = math.Max(1, baseline.MeanReports+(2*baseline.StdDevReports))
	}
	windowUsage := int(math.Min(100, math.Round((float64(recentReports)/alertThreshold)*100)))

	return structs.ServiceDetailResponse{
		ID:                    service.ID,
		Slug:                  service.Slug,
		Name:                  service.Name,
		URL:                   service.HomepageURL,
		IconURL:               fmt.Sprintf("https://s2.googleusercontent.com/s2/favicons?sz=64&domain=%s", service.HomepageURL),
		Category:              service.Category,
		Status:                string(status),
		RecentReports:         recentReports,
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
		ProbeRecentTotal:      probePresentation.RecentTotal,
		ProbeRecentSuccesses:  probePresentation.RecentSuccesses,
		ProbeRecentFailures:   probePresentation.RecentFailures,
		LastProbeCheckedLabel: probePresentation.LastCheckedLabel,
		LastProbeSuccessLabel: probePresentation.LastSuccessLabel,
		LastProbeFailureLabel: probePresentation.LastFailureLabel,
		LastProbeOutcome:      probePresentation.LastOutcome,
		LastProbeStatusCode:   probePresentation.LastStatusCode,
		LastProbeLatencyMs:    probePresentation.LastResponseTimeMs,
		ProbeLatencyAverageMs: probePresentation.LatencyAverageMs,
		ProbeLatencyMaxMs:     probePresentation.LatencyMaxMs,
		ProbeHistory:          probePresentation.History,
	}, nil
}
