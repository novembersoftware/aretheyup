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

type ProbeCleanupStats struct {
	RowsDeleted int64
	Batches     int
	Vacuumed    bool
	Duration    time.Duration
}

func StartProbeResultCleaner(store *storage.Storage) {
	log.Info().Dur("interval", probeCleanupInterval).Msg("Starting probe result cleaner")

	go func() {
		cleanup := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			stats, err := cleanupOldProbeResults(ctx, store, time.Now().UTC())
			if err != nil {
				log.Error().
					Err(err).
					Str("mode", "worker").
					Str("job", "probe_result_cleanup").
					Dur("duration", stats.Duration).
					Bool("success", false).
					Int64("rows_deleted", stats.RowsDeleted).
					Int("batches", stats.Batches).
					Bool("vacuumed", stats.Vacuumed).
					Msg("Probe result cleanup failed")
				return
			}

			log.Info().
				Str("mode", "worker").
				Str("job", "probe_result_cleanup").
				Dur("duration", stats.Duration).
				Bool("success", true).
				Int64("rows_deleted", stats.RowsDeleted).
				Int("batches", stats.Batches).
				Bool("vacuumed", stats.Vacuumed).
				Msg("Probe result cleanup completed")
		}

		cleanup()

		ticker := time.NewTicker(probeCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			cleanup()
		}
	}()
}

func cleanupOldProbeResults(ctx context.Context, store probeCleanupStore, now time.Time) (ProbeCleanupStats, error) {
	start := time.Now()
	stats := ProbeCleanupStats{}

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
		stats.Duration = time.Since(start)
		return stats, err
	}
	stats.RowsDeleted = deleted
	stats.Batches = batches

	vacuumed := false
	if deleted >= probeCleanupVacuumThreshold {
		if err := store.VacuumAnalyzeProbeResults(ctx); err != nil {
			stats.Duration = time.Since(start)
			return stats, fmt.Errorf("vacuum analyze probe_results: %w", err)
		}
		vacuumed = true
	}
	stats.Vacuumed = vacuumed

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

	stats.Duration = time.Since(start)
	return stats, nil
}
