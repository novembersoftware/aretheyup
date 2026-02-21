package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
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
	ID                  uint                     `json:"id"`
	Slug                string                   `json:"slug"`
	Name                string                   `json:"name"`
	URL                 string                   `json:"url"`
	Category            string                   `json:"category"`
	Status              string                   `json:"status"`
	RecentReports       int64                    `json:"recent_reports"`
	ReportWindowLabel   string                   `json:"report_window_label"`
	BaselineMeanReports float64                  `json:"baseline_mean_reports"`
	WindowUsagePercent  int                      `json:"window_usage_percent"`
	UptimePercent       float64                  `json:"uptime_percent"`
	UptimeDays          []UptimeDayResponse      `json:"uptime_days"`
	OutageDayCount      int                      `json:"outage_day_count"`
	ElevatedDayCount    int                      `json:"elevated_day_count"`
	ReportBuckets       []ReportBucketResponse   `json:"report_buckets"`
	RegionalReports     []RegionalReportResponse `json:"regional_reports"`
	IncidentTimeline    []IncidentEntryResponse  `json:"incident_timeline"`
}

type ReportBucketResponse struct {
	Label     string `json:"label"`
	Count     int64  `json:"count"`
	HeightPct int    `json:"height_pct"`
	Level     string `json:"level"`
}

type UptimeDayResponse struct {
	Label string `json:"label"`
	Level string `json:"level"`
}

type IncidentEntryResponse struct {
	StartedAtLabel  string `json:"started_at_label"`
	ResolvedAtLabel string `json:"resolved_at_label"`
	DurationLabel   string `json:"duration_label"`
	Ongoing         bool   `json:"ongoing"`
}

type RegionalReportResponse struct {
	Region  string `json:"region"`
	Count   int64  `json:"count"`
	Percent int    `json:"percent"`
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

	respondServiceCard(c, store, service, false)
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

	userAgent := c.GetHeader("User-Agent")
	if userAgent == "" {
		userAgent = "unknown"
	}

	report := structs.UserReport{
		ServiceID:   service.ID,
		IPAddress:   c.ClientIP(),
		UserAgent:   userAgent,
		Fingerprint: requestFingerprint(c),
		Region:      requestRegion(c),
	}

	if err := store.CreateUserReport(c.Request.Context(), &report); err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to create report"})
		return
	}

	respondServiceCard(c, store, service, true)
}

func respondServiceCard(c *gin.Context, store *storage.Storage, service *structs.Service, reported bool) {
	ctx := c.Request.Context()
	now := time.Now().UTC()

	reportWindowStart := now.Add(-algorithm.ReportWindow)
	recentReports, err := store.CountRecentReports(ctx, service.ID, reportWindowStart)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to count recent reports"})
		return
	}

	hourOfWeek := toHourOfWeek(now)
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

	status := determineStatus(recentReports, baseline, recentProbeTotal, recentProbeFailures)

	histogramSince := now.Truncate(30 * time.Minute).Add(-47 * 30 * time.Minute)
	reportBuckets, err := store.GetReportBucketsForService(ctx, service.ID, histogramSince, 30*time.Minute)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load report timeline"})
		return
	}
	histogram := buildReportHistogram(now, reportBuckets, baseline, status)

	regionalCounts, err := store.GetRegionalReportCountsForService(ctx, service.ID, reportWindowStart, 8)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load regional report data"})
		return
	}
	regionalReports := buildRegionalReportBreakdown(regionalCounts, recentReports)

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

	uptimeDays, uptimePercent, outageDays, elevatedDays := buildUptimeDays(windowStartDay, 90, windowEnd, windowIncidents, dailyReports, status)

	incidents, err := store.GetRecentIncidentsForService(ctx, service.ID, windowStartDay, 20)
	if err != nil {
		utils.Respond(c, 500, "error", gin.H{"error": "Failed to load incident timeline"})
		return
	}

	timeline := buildIncidentTimeline(incidents, now)

	baselineMean := 0.0
	alertThreshold := math.Max(1, float64(recentReports))
	if baseline != nil {
		baselineMean = baseline.MeanReports
		alertThreshold = math.Max(1, baseline.MeanReports+(2*baseline.StdDevReports))
	}
	windowUsage := int(math.Min(100, math.Round((float64(recentReports)/alertThreshold)*100)))

	response := ServiceDetailResponse{
		ID:                  service.ID,
		Slug:                service.Slug,
		Name:                service.Name,
		URL:                 service.HomepageURL,
		Category:            service.Category,
		Status:              string(status),
		RecentReports:       recentReports,
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

func requestFingerprint(c *gin.Context) string {
	fingerprint := c.GetHeader("X-Fingerprint")
	if fingerprint != "" {
		return fingerprint
	}

	hash := sha256.Sum256([]byte(c.ClientIP() + "|" + c.GetHeader("User-Agent") + "|" + c.GetHeader("Accept-Language")))
	return hex.EncodeToString(hash[:])
}

func requestRegion(c *gin.Context) string {
	for _, key := range []string{
		"X-Region",
		"X-Country-Code",
		"CF-IPCountry",
		"CloudFront-Viewer-Country",
		"X-AppEngine-Country",
	} {
		if value := sanitizeRegion(c.GetHeader(key)); value != "" {
			return value
		}
	}

	if languageRegion := regionFromAcceptLanguage(c.GetHeader("Accept-Language")); languageRegion != "" {
		return languageRegion
	}

	return "Unknown"
}

func sanitizeRegion(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" || v == "XX" || v == "T1" || v == "A1" || v == "UNKNOWN" {
		return ""
	}
	if len(v) > 24 {
		return v[:24]
	}
	return v
}

