package workers

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/rs/zerolog/log"
)

const incidentRefreshInterval = time.Minute

func StartIncidentTracker(store *storage.Storage) {
	// This loop turns status transitions into open and close incident records
	go func() {
		reconcile := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			now := time.Now().UTC()
			if err := reconcileIncidents(ctx, store, now); err != nil {
				log.Error().Err(err).Msg("Failed to reconcile incidents")
			}
		}

		reconcile()

		ticker := time.NewTicker(incidentRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			reconcile()
		}
	}()
}

func reconcileIncidents(ctx context.Context, store *storage.Storage, now time.Time) error {
	// Active services are the only ones considered for incident transitions
	serviceIDs, err := store.GetActiveServiceIDs(ctx)
	if err != nil {
		return err
	}

	if len(serviceIDs) == 0 {
		return nil
	}

	// Gather all algorithm inputs in batches for this cycle
	reportSince := now.Add(-algorithm.ReportWindow)
	reportCounts, err := store.GetRecentReportCountsForServices(ctx, serviceIDs, reportSince)
	if err != nil {
		return err
	}

	hourOfWeek := toHourOfWeek(now)
	baselines, err := store.GetBaselinesForServicesHour(ctx, serviceIDs, hourOfWeek)
	if err != nil {
		return err
	}

	probeStats, err := store.GetRecentProbeStatsForServices(ctx, serviceIDs, algorithm.RecentProbeWindow)
	if err != nil {
		return err
	}

	activeIncidents, err := store.GetActiveIncidentsByServiceIDs(ctx, serviceIDs)
	if err != nil {
		return err
	}

	for _, serviceID := range serviceIDs {
		// Reuse the same status calculation path used by API responses
		status := determineServiceStatus(serviceID, reportCounts, baselines, probeStats)
		_, hasActiveIncident := activeIncidents[serviceID]

		// Opening only happens on transition to issues when no active incident exists
		if status == algorithm.StatusIssuesDetected && !hasActiveIncident {
			opened, err := store.OpenIncidentIfNoneActive(ctx, serviceID, now)
			if err != nil {
				return err
			}
			if opened {
				log.Info().Uint("service_id", serviceID).Time("started_at", now).Msg("Opened incident")
			}
			continue
		}

		// Closing only happens when the service is back to operational
		if status == algorithm.StatusOperational && hasActiveIncident {
			closed, err := store.ResolveActiveIncident(ctx, serviceID, now)
			if err != nil {
				return err
			}
			if closed {
				log.Info().Uint("service_id", serviceID).Time("resolved_at", now).Msg("Resolved incident")
			}
		}
	}

	return nil
}

func determineServiceStatus(
	serviceID uint,
	reportCounts map[uint]int64,
	baselines map[uint]structs.ServiceBaseline,
	probeStats map[uint]storage.ProbeStats,
) algorithm.Status {
	probe := probeStats[serviceID]
	// Missing map values naturally resolve to zero which is the cold start path
	signals := algorithm.Signals{
		RecentReports:       reportCounts[serviceID],
		RecentProbeTotal:    probe.RecentProbeTotal,
		RecentProbeFailures: probe.RecentProbeFailures,
	}

	if baseline, exists := baselines[serviceID]; exists {
		signals.ReportBaselineMean = baseline.MeanReports
		signals.ReportBaselineStdDev = baseline.StdDevReports
		signals.ReportBaselineWeeks = baseline.SampleCount
		signals.ProbeBaselineFailureRate = baseline.ProbeFailureRate
		signals.ProbeBaselineSamples = baseline.ProbeFailureSamples
	}

	return algorithm.DetermineStatus(signals)
}

func toHourOfWeek(t time.Time) int {
	// 0..167 bucket index in UTC
	return int(t.Weekday())*24 + t.Hour()
}
