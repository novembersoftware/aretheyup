package workers

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

const baselineRefreshInterval = time.Hour

func StartBaselineRefresher(store *storage.Storage) {
	log.Info().Dur("interval", baselineRefreshInterval).Msg("Starting baseline refresher")

	// Keep baselines warm in the background so request handlers can just read them
	go func() {
		refresh := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			start := time.Now()
			stats, err := store.RefreshAllBaselines(ctx, time.Now().UTC())
			duration := time.Since(start)
			if err != nil {
				log.Error().
					Err(err).
					Str("mode", "worker").
					Str("job", "baseline_refresh").
					Dur("duration", duration).
					Bool("success", false).
					Int("services_scanned", stats.ServicesScanned).
					Int64("baseline_rows_affected", stats.BaselineRowsAffected).
					Msg("Baseline refresh failed")
				return
			}

			log.Info().
				Str("mode", "worker").
				Str("job", "baseline_refresh").
				Dur("duration", duration).
				Bool("success", true).
				Int("services_scanned", stats.ServicesScanned).
				Int64("baseline_rows_affected", stats.BaselineRowsAffected).
				Msg("Baseline refresh completed")
		}

		refresh()

		ticker := time.NewTicker(baselineRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			refresh()
		}
	}()
}