func regionFromAcceptLanguage(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}

	first := strings.TrimSpace(parts[0])
	if first == "" {
		return ""
	}

	lang := strings.Split(first, ";")[0]
	chunks := strings.Split(lang, "-")
	if len(chunks) < 2 {
		return ""
	}

	return sanitizeRegion(chunks[len(chunks)-1])
}

func buildRegionalReportBreakdown(regionalCounts []storage.RegionalReportCount, total int64) []RegionalReportResponse {
	if len(regionalCounts) == 0 {
		return []RegionalReportResponse{}
	}

	denominator := float64(total)
	if denominator <= 0 {
		for _, row := range regionalCounts {
			denominator += float64(row.Count)
		}
	}
	if denominator <= 0 {
		denominator = 1
	}

	response := make([]RegionalReportResponse, len(regionalCounts))
	for i, row := range regionalCounts {
		pct := int(math.Round((float64(row.Count) / denominator) * 100))
		if pct == 0 && row.Count > 0 {
			pct = 1
		}
		response[i] = RegionalReportResponse{
			Region:  row.Region,
			Count:   row.Count,
			Percent: pct,
		}
	}

	return response
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

func buildReportHistogram(now time.Time, buckets []storage.ReportBucket, baseline *structs.ServiceBaseline, currentStatus algorithm.Status) []ReportBucketResponse {
	byStart := make(map[int64]int64, len(buckets))
	for _, bucket := range buckets {
		byStart[bucket.Start.UTC().Unix()] = bucket.Count
	}

	start := now.UTC().Truncate(30 * time.Minute).Add(-47 * 30 * time.Minute)
	points := make([]ReportBucketResponse, 0, 48)
	maxCount := int64(1)

	for i := 0; i < 48; i++ {
		bucketTime := start.Add(time.Duration(i) * 30 * time.Minute)
		count := byStart[bucketTime.Unix()]
		if count > maxCount {
			maxCount = count
		}

		points = append(points, ReportBucketResponse{
			Label: bucketTime.Format("3:04 PM"),
			Count: count,
		})
	}

	warningThreshold, errorThreshold := reportLevelThresholds(points)
	if baseline != nil {
		baselineWarning := math.Max(2, baseline.MeanReports)
		baselineError := math.Max(5, baseline.MeanReports+(2*baseline.StdDevReports))
		// Keep colors stable across day boundaries: prefer recent-distribution thresholds,
		// but don't go below a reasonable baseline-derived floor.
		warningThreshold = math.Max(warningThreshold, baselineWarning*0.6)
		errorThreshold = math.Max(errorThreshold, baselineError*0.6)
	}

	for i := range points {
		height := int(math.Round((float64(points[i].Count) / float64(maxCount)) * 100))
		if height < 6 && points[i].Count > 0 {
			height = 6
		}
		if height == 0 {
			height = 4
		}
		points[i].HeightPct = height

		level := "normal"
		if float64(points[i].Count) >= errorThreshold {
			level = "high"
		} else if float64(points[i].Count) >= warningThreshold {
			level = "elevated"
		}
		points[i].Level = level
	}

	if currentStatus == algorithm.StatusIssuesDetected && len(points) > 0 {
		last := len(points) - 1
		points[last].Level = "high"
		if points[last].HeightPct < 20 {
			points[last].HeightPct = 20
		}
	}

	return points
}

func reportLevelThresholds(points []ReportBucketResponse) (float64, float64) {
	nonZero := make([]int64, 0, len(points))
	for _, p := range points {
		if p.Count > 0 {
			nonZero = append(nonZero, p.Count)
		}
	}

	if len(nonZero) == 0 {
		return 2, 5
	}

	sort.Slice(nonZero, func(i, j int) bool { return nonZero[i] < nonZero[j] })

	p70 := percentileCount(nonZero, 0.70)
	p90 := percentileCount(nonZero, 0.90)

	warning := math.Max(2, float64(p70))
	error := math.Max(warning+1, float64(p90))

	return warning, error
}

func percentileCount(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	idx := int(math.Ceil(float64(len(sorted))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}

func buildUptimeDays(windowStart time.Time, totalDays int, now time.Time, incidents []structs.Incident, dailyReports []storage.DailyReportCount, currentStatus algorithm.Status) ([]UptimeDayResponse, float64, int, int) {
	incidentDays := map[string]bool{}
	reportDays := map[string]int64{}

	for _, row := range dailyReports {
		key := row.Day.UTC().Format("2006-01-02")
		reportDays[key] = row.Count
	}

	for _, incident := range incidents {
		start := incident.StartedAt.UTC()
		end := now.UTC()
		if incident.ResolvedAt != nil {
			end = incident.ResolvedAt.UTC()
		}

		if end.Before(windowStart) {
			continue
		}
		if start.Before(windowStart) {
			start = windowStart
		}

		day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		lastDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
		for !day.After(lastDay) {
			incidentDays[day.Format("2006-01-02")] = true
			day = day.AddDate(0, 0, 1)
		}
	}

	days := make([]UptimeDayResponse, 0, totalDays)
	upDays := 0
	outageDays := 0
	elevatedDays := 0

	for i := 0; i < totalDays; i++ {
		day := windowStart.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		level := "operational"
		label := day.Format("Jan 2")

		if incidentDays[key] {
			level = "outage"
			outageDays++
			label = label + " - incident"
		} else {
			if reports := reportDays[key]; reports > 0 {
				level = "elevated"
				elevatedDays++
				label = fmt.Sprintf("%s - %d reports", label, reports)
			}
			upDays++
		}

		days = append(days, UptimeDayResponse{Label: label, Level: level})
	}

	if currentStatus == algorithm.StatusIssuesDetected && len(days) > 0 {
		last := len(days) - 1
		if days[last].Level == "operational" {
			upDays--
		}
		if days[last].Level == "elevated" {
			elevatedDays--
			upDays--
		}
		if days[last].Level != "outage" {
			days[last].Level = "outage"
			days[last].Label = days[last].Label + " - current issue"
			outageDays++
		}
	}

	uptimePercent := (float64(upDays) / float64(totalDays)) * 100
	return days, uptimePercent, outageDays, elevatedDays
}

func buildIncidentTimeline(incidents []structs.Incident, now time.Time) []IncidentEntryResponse {
	if len(incidents) == 0 {
		return []IncidentEntryResponse{}
	}

	items := make([]IncidentEntryResponse, 0, len(incidents))
	for _, incident := range incidents {
		start := incident.StartedAt.UTC()
		end := now.UTC()
		ongoing := incident.ResolvedAt == nil
		resolvedLabel := "Active now"

		if incident.ResolvedAt != nil {
			end = incident.ResolvedAt.UTC()
			resolvedLabel = incident.ResolvedAt.UTC().Format("Jan 2, 3:04 PM MST")
		}

		items = append(items, IncidentEntryResponse{
			StartedAtLabel:  start.Format("Jan 2, 3:04 PM MST"),
			ResolvedAtLabel: resolvedLabel,
			DurationLabel:   formatDuration(end.Sub(start)),
			Ongoing:         ongoing,
		})
	}

	return items
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "under a minute"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}

	return fmt.Sprintf("%dh %dm", hours, minutes)
}
