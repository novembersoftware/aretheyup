package storage

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

const listCacheStatsJob = "list_cache_summary"

var defaultObservedTableNames = []string{
	"services",
	"service_submissions",
	"user_reports",
	"probe_results",
	"probe_configs",
	"service_baselines",
	"incidents",
	"service_probe_states",
	"probe_recent_results",
	"probe_hourly_rollups",
	"service_statuses",
}

// TableStat is a cheap Postgres estimate for operator-facing table growth logs.
type TableStat struct {
	TableName     string     `gorm:"column:table_name"`
	EstimatedRows int64      `gorm:"column:estimated_rows"`
	TotalBytes    int64      `gorm:"column:total_bytes"`
	HeapBytes     int64      `gorm:"column:heap_bytes"`
	IndexBytes    int64      `gorm:"column:index_bytes"`
	ToastBytes    int64      `gorm:"column:toast_bytes"`
	LastAnalyze   *time.Time `gorm:"column:last_analyze"`
	AutoAnalyze   *time.Time `gorm:"column:last_autoanalyze"`
}

func DefaultObservedTableNames() []string {
	names := make([]string, len(defaultObservedTableNames))
	copy(names, defaultObservedTableNames)
	return names
}

func (s *Storage) GetTableStats(ctx context.Context, tableNames []string) ([]TableStat, error) {
	tableNames = normalizeTableNames(tableNames)
	if len(tableNames) == 0 {
		return []TableStat{}, nil
	}

	var stats []TableStat
	err := s.db.WithContext(ctx).Raw(`
		WITH requested(table_name, ordinal) AS (
			SELECT *
			FROM unnest(?::text[]) WITH ORDINALITY AS requested(table_name, ordinal)
		)
		SELECT
			requested.table_name,
			GREATEST(COALESCE(pg_class.reltuples, 0), 0)::bigint AS estimated_rows,
			pg_total_relation_size(pg_class.oid)::bigint AS total_bytes,
			pg_relation_size(pg_class.oid)::bigint AS heap_bytes,
			(pg_indexes_size(pg_class.oid))::bigint AS index_bytes,
			(GREATEST(pg_total_relation_size(pg_class.oid) - pg_relation_size(pg_class.oid) - pg_indexes_size(pg_class.oid), 0))::bigint AS toast_bytes,
			pg_stat_user_tables.last_analyze,
			pg_stat_user_tables.last_autoanalyze
		FROM requested
		JOIN pg_class ON pg_class.relname = requested.table_name
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		LEFT JOIN pg_stat_user_tables ON pg_stat_user_tables.relid = pg_class.oid
		WHERE pg_class.relkind IN ('r', 'p')
			AND pg_namespace.nspname = ANY(current_schemas(false))
		ORDER BY requested.ordinal ASC
	`, pq.Array(tableNames)).Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *Storage) StartListCacheStatsLogger(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.LogListCacheStats("api", interval)
			}
		}
	}()
}

func (s *Storage) LogListCacheStats(mode string, duration time.Duration) {
	stats := s.ResetListCacheStats()
	reads := stats.Hit + stats.Miss + stats.ReadError + stats.DecodeError
	hitRate := 0.0
	if reads > 0 {
		hitRate = float64(stats.Hit) / float64(reads)
	}

	log.Info().
		Str("mode", mode).
		Str("job", listCacheStatsJob).
		Dur("duration", duration).
		Bool("success", true).
		Int64("hit", stats.Hit).
		Int64("miss", stats.Miss).
		Int64("bypass", stats.Bypass).
		Int64("read_error", stats.ReadError).
		Int64("decode_error", stats.DecodeError).
		Int64("write_error", stats.WriteError).
		Int64("invalidation", stats.Invalidation).
		Float64("hit_rate", hitRate).
		Msg("List cache stats")
}

func normalizeTableNames(tableNames []string) []string {
	seen := make(map[string]struct{}, len(tableNames))
	normalized := make([]string, 0, len(tableNames))
	for _, name := range tableNames {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized
}
