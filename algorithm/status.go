package algorithm

import (
	"math"
	"time"
)

type Status string

const (
	StatusOperational Status = "Operational"
	StatusDegraded    Status = "Degraded"
	StatusOutage      Status = "Outage"
)

const (
	// Reports are counted in this rolling window for every status check
	ReportWindow = 30 * time.Minute
	// We only look at the latest N probe results when deciding if probes look bad
	RecentProbeWindow = 5
	// Baselines are considered trustworthy only after enough weekly history
	minBaselineWeeks        = 4
	minProbeBaselineSamples = 20
	// Cold start guardrail: a brand new service needs a lot of reports to flip status
	coldStartReportThreshold = 15
	// Even with a strong z-score, don't trigger on tiny report counts
	minAbsoluteReports    = 3
	reportZScoreThreshold = 3.0
	minProbeSamples       = 3
	// Fallback probe threshold when we do not yet trust probe baselines
	probeFailRateThreshold = 0.8
	// Softer report anomaly signal used only to combine with degraded probes
	reportSupportZScoreThreshold = 1.0
)

type Signals struct {
	RecentReports int64

	// User-report baseline for the current hour-of-week bucket
	ReportBaselineMean   float64
	ReportBaselineStdDev float64
	ReportBaselineWeeks  int

	// Recent probe behavior in the rolling probe window
	RecentProbeTotal    int64
	RecentProbeFailures int64

	// Probe baseline for this hour-of-week bucket
	ProbeBaselineFailureRate float64
	ProbeBaselineSamples     int
}

func DetermineStatus(signals Signals) Status {
	if hasUserReportOutage(signals) {
		return StatusOutage
	}
	if hasProbeProblem(signals) {
		if hasReportSupportAnomaly(signals) {
			return StatusOutage
		}
		return StatusDegraded
	}
	return StatusOperational
}

func hasUserReportOutage(signals Signals) bool {
	// Cold start path: avoid false positives when we barely have history
	if signals.ReportBaselineWeeks < minBaselineWeeks {
		return signals.RecentReports >= coldStartReportThreshold
	}

	// Mature baseline path: compare current reports to normal for this time slot
	stdDev := math.Max(signals.ReportBaselineStdDev, 1.0)
	z := (float64(signals.RecentReports) - signals.ReportBaselineMean) / stdDev

	return z >= reportZScoreThreshold && signals.RecentReports >= minAbsoluteReports
}

func hasReportSupportAnomaly(signals Signals) bool {
	if signals.ReportBaselineWeeks < minBaselineWeeks {
		return false
	}

	stdDev := math.Max(signals.ReportBaselineStdDev, 1.0)
	z := (float64(signals.RecentReports) - signals.ReportBaselineMean) / stdDev

	return z >= reportSupportZScoreThreshold && float64(signals.RecentReports) > signals.ReportBaselineMean
}

func hasProbeProblem(signals Signals) bool {
	// Not enough probe data in the short window yet
	if signals.RecentProbeTotal < minProbeSamples {
		return false
	}

	failRate := float64(signals.RecentProbeFailures) / float64(signals.RecentProbeTotal)

	// Early days for probe baseline: use a strict static failure-rate threshold
	if signals.ProbeBaselineSamples < minProbeBaselineSamples {
		return failRate >= probeFailRateThreshold
	}

	// Once baseline is mature, require a meaningful jump above normal failure rate
	threshold := math.Max(0.6, signals.ProbeBaselineFailureRate+0.4)
	threshold = math.Min(threshold, 0.95)

	return failRate >= threshold
}
