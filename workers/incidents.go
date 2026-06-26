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
	log.Info().Dur("interval", incidentRefreshInterval).Msg("Starting incident tracker")

	// This loop turns status transitions into open and close incident records
	go func() {
		reconcile := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			now := time.Now().UTC()
			stats, err := reconcileIncidents(ctx, store, now)
			if err != nil {
				log.Error().
					Err(err).
					Str("mode", "worker").
					Str("job", "incident_reconciliation").
					Dur("duration", stats.Duration).
					Bool("success", false).
					Int("services_scanned", stats.ServicesScanned).
					Int("opened", stats.Opened).
					Int("resolved", stats.Resolved).
					Msg("Incident reconciliation failed")
				return
			}

			log.Info().
				Str("mode", "worker").
				Str("job", "incident_reconciliation").
				Dur("duration", stats.Duration).
				Bool("success", true).
				Int("services_scanned", stats.ServicesScanned).
				Int("opened", stats.Opened).
				Int("resolved", stats.Resolved).
				Msg("Incident reconciliation completed")
		}

		reconcile()

		ticker := time.NewTicker(incidentRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			reconcile()
		}
	}()
}

type IncidentReconcileStats struct {
	ServicesScanned int
	Opened          int
	Resolved        int
	Duration        time.Duration
}

func reconcileIncidents(ctx context.Context, store *storage.Storage, now time.Time) (IncidentReconcileStats, error) {
	start := time.Now()
	stats := IncidentReconcileStats{}

	// Active services are the only ones considered for incident transitions
	serviceIDs, err := store.GetActiveServiceIDs(ctx)
	if err != nil {
		stats.Duration = time.Since(start)
		return stats, err
	}
	stats.ServicesScanned = len(serviceIDs)

	if len(serviceIDs) == 0 {
		stats.Duration = time.Since(start)
		return stats, nil
	}

	// Gather all algorithm inputs in batches for this cycle
	reportSince := now.Add(-algorithm.ReportWindow)
	reportCounts, err := store.GetRecentReportCountsForServices(ctx, serviceIDs, reportSince)
	if err != nil {
		stats.Duration = time.Since(start)
		return stats, err
	}

	hourOfWeek := toHourOfWeek(now)
	baselines, err := store.GetBaselinesForServicesHour(ctx, serviceIDs, hourOfWeek)
	if err != nil {
		stats.Duration = time.Since(start)
		return stats, err
	}

	probeStats, err := store.GetRecentProbeStatsForServices(ctx, serviceIDs, algorithm.RecentProbeWindow)
	if err != nil {
		stats.Duration = time.Since(start)
		return stats, err
	}

	activeIncidents, err := store.GetActiveIncidentsByServiceIDs(ctx, serviceIDs)
	if err != nil {
		stats.Duration = time.Since(start)
		return stats, err
	}

	for _, serviceID := range serviceIDs {
		// Reuse the same status calculation path used by API responses
		status := determineServiceStatus(serviceID, reportCounts, baselines, probeStats)
		_, hasActiveIncident := activeIncidents[serviceID]
		shouldOpen, shouldResolve := incidentTransition(status, hasActiveIncident)

		if shouldOpen {
			opened, err := store.OpenIncidentIfNoneActive(ctx, serviceID, now)
			if err != nil {
				stats.Duration = time.Since(start)
				return stats, err
			}
			if opened {
				stats.Opened++
				log.Info().Uint("service_id", serviceID).Time("started_at", now).Msg("Opened incident")
			}
			continue
		}

		if shouldResolve {
			closed, err := store.ResolveActiveIncident(ctx, serviceID, now)
			if err != nil {
				stats.Duration = time.Since(start)
				return stats, err
			}
			if closed {
				stats.Resolved++
				log.Info().Uint("service_id", serviceID).Time("resolved_at", now).Msg("Resolved incident")
			}
		}
	}

	stats.Duration = time.Since(start)
	return stats, nil
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

func incidentTransition(status algorithm.Status, hasActiveIncident bool) (bool, bool) {
	if status == algorithm.StatusOutage && !hasActiveIncident {
		return true, false
	}
	if status != algorithm.StatusOutage && hasActiveIncident {
		return false, true
	}
	return false, false
}

func toHourOfWeek(t time.Time) int {
	// 0..167 bucket index in UTC
	return int(t.Weekday())*24 + t.Hour()
}
