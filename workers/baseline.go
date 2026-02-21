package workers

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

const baselineRefreshInterval = time.Hour

func StartBaselineRefresher(store *storage.Storage) {
	// Keep baselines warm in the background so request handlers can just read them
	go func() {
		refresh := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			if err := store.RefreshAllBaselines(ctx, time.Now().UTC()); err != nil {
				log.Error().Err(err).Msg("Failed to refresh baselines")
				return
			}

			log.Debug().Msg("Baselines refreshed")
		}

		refresh()

		ticker := time.NewTicker(baselineRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			refresh()
		}
	}()
}
