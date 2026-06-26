package workers

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

const (
	probeCleanupInterval = time.Hour
	probeRawRetention    = 30 * 24 * time.Hour
)

type probeCleanupStore interface {
	DeleteProbeResultsOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type ProbeCleanupStats struct {
	RowsDeleted int64
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
					Msg("Probe result cleanup failed")
				return
			}

			log.Info().
				Str("mode", "worker").
				Str("job", "probe_result_cleanup").
				Dur("duration", stats.Duration).
				Bool("success", true).
				Int64("rows_deleted", stats.RowsDeleted).
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

	cutoff := now.Add(-probeRawRetention)
	deleted, err := store.DeleteProbeResultsOlderThan(ctx, cutoff)
	if err != nil {
		stats.Duration = time.Since(start)
		return stats, err
	}

	stats.RowsDeleted = deleted
	stats.Duration = time.Since(start)
	return stats, nil
}
