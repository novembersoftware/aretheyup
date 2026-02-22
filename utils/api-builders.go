package utils

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
)

func BuildRegionalReportBreakdown(regionalCounts []storage.RegionalReportCount, total int64) []RegionalReportResponse {
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

func BuildServiceResponses(c *gin.Context, store *storage.Storage, rows []storage.ServiceRow) ([]ServiceResponse, error) {
	serviceIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		serviceIDs = append(serviceIDs, row.ID)
	}

	hourOfWeek := ToHourOfWeek(time.Now().UTC())
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
		status := DetermineStatus(row.RecentReportCount, &baseline, probe.RecentProbeTotal, probe.RecentProbeFailures)

		response[i] = ServiceResponse{
			ID:            row.ID,
			Slug:          row.Slug,
			Name:          row.Name,
			URL:           row.HomepageURL,
			IconURL:       fmt.Sprintf("https://s2.googleusercontent.com/s2/favicons?sz=64&domain=%s", row.HomepageURL),
			Category:      row.Category,
			Status:        string(status),
			RecentReports: row.RecentReportCount,
		}
	}

	return response, nil
}

func BuildReportHistogram(now time.Time, buckets []storage.ReportBucket, baseline *structs.ServiceBaseline, currentStatus algorithm.Status) []ReportBucketResponse {
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

func BuildUptimeDays(windowStart time.Time, totalDays int, now time.Time, incidents []structs.Incident, dailyReports []storage.DailyReportCount, currentStatus algorithm.Status) ([]UptimeDayResponse, float64, int, int) {
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

func BuildIncidentTimeline(incidents []structs.Incident, now time.Time) []IncidentEntryResponse {
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
