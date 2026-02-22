package structs

type ServiceResponse struct {
	ID            uint   `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	IconURL       string `json:"icon_url"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	RecentReports int64  `json:"recent_reports"`
}

type ServiceDetailResponse struct {
	ID                  uint                     `json:"id"`
	Slug                string                   `json:"slug"`
	Name                string                   `json:"name"`
	URL                 string                   `json:"url"`
	IconURL             string                   `json:"icon_url"`
	Category            string                   `json:"category"`
	Status              string                   `json:"status"`
	RecentReports       int64                    `json:"recent_reports"`
	CanReport           bool                     `json:"can_report"`
	ReportRetryAfterSec int64                    `json:"report_retry_after_sec"`
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
