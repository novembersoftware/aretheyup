package workers

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

const statusRefreshInterval = 30 * time.Second

func StartStatusRefresher(store *storage.Storage) {
	log.Info().Dur("interval", statusRefreshInterval).Msg("Starting status refresher")

	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		startedAt := time.Now()
		count, err := store.RefreshAllServiceStatuses(ctx, time.Now().UTC())
		if err != nil {
			log.Error().Err(err).Dur("duration", time.Since(startedAt)).Msg("Failed to refresh service statuses")
			return
		}

		log.Debug().Int("count", count).Dur("duration", time.Since(startedAt)).Msg("Service statuses refreshed")
	}

	refresh()

	go func() {
		ticker := time.NewTicker(statusRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			refresh()
		}
	}()
}
