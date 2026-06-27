package storage

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/algorithm"
	"gorm.io/gorm"
)

type BackfillProbeDerivedResult struct {
	ServicesScanned    int
	ServiceBatches     int
	RecentRowsInserted int64
	StateRowsUpserted  int64
}

func (s *Storage) BackfillProbeDerived(ctx context.Context, cutoff time.Time, serviceBatchSize int) (BackfillProbeDerivedResult, error) {
	cutoff = cutoff.UTC()
	if serviceBatchSize <= 0 {
		serviceBatchSize = 500
	}

	var serviceIDs []uint
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT service_id
		FROM probe_results
		WHERE created_at < ?
		ORDER BY service_id ASC
	`, cutoff).Scan(&serviceIDs).Error; err != nil {
		return BackfillProbeDerivedResult{}, err
	}

	result := BackfillProbeDerivedResult{ServicesScanned: len(serviceIDs)}
	for start := 0; start < len(serviceIDs); start += serviceBatchSize {
		end := start + serviceBatchSize
		if end > len(serviceIDs) {
			end = len(serviceIDs)
		}

		chunk := serviceIDs[start:end]
		chunkResult, err := s.backfillProbeDerivedForServices(ctx, chunk, cutoff)
		if err != nil {
			return result, err
		}

		result.ServiceBatches++
		result.RecentRowsInserted += chunkResult.RecentRowsInserted
		result.StateRowsUpserted += chunkResult.StateRowsUpserted
	}

	return result, nil
}

func (s *Storage) backfillProbeDerivedForServices(ctx context.Context, serviceIDs []uint, cutoff time.Time) (BackfillProbeDerivedResult, error) {
	result := BackfillProbeDerivedResult{}
	if len(serviceIDs) == 0 {
		return result, nil
	}

	serviceIDList := toInt64Slice(serviceIDs)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			DELETE FROM probe_recent_results
			WHERE service_id = ANY(?)
				AND checked_at < ?
		`, pq.Array(serviceIDList), cutoff).Error; err != nil {
			return err
		}

		recentInsert := tx.Exec(`
			WITH ranked AS (
				SELECT
					service_id,
					created_at AS checked_at,
					success,
					status_code,
					response_time_ms,
					CASE WHEN success THEN '' ELSE COALESCE(failure_type, '') END AS failure_type,
					error_message,
					ROW_NUMBER() OVER (
						PARTITION BY service_id
						ORDER BY created_at DESC, id DESC
					) AS recent_rank
				FROM probe_results
				WHERE service_id = ANY(?)
					AND created_at < ?
			)
			INSERT INTO probe_recent_results (
				service_id,
				checked_at,
				success,
				status_code,
				response_time_ms,
				failure_type,
				error_message,
				created_at,
				updated_at
			)
			SELECT
				service_id,
				checked_at,
				success,
				status_code,
				response_time_ms,
				failure_type,
				error_message,
				checked_at,
				checked_at
			FROM ranked
			WHERE recent_rank <= ?
			ORDER BY service_id ASC, checked_at ASC
		`, pq.Array(serviceIDList), cutoff, probeRecentResultsCap)
		if recentInsert.Error != nil {
			return recentInsert.Error
		}
		result.RecentRowsInserted = recentInsert.RowsAffected

		prune := tx.Exec(`
			DELETE FROM probe_recent_results prr
			USING (
				SELECT
					id,
					ROW_NUMBER() OVER (
						PARTITION BY service_id
						ORDER BY checked_at DESC, id DESC
					) AS recent_rank
				FROM probe_recent_results
				WHERE service_id = ANY(?)
			) ranked
			WHERE prr.id = ranked.id
				AND ranked.recent_rank > ?
		`, pq.Array(serviceIDList), probeRecentResultsCap)
		if prune.Error != nil {
			return prune.Error
		}

		stateUpsert := tx.Exec(`
			WITH raw_ranked AS (
				SELECT
					id,
					service_id,
					created_at,
					success,
					status_code,
					response_time_ms,
					CASE WHEN success THEN '' ELSE COALESCE(failure_type, '') END AS failure_type,
					error_message,
					ROW_NUMBER() OVER (
						PARTITION BY service_id
						ORDER BY created_at DESC, id DESC
					) AS latest_rank,
					ROW_NUMBER() OVER (
						PARTITION BY service_id, success
						ORDER BY created_at DESC, id DESC
					) AS outcome_rank
				FROM probe_results
				WHERE service_id = ANY(?)
					AND created_at < ?
			),
			latest AS (
				SELECT *
				FROM raw_ranked
				WHERE latest_rank = 1
			),
			latest_success AS (
				SELECT *
				FROM raw_ranked
				WHERE success = true
					AND outcome_rank = 1
			),
			latest_failure AS (
				SELECT *
				FROM raw_ranked
				WHERE success = false
					AND outcome_rank = 1
			),
			recent_ranked AS (
				SELECT
					service_id,
					checked_at,
					success,
					ROW_NUMBER() OVER (
						PARTITION BY service_id
						ORDER BY checked_at DESC, id DESC
					) AS recent_rank
				FROM probe_recent_results
				WHERE service_id = ANY(?)
			),
			recent_stats AS (
				SELECT
					service_id,
					MAX(checked_at) AS recent_window_updated_at,
					COUNT(*)::bigint AS recent_probe_total,
					COALESCE(SUM(CASE WHEN success = false THEN 1 ELSE 0 END), 0)::bigint AS recent_probe_failures
				FROM recent_ranked
				WHERE recent_rank <= ?
				GROUP BY service_id
			)
			INSERT INTO service_probe_states (
				service_id,
				last_checked_at,
				last_result_success,
				last_status_code,
				last_response_time_ms,
				last_result_failure_type,
				last_result_error_message,
				last_success_at,
				last_success_status_code,
				last_success_response_time_ms,
				last_failure_at,
				last_failure_status_code,
				last_failure_response_time_ms,
				last_failure_type,
				last_failure_error_message,
				recent_probe_total,
				recent_probe_failures,
				recent_window_updated_at,
				created_at,
				updated_at
			)
			SELECT
				l.service_id,
				l.created_at AS last_checked_at,
				l.success AS last_result_success,
				l.status_code AS last_status_code,
				l.response_time_ms AS last_response_time_ms,
				l.failure_type AS last_result_failure_type,
				l.error_message AS last_result_error_message,
				ls.created_at AS last_success_at,
				ls.status_code AS last_success_status_code,
				ls.response_time_ms AS last_success_response_time_ms,
				lf.created_at AS last_failure_at,
				lf.status_code AS last_failure_status_code,
				lf.response_time_ms AS last_failure_response_time_ms,
				COALESCE(lf.failure_type, '') AS last_failure_type,
				COALESCE(lf.error_message, '') AS last_failure_error_message,
				COALESCE(rs.recent_probe_total, 0) AS recent_probe_total,
				COALESCE(rs.recent_probe_failures, 0) AS recent_probe_failures,
				COALESCE(rs.recent_window_updated_at, l.created_at) AS recent_window_updated_at,
				NOW() AS created_at,
				NOW() AS updated_at
			FROM latest l
			LEFT JOIN latest_success ls ON ls.service_id = l.service_id
			LEFT JOIN latest_failure lf ON lf.service_id = l.service_id
			LEFT JOIN recent_stats rs ON rs.service_id = l.service_id
			ON CONFLICT (service_id) DO UPDATE SET
				last_checked_at = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_checked_at
					ELSE service_probe_states.last_checked_at
				END,
				last_result_success = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_result_success
					ELSE service_probe_states.last_result_success
				END,
				last_status_code = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_status_code
					ELSE service_probe_states.last_status_code
				END,
				last_response_time_ms = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_response_time_ms
					ELSE service_probe_states.last_response_time_ms
				END,
				last_result_failure_type = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_result_failure_type
					ELSE service_probe_states.last_result_failure_type
				END,
				last_result_error_message = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_result_error_message
					ELSE service_probe_states.last_result_error_message
				END,
				last_success_at = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_success_at
					ELSE service_probe_states.last_success_at
				END,
				last_success_status_code = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_success_status_code
					ELSE service_probe_states.last_success_status_code
				END,
				last_success_response_time_ms = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_success_response_time_ms
					ELSE service_probe_states.last_success_response_time_ms
				END,
				last_failure_at = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_failure_at
					ELSE service_probe_states.last_failure_at
				END,
				last_failure_status_code = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_failure_status_code
					ELSE service_probe_states.last_failure_status_code
				END,
				last_failure_response_time_ms = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_failure_response_time_ms
					ELSE service_probe_states.last_failure_response_time_ms
				END,
				last_failure_type = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_failure_type
					ELSE service_probe_states.last_failure_type
				END,
				last_failure_error_message = CASE
					WHEN service_probe_states.last_checked_at IS NULL
						OR service_probe_states.last_checked_at <= EXCLUDED.last_checked_at
					THEN EXCLUDED.last_failure_error_message
					ELSE service_probe_states.last_failure_error_message
				END,
				recent_probe_total = EXCLUDED.recent_probe_total,
				recent_probe_failures = EXCLUDED.recent_probe_failures,
				recent_window_updated_at = EXCLUDED.recent_window_updated_at,
				updated_at = EXCLUDED.updated_at
		`, pq.Array(serviceIDList), cutoff, pq.Array(serviceIDList), algorithm.RecentProbeWindow)
		if stateUpsert.Error != nil {
			return stateUpsert.Error
		}
		result.StateRowsUpserted = stateUpsert.RowsAffected
		return nil
	})
	if err != nil {
		return BackfillProbeDerivedResult{}, err
	}

	return result, nil
}
