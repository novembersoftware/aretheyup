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
	cutoff := now.Add(-probeRawRetention)
	deleted, err := store.DeleteProbeResultsOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}
	if deleted > 0 {
		log.Info().Int64("deleted", deleted).Time("cutoff", cutoff).Msg("Deleted expired probe results")
	}

	return nil
}
