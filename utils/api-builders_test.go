package utils

import (
	"math"
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
)

func TestBuildRegionalReportBreakdown(t *testing.T) {
	// Partition coverage:
	// - empty input
	// - explicit denominator
	// - fallback denominator derived from counts
	tests := []struct {
		name   string
		counts []storage.RegionalReportCount
		total  int64
		want   []structs.RegionalReportResponse
	}{
		{
			name:   "empty input returns empty slice",
			counts: []storage.RegionalReportCount{},
			total:  0,
			want:   []structs.RegionalReportResponse{},
		},
		{
			name: "uses explicit total denominator",
			counts: []storage.RegionalReportCount{
				{Region: "US", Count: 3},
				{Region: "CA", Count: 7},
			},
			total: 10,
			want: []structs.RegionalReportResponse{
				{Region: "US", Count: 3, Percent: 30},
				{Region: "CA", Count: 7, Percent: 70},
			},
		},
		{
			name: "falls back to summed denominator when total is non-positive",
			counts: []storage.RegionalReportCount{
				{Region: "US", Count: 1},
				{Region: "CA", Count: 1},
				{Region: "GB", Count: 0},
			},
			total: 0,
			want: []structs.RegionalReportResponse{
				{Region: "US", Count: 1, Percent: 50},
				{Region: "CA", Count: 1, Percent: 50},
				{Region: "GB", Count: 0, Percent: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRegionalReportBreakdown(tt.counts, tt.total)
			if len(got) != len(tt.want) {
				t.Fatalf("len(BuildRegionalReportBreakdown) = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("BuildRegionalReportBreakdown[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildServiceResponsesFromData(t *testing.T) {
	// This isolates list-item shaping logic from storage access so expectations stay deterministic.
	rows := []storage.ServiceRow{
		{ID: 1, Slug: "one", Name: "One", HomepageURL: "https://one.example", Category: "infra", RecentReportCount: 16},
		{ID: 2, Slug: "two", Name: "Two", HomepageURL: "https://two.example", Category: "infra", RecentReportCount: 2},
		{ID: 3, Slug: "three", Name: "Three", HomepageURL: "https://three.example", Category: "infra", RecentReportCount: 1},
	}

	baselines := map[uint]structs.ServiceBaseline{
		2: {
			ServiceID:     2,
			MeanReports:   1,
			StdDevReports: 1,
			SampleCount:   4,
		},
	}

	probeStats := map[uint]storage.ProbeStats{
		3: {
			ServiceID:           3,
			RecentProbeTotal:    5,
			RecentProbeFailures: 4,
		},
	}

	got := buildServiceResponsesFromData(rows, baselines, probeStats)
	if len(got) != 3 {
		t.Fatalf("len(buildServiceResponsesFromData) = %d, want 3", len(got))
	}

	if got[0].Status != string(algorithm.StatusIssuesDetected) {
		t.Fatalf("service 1 status = %q, want %q", got[0].Status, algorithm.StatusIssuesDetected)
	}
	if got[1].Status != string(algorithm.StatusOperational) {
		t.Fatalf("service 2 status = %q, want %q", got[1].Status, algorithm.StatusOperational)
	}
	if got[2].Status != string(algorithm.StatusIssuesDetected) {
		t.Fatalf("service 3 status = %q, want %q", got[2].Status, algorithm.StatusIssuesDetected)
	}

	if got[1].IconURL != "https://s2.googleusercontent.com/s2/favicons?sz=64&domain=https://two.example" {
		t.Fatalf("unexpected icon URL %q", got[1].IconURL)
	}
}

func TestBuildReportHistogram(t *testing.T) {
	// Validate the chart contract:
	// - exactly 48 half-hour buckets
	// - stable labels/count placement
	// - floor heights and severity levels
	now := time.Date(2026, time.January, 10, 12, 15, 0, 0, time.UTC)
	start := now.UTC().Truncate(30 * time.Minute).Add(-47 * 30 * time.Minute)

	buckets := []storage.ReportBucket{
		{Start: start, Count: 1},
		{Start: start.Add(30 * time.Minute), Count: 10},
	}

	points := BuildReportHistogram(now, buckets, nil, algorithm.StatusOperational)
	if len(points) != 48 {
		t.Fatalf("len(BuildReportHistogram) = %d, want 48", len(points))
	}

	if points[0].Label != start.Format("3:04 PM") {
		t.Fatalf("first label = %q, want %q", points[0].Label, start.Format("3:04 PM"))
	}
	if points[0].Count != 1 {
		t.Fatalf("first count = %d, want 1", points[0].Count)
	}
	if points[1].Count != 10 {
		t.Fatalf("second count = %d, want 10", points[1].Count)
	}
	if points[2].HeightPct != 4 {
		t.Fatalf("zero-count height = %d, want 4", points[2].HeightPct)
	}
	if points[1].Level != "elevated" {
		t.Fatalf("count 10 level = %q, want elevated", points[1].Level)
	}

	forced := BuildReportHistogram(now, nil, nil, algorithm.StatusIssuesDetected)
	last := forced[len(forced)-1]
	if last.Level != "high" {
		t.Fatalf("last level during active issue = %q, want high", last.Level)
	}
	if last.HeightPct != 20 {
		t.Fatalf("last height during active issue = %d, want 20", last.HeightPct)
	}
}

func TestBuildUptimeDays(t *testing.T) {
	// Covers normal timeline accounting plus "active issue" override semantics on the latest day.
	windowStart := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC)

	resolvedAt := time.Date(2026, time.January, 1, 11, 0, 0, 0, time.UTC)
	incidents := []structs.Incident{
		{StartedAt: time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC), ResolvedAt: &resolvedAt},
	}
	reports := []storage.DailyReportCount{{Day: time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC), Count: 2}}

	days, uptime, outageCount, elevatedCount := BuildUptimeDays(windowStart, 3, now, incidents, reports, algorithm.StatusOperational)
	if len(days) != 3 {
		t.Fatalf("len(BuildUptimeDays) = %d, want 3", len(days))
	}
	if days[0].Level != "outage" || days[1].Level != "elevated" || days[2].Level != "operational" {
		t.Fatalf("unexpected day levels: %+v", days)
	}
	if outageCount != 1 || elevatedCount != 1 {
		t.Fatalf("counts = outage:%d elevated:%d, want outage:1 elevated:1", outageCount, elevatedCount)
	}
	if math.Abs(uptime-66.6666666667) > 0.0001 {
		t.Fatalf("uptime = %.4f, want ~66.6667", uptime)
	}

	daysIssue, uptimeIssue, outageIssue, elevatedIssue := BuildUptimeDays(windowStart, 3, now, incidents, reports, algorithm.StatusIssuesDetected)
	if daysIssue[2].Level != "outage" {
		t.Fatalf("last day level during active issue = %q, want outage", daysIssue[2].Level)
	}
	if outageIssue != 2 || elevatedIssue != 1 {
		t.Fatalf("issue counts = outage:%d elevated:%d, want outage:2 elevated:1", outageIssue, elevatedIssue)
	}
	if math.Abs(uptimeIssue-33.3333333333) > 0.0001 {
		t.Fatalf("uptime with active issue = %.4f, want ~33.3333", uptimeIssue)
	}
}

func TestBuildIncidentTimeline(t *testing.T) {
	// Validate label/format behavior for resolved and ongoing incidents.
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	resolvedAt := time.Date(2026, time.January, 10, 11, 30, 0, 0, time.UTC)

	incidents := []structs.Incident{
		{
			StartedAt:  time.Date(2026, time.January, 10, 10, 0, 0, 0, time.UTC),
			ResolvedAt: &resolvedAt,
		},
		{
			StartedAt: time.Date(2026, time.January, 10, 11, 59, 45, 0, time.UTC),
		},
	}

	timeline := BuildIncidentTimeline(incidents, now)
	if len(timeline) != 2 {
		t.Fatalf("len(BuildIncidentTimeline) = %d, want 2", len(timeline))
	}

	if timeline[0].ResolvedAtLabel != "Jan 10, 11:30 AM UTC" {
		t.Fatalf("resolved label = %q, want Jan 10, 11:30 AM UTC", timeline[0].ResolvedAtLabel)
	}
	if timeline[0].DurationLabel != "1h 30m" {
		t.Fatalf("duration label = %q, want 1h 30m", timeline[0].DurationLabel)
	}
	if timeline[1].ResolvedAtLabel != "Active now" || !timeline[1].Ongoing {
		t.Fatalf("ongoing item = %+v, want Active now + ongoing=true", timeline[1])
	}
	if timeline[1].DurationLabel != "under a minute" {
		t.Fatalf("ongoing duration label = %q, want under a minute", timeline[1].DurationLabel)
	}
}

func TestHelperThresholdAndDurationFunctions(t *testing.T) {
	// These are low-level primitives used by the chart and timeline builders.
	warning, errThreshold := reportLevelThresholds([]structs.ReportBucketResponse{{Count: 0}, {Count: 0}})
	if warning != 2 || errThreshold != 5 {
		t.Fatalf("reportLevelThresholds(empty) = (%.1f, %.1f), want (2.0, 5.0)", warning, errThreshold)
	}

	warning, errThreshold = reportLevelThresholds([]structs.ReportBucketResponse{{Count: 1}, {Count: 2}, {Count: 3}, {Count: 4}, {Count: 5}, {Count: 6}, {Count: 7}, {Count: 8}, {Count: 9}, {Count: 10}})
	if warning != 7 || errThreshold != 9 {
		t.Fatalf("reportLevelThresholds(populated) = (%.1f, %.1f), want (7.0, 9.0)", warning, errThreshold)
	}

	if got := percentileCount([]int64{1, 3, 5, 7, 9}, 0.5); got != 5 {
		t.Fatalf("percentileCount(p50) = %d, want 5", got)
	}
	if got := percentileCount([]int64{1, 3, 5}, 0); got != 1 {
		t.Fatalf("percentileCount(p0) = %d, want 1", got)
	}
	if got := percentileCount([]int64{1, 3, 5}, 1); got != 5 {
		t.Fatalf("percentileCount(p100) = %d, want 5", got)
	}

	if got := formatDuration(45 * time.Second); got != "under a minute" {
		t.Fatalf("formatDuration(45s) = %q, want under a minute", got)
	}
	if got := formatDuration(59 * time.Minute); got != "59m" {
		t.Fatalf("formatDuration(59m) = %q, want 59m", got)
	}
	if got := formatDuration(2 * time.Hour); got != "2h" {
		t.Fatalf("formatDuration(2h) = %q, want 2h", got)
	}
	if got := formatDuration(2*time.Hour + 5*time.Minute); got != "2h 5m" {
		t.Fatalf("formatDuration(2h5m) = %q, want 2h 5m", got)
	}
}
