package algorithm

import "testing"

// These tests use ISP/BCC-style coverage:
//   - ISP (Input Space Partitioning): split each decision branch into partitions
//     around maturity and threshold boundaries.
//   - BCC (Base Choice Coverage): keep one representative "base" behavior and
//     vary one important dimension at a time to make failures easy to diagnose.

func TestHasUserReportOutage(t *testing.T) {
	// User-report path partitions:
	//   1) cold-start vs mature baseline
	//   2) threshold boundaries for cold-start reports
	//   3) z-score boundary + absolute report floor on mature baselines
	//   4) stddev floor behavior (max(stddev, 1.0))
	tests := []struct {
		name    string
		signals Signals
		want    bool
	}{
		{
			name: "cold start below threshold stays operational",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks - 1,
				RecentReports:        coldStartReportThreshold - 1,
				ReportBaselineStdDev: 1,
			},
			want: false,
		},
		{
			name: "cold start at threshold triggers issues",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks - 1,
				RecentReports:        coldStartReportThreshold,
				ReportBaselineStdDev: 1,
			},
			want: true,
		},
		{
			name: "exactly min baseline weeks uses mature path",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        14,
				ReportBaselineMean:   10,
				ReportBaselineStdDev: 2,
			},
			want: false,
		},
		{
			name: "mature baseline z score below threshold stays operational",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        5,
				ReportBaselineMean:   3,
				ReportBaselineStdDev: 1,
			},
			want: false,
		},
		{
			name: "mature baseline at z score threshold with enough reports triggers issues",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        minAbsoluteReports,
				ReportBaselineMean:   0,
				ReportBaselineStdDev: 1,
			},
			want: true,
		},
		{
			name: "mature baseline high z score but tiny absolute report count is ignored",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        minAbsoluteReports - 1,
				ReportBaselineMean:   -2,
				ReportBaselineStdDev: 1,
			},
			want: false,
		},
		{
			name: "stddev floor of 1.0 is applied for low-variance baselines",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        minAbsoluteReports,
				ReportBaselineMean:   0,
				ReportBaselineStdDev: 0.2,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasUserReportOutage(tt.signals)
			if got != tt.want {
				t.Fatalf("hasUserReportOutage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasReportSupportAnomaly(t *testing.T) {
	tests := []struct {
		name    string
		signals Signals
		want    bool
	}{
		{
			name: "cold start never produces support anomaly",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks - 1,
				RecentReports:        8,
				ReportBaselineMean:   3,
				ReportBaselineStdDev: 1,
			},
			want: false,
		},
		{
			name: "mature baseline below support z score stays false",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        5,
				ReportBaselineMean:   4.5,
				ReportBaselineStdDev: 1,
			},
			want: false,
		},
		{
			name: "mature baseline at support z score and above mean is true",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        4,
				ReportBaselineMean:   3,
				ReportBaselineStdDev: 1,
			},
			want: true,
		},
		{
			name: "stddev floor still applies on support anomaly path",
			signals: Signals{
				ReportBaselineWeeks:  minBaselineWeeks,
				RecentReports:        2,
				ReportBaselineMean:   1,
				ReportBaselineStdDev: 0.2,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasReportSupportAnomaly(tt.signals)
			if got != tt.want {
				t.Fatalf("hasReportSupportAnomaly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasProbeProblem(t *testing.T) {
	// Probe path partitions:
	//   1) insufficient recent samples guard
	//   2) immature baseline static threshold (0.8)
	//   3) mature baseline threshold regimes: floor (0.6), linear, cap (0.95)
	//   4) boundary behavior at each threshold
	tests := []struct {
		name    string
		signals Signals
		want    bool
	}{
		{
			name: "insufficient recent probe samples never trigger",
			signals: Signals{
				RecentProbeTotal:         minProbeSamples - 1,
				RecentProbeFailures:      minProbeSamples - 1,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.2,
			},
			want: false,
		},
		{
			name: "immature probe baseline below fallback fail rate threshold stays operational",
			signals: Signals{
				RecentProbeTotal:         5,
				RecentProbeFailures:      3,
				ProbeBaselineSamples:     minProbeBaselineSamples - 1,
				ProbeBaselineFailureRate: 0.2,
			},
			want: false,
		},
		{
			name: "immature probe baseline at fallback fail rate threshold triggers issues",
			signals: Signals{
				RecentProbeTotal:         5,
				RecentProbeFailures:      4,
				ProbeBaselineSamples:     minProbeBaselineSamples - 1,
				ProbeBaselineFailureRate: 0.2,
			},
			want: true,
		},
		{
			name: "mature baseline floor threshold 0.6 below threshold stays operational",
			signals: Signals{
				RecentProbeTotal:         5,
				RecentProbeFailures:      2,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.1,
			},
			want: false,
		},
		{
			name: "mature baseline floor threshold 0.6 at threshold triggers issues",
			signals: Signals{
				RecentProbeTotal:         5,
				RecentProbeFailures:      3,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.1,
			},
			want: true,
		},
		{
			name: "mature baseline linear threshold base plus offset below threshold stays operational",
			signals: Signals{
				RecentProbeTotal:         10,
				RecentProbeFailures:      6,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.3,
			},
			want: false,
		},
		{
			name: "mature baseline linear threshold base plus offset at threshold triggers issues",
			signals: Signals{
				RecentProbeTotal:         10,
				RecentProbeFailures:      7,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.3,
			},
			want: true,
		},
		{
			name: "mature baseline cap threshold 0.95 below threshold stays operational",
			signals: Signals{
				RecentProbeTotal:         20,
				RecentProbeFailures:      18,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.7,
			},
			want: false,
		},
		{
			name: "mature baseline cap threshold 0.95 at threshold triggers issues",
			signals: Signals{
				RecentProbeTotal:         20,
				RecentProbeFailures:      19,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.7,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasProbeProblem(tt.signals)
			if got != tt.want {
				t.Fatalf("hasProbeProblem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name    string
		signals Signals
		want    Status
	}{
		{
			name: "no strong signals returns operational",
			signals: Signals{
				ReportBaselineWeeks:      minBaselineWeeks,
				RecentReports:            1,
				ReportBaselineMean:       1,
				ReportBaselineStdDev:     1,
				RecentProbeTotal:         minProbeSamples - 1,
				RecentProbeFailures:      minProbeSamples - 1,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.2,
			},
			want: StatusOperational,
		},
		{
			name: "user outage alone triggers outage",
			signals: Signals{
				ReportBaselineWeeks:      minBaselineWeeks - 1,
				RecentReports:            coldStartReportThreshold,
				ReportBaselineStdDev:     1,
				RecentProbeTotal:         minProbeSamples - 1,
				RecentProbeFailures:      minProbeSamples - 1,
				ProbeBaselineSamples:     minProbeBaselineSamples,
				ProbeBaselineFailureRate: 0.2,
			},
			want: StatusOutage,
		},
		{
			name: "probe signal alone triggers degraded",
			signals: Signals{
				ReportBaselineWeeks:      minBaselineWeeks,
				RecentReports:            1,
				ReportBaselineMean:       1,
				ReportBaselineStdDev:     1,
				RecentProbeTotal:         5,
				RecentProbeFailures:      4,
				ProbeBaselineSamples:     minProbeBaselineSamples - 1,
				ProbeBaselineFailureRate: 0.2,
			},
			want: StatusDegraded,
		},
		{
			name: "degraded probes plus support anomaly return outage",
			signals: Signals{
				ReportBaselineWeeks:      minBaselineWeeks,
				RecentReports:            4,
				ReportBaselineMean:       2,
				ReportBaselineStdDev:     1,
				RecentProbeTotal:         5,
				RecentProbeFailures:      4,
				ProbeBaselineSamples:     minProbeBaselineSamples - 1,
				ProbeBaselineFailureRate: 0.2,
			},
			want: StatusOutage,
		},
		{
			name: "cold start small reports do not promote degraded probes to outage",
			signals: Signals{
				ReportBaselineWeeks:      minBaselineWeeks - 1,
				RecentReports:            2,
				RecentProbeTotal:         5,
				RecentProbeFailures:      4,
				ProbeBaselineSamples:     minProbeBaselineSamples - 1,
				ProbeBaselineFailureRate: 0.2,
			},
			want: StatusDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineStatus(tt.signals)
			if got != tt.want {
				t.Fatalf("DetermineStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
