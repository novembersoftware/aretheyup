package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

const (
	probeCleanupInterval        = time.Hour
	probeRawSuccessRetention    = 24 * time.Hour
	probeRawFailureRetention    = 14 * 24 * time.Hour
	probeCleanupBatchSize       = 5000
	probeCleanupMaxBatches      = 100
	probeCleanupVacuumThreshold = 50000
)

type probeCleanupStore interface {
	DeleteExpiredRawProbeResults(ctx context.Context, successCutoff, failureCutoff time.Time, batchSize, maxBatches int) (int64, int, error)
	VacuumAnalyzeProbeResults(ctx context.Context) error
}

func StartProbeResultCleaner(store *storage.Storage) {
	log.Info().Dur("interval", probeCleanupInterval).Msg("Starting probe result cleaner")

	go func() {
		cleanup := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			if err := cleanupOldProbeResults(ctx, store, time.Now().UTC()); err != nil {
				log.Error().Err(err).Msg("Failed to clean old probe results")
			}
		}

		cleanup()

		ticker := time.NewTicker(probeCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			cleanup()
		}
	}()
}

func cleanupOldProbeResults(ctx context.Context, store probeCleanupStore, now time.Time) error {
	now = now.UTC()
	successCutoff := now.Add(-probeRawSuccessRetention)
	failureCutoff := now.Add(-probeRawFailureRetention)

	deleted, batches, err := store.DeleteExpiredRawProbeResults(
		ctx,
		successCutoff,
		failureCutoff,
		probeCleanupBatchSize,
		probeCleanupMaxBatches,
	)
	if err != nil {
		return err
	}

	vacuumed := false
	if deleted >= probeCleanupVacuumThreshold {
		if err := store.VacuumAnalyzeProbeResults(ctx); err != nil {
			return fmt.Errorf("vacuum analyze probe_results: %w", err)
		}
		vacuumed = true
	}

	log.Info().
		Time("success_cutoff", successCutoff).
		Time("failure_cutoff", failureCutoff).
		Int("batch_size", probeCleanupBatchSize).
		Int("max_batches", probeCleanupMaxBatches).
		Int("batches", batches).
		Int64("deleted", deleted).
		Int("vacuum_threshold", probeCleanupVacuumThreshold).
		Bool("vacuumed", vacuumed).
		Msg("Cleaned expired raw probe results")

	return nil
}
