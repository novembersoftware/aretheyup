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
	GetActiveServiceStatuses(ctx context.Context) ([]structs.ServiceStatus, error)
	GetActiveIncidentsByServiceIDs(ctx context.Context, serviceIDs []uint) (map[uint]structs.Incident, error)
	OpenIncidentIfNoneActive(ctx context.Context, serviceID uint, startedAt time.Time) (bool, error)
	ResolveActiveIncident(ctx context.Context, serviceID uint, resolvedAt time.Time) (bool, error)
}

func StartIncidentTracker(store *storage.Storage) {
	log.Info().Dur("interval", incidentRefreshInterval).Msg("Starting incident tracker")

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

func reconcileIncidents(ctx context.Context, store incidentStore, now time.Time) error {
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

func statusFromSnapshot(snapshot structs.ServiceStatus) algorithm.Status {
	switch algorithm.Status(snapshot.Status) {
	case algorithm.StatusOperational, algorithm.StatusDegraded, algorithm.StatusOutage:
		return algorithm.Status(snapshot.Status)
	default:
		return algorithm.StatusOperational
	}
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
