package utils

import (
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
)

func DetermineStatus(recentReports int64, baseline *structs.ServiceBaseline, recentProbeTotal, recentProbeFailures int64) algorithm.Status {
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

func ToHourOfWeek(t time.Time) int {
	return int(t.Weekday())*24 + t.Hour()
}
