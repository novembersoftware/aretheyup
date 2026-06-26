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

type incidentStore interface {
	GetActiveServiceIDs(ctx context.Context) ([]uint, error)
	GetRecentReportCountsForServices(ctx context.Context, serviceIDs []uint, since time.Time) (map[uint]int64, error)
	GetBaselinesForServicesHour(ctx context.Context, serviceIDs []uint, hourOfWeek int) (map[uint]structs.ServiceBaseline, error)
	GetRecentProbeStatsForServices(ctx context.Context, serviceIDs []uint, limit int) (map[uint]storage.ProbeStats, error)
	GetActiveServiceStatuses(ctx context.Context) ([]structs.ServiceStatus, error)
	GetActiveIncidentsByServiceIDs(ctx context.Context, serviceIDs []uint) (map[uint]structs.Incident, error)
	OpenIncidentIfNoneActive(ctx context.Context, serviceID uint, startedAt time.Time) (bool, error)
	ResolveActiveIncident(ctx context.Context, serviceID uint, resolvedAt time.Time) (bool, error)
}

func StartIncidentTracker(store *storage.Storage, useStatusSnapshots bool) {
	log.Info().
		Dur("interval", incidentRefreshInterval).
		Bool("status_snapshot_reads", useStatusSnapshots).
		Msg("Starting incident tracker")

	// This loop turns status transitions into open and close incident records
	go func() {
		reconcile := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			now := time.Now().UTC()
			if err := reconcileIncidents(ctx, store, now, useStatusSnapshots); err != nil {
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

func reconcileIncidents(ctx context.Context, store incidentStore, now time.Time, useStatusSnapshots bool) error {
	if useStatusSnapshots {
		return reconcileIncidentsFromSnapshots(ctx, store, now)
	}
	return reconcileIncidentsFromLegacy(ctx, store, now)
}

func reconcileIncidentsFromSnapshots(ctx context.Context, store incidentStore, now time.Time) error {
	statuses, err := store.GetActiveServiceStatuses(ctx)
	if err != nil {
		return err
	}

	if len(statuses) == 0 {
		return nil
	}

	serviceIDs := make([]uint, 0, len(statuses))
	for _, status := range statuses {
		serviceIDs = append(serviceIDs, status.ServiceID)
	}

	activeIncidents, err := store.GetActiveIncidentsByServiceIDs(ctx, serviceIDs)
	if err != nil {
		return err
	}

	for _, snapshot := range statuses {
		status := statusFromSnapshot(snapshot)
		_, hasActiveIncident := activeIncidents[snapshot.ServiceID]
		shouldOpen, shouldResolve := incidentTransition(status, hasActiveIncident)

		if shouldOpen {
			opened, err := store.OpenIncidentIfNoneActive(ctx, snapshot.ServiceID, now)
			if err != nil {
				return err
			}
			if opened {
				log.Info().Uint("service_id", snapshot.ServiceID).Time("started_at", now).Msg("Opened incident")
			}
			continue
		}

		if shouldResolve {
			closed, err := store.ResolveActiveIncident(ctx, snapshot.ServiceID, now)
			if err != nil {
				return err
			}
			if closed {
				log.Info().Uint("service_id", snapshot.ServiceID).Time("resolved_at", now).Msg("Resolved incident")
			}
		}
	}

	return nil
}

func reconcileIncidentsFromLegacy(ctx context.Context, store incidentStore, now time.Time) error {
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
		shouldOpen, shouldResolve := incidentTransition(status, hasActiveIncident)

		if shouldOpen {
			opened, err := store.OpenIncidentIfNoneActive(ctx, serviceID, now)
			if err != nil {
				return err
			}
			if opened {
				log.Info().Uint("service_id", serviceID).Time("started_at", now).Msg("Opened incident")
			}
			continue
		}

		if shouldResolve {
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

func statusFromSnapshot(snapshot structs.ServiceStatus) algorithm.Status {
	switch algorithm.Status(snapshot.Status) {
	case algorithm.StatusOperational, algorithm.StatusDegraded, algorithm.StatusOutage:
		return algorithm.Status(snapshot.Status)
	default:
		return algorithm.StatusOperational
	}
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
