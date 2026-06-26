package storage

import (
	"context"
	"time"
)

func (s *Storage) BackfillProbeHourlyRollups(ctx context.Context, start, end time.Time) (int64, error) {
	start = start.UTC()
	end = end.UTC()
	if !end.After(start) {
		return 0, nil
	}

	result := s.db.WithContext(ctx).Exec(`
		WITH hourly AS (
			SELECT
				service_id,
				date_trunc('hour', created_at AT TIME ZONE 'UTC') AS bucket_start_utc,
				COUNT(*)::bigint AS total_count,
				COUNT(*) FILTER (WHERE success = false)::bigint AS failure_count,
				COALESCE(SUM(response_time_ms) FILTER (WHERE success = true AND response_time_ms IS NOT NULL), 0)::bigint AS success_latency_sum_ms,
				COUNT(response_time_ms) FILTER (WHERE success = true AND response_time_ms IS NOT NULL)::bigint AS success_latency_count,
				MIN(response_time_ms) FILTER (WHERE success = true AND response_time_ms IS NOT NULL)::int AS min_latency_ms,
				MAX(response_time_ms) FILTER (WHERE success = true AND response_time_ms IS NOT NULL)::int AS max_latency_ms
			FROM probe_results
			WHERE created_at >= ?
				AND created_at < ?
			GROUP BY service_id, bucket_start_utc
		)
		INSERT INTO probe_hourly_rollups (
			service_id,
			bucket_start,
			hour_of_week,
			total_count,
			failure_count,
			success_latency_sum_ms,
			success_latency_count,
			min_latency_ms,
			max_latency_ms,
			created_at,
			updated_at
		)
		SELECT
			service_id,
			bucket_start_utc AT TIME ZONE 'UTC' AS bucket_start,
			(EXTRACT(DOW FROM bucket_start_utc)::int * 24 + EXTRACT(HOUR FROM bucket_start_utc)::int) AS hour_of_week,
			total_count,
			failure_count,
			success_latency_sum_ms,
			success_latency_count,
			min_latency_ms,
			max_latency_ms,
			NOW(),
			NOW()
		FROM hourly
		ON CONFLICT (service_id, bucket_start) DO UPDATE SET
			hour_of_week = EXCLUDED.hour_of_week,
			total_count = EXCLUDED.total_count,
			failure_count = EXCLUDED.failure_count,
			success_latency_sum_ms = EXCLUDED.success_latency_sum_ms,
			success_latency_count = EXCLUDED.success_latency_count,
			min_latency_ms = EXCLUDED.min_latency_ms,
			max_latency_ms = EXCLUDED.max_latency_ms,
			updated_at = EXCLUDED.updated_at
	`, start, end)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
