package workers

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/rs/zerolog/log"
)

const tableStatsInterval = time.Hour

func StartTableStatsLogger(store *storage.Storage) {
	log.Info().Dur("interval", tableStatsInterval).Msg("Starting table stats logger")

	go func() {
		logTableStats(store)

		ticker := time.NewTicker(tableStatsInterval)
		defer ticker.Stop()

		for range ticker.C {
			logTableStats(store)
		}
	}()
}

func logTableStats(store *storage.Storage) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	start := time.Now()
	stats, err := store.GetTableStats(ctx, storage.DefaultObservedTableNames())
	duration := time.Since(start)
	if err != nil {
		log.Error().
			Err(err).
			Str("mode", "worker").
			Str("job", "table_stats").
			Dur("duration", duration).
			Bool("success", false).
			Msg("Table stats query failed")
		return
	}

	if len(stats) == 0 {
		log.Info().
			Str("mode", "worker").
			Str("job", "table_stats").
			Dur("duration", duration).
			Bool("success", true).
			Int("tables", 0).
			Msg("Table stats completed")
		return
	}

	for _, stat := range stats {
		log.Info().
			Str("mode", "worker").
			Str("job", "table_stats").
			Dur("duration", duration).
			Bool("success", true).
			Str("table", stat.TableName).
			Int64("estimated_rows", stat.EstimatedRows).
			Int64("total_bytes", stat.TotalBytes).
			Int64("heap_bytes", stat.HeapBytes).
			Int64("index_bytes", stat.IndexBytes).
			Int64("toast_bytes", stat.ToastBytes).
			Time("last_analyze", timeOrZero(stat.LastAnalyze)).
			Time("last_autoanalyze", timeOrZero(stat.AutoAnalyze)).
			Msg("Table stats completed")
	}
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
