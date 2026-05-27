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
	ID                    uint                        `json:"id"`
	Slug                  string                      `json:"slug"`
	Name                  string                      `json:"name"`
	URL                   string                      `json:"url"`
	IconURL               string                      `json:"icon_url"`
	Category              string                      `json:"category"`
	Status                string                      `json:"status"`
	RecentReports         int64                       `json:"recent_reports"`
	CanReport             bool                        `json:"can_report"`
	ReportRetryAfterSec   int64                       `json:"report_retry_after_sec"`
	ReportWindowLabel     string                      `json:"report_window_label"`
	BaselineMeanReports   float64                     `json:"baseline_mean_reports"`
	WindowUsagePercent    int                         `json:"window_usage_percent"`
	UptimePercent         float64                     `json:"uptime_percent"`
	UptimeDays            []UptimeDayResponse         `json:"uptime_days"`
	OutageDayCount        int                         `json:"outage_day_count"`
	ElevatedDayCount      int                         `json:"elevated_day_count"`
	ReportBuckets         []ReportBucketResponse      `json:"report_buckets"`
	RegionalReports       []RegionalReportResponse    `json:"regional_reports"`
	IncidentTimeline      []IncidentEntryResponse     `json:"incident_timeline"`
	ProbeConfigured       bool                        `json:"probe_configured"`
	ProbeEnabled          bool                        `json:"probe_enabled"`
	ProbeRecentTotal      int                         `json:"probe_recent_total"`
	ProbeRecentSuccesses  int                         `json:"probe_recent_successes"`
	ProbeRecentFailures   int                         `json:"probe_recent_failures"`
	LastProbeCheckedLabel string                      `json:"last_probe_checked_label"`
	LastProbeSuccessLabel string                      `json:"last_probe_success_label"`
	LastProbeFailureLabel string                      `json:"last_probe_failure_label"`
	LastProbeOutcome      string                      `json:"last_probe_outcome"`
	LastProbeStatusCode   int                         `json:"last_probe_status_code"`
	LastProbeLatencyMs    int                         `json:"last_probe_latency_ms"`
	ProbeLatencyAverageMs int                         `json:"probe_latency_average_ms"`
	ProbeLatencyMaxMs     int                         `json:"probe_latency_max_ms"`
	ProbeHistory          []ProbeHistoryEntryResponse `json:"probe_history"`
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

type ProbeHistoryEntryResponse struct {
	Label          string           `json:"label"`
	Outcome        string           `json:"outcome"`
	Success        bool             `json:"success"`
	StatusCode     int              `json:"status_code"`
	ResponseTimeMs int              `json:"response_time_ms"`
	HeightPct      int              `json:"height_pct"`
	Level          string           `json:"level"`
	FailureType    ProbeFailureType `json:"failure_type"`
	ErrorMessage   string           `json:"error_message"`
}
