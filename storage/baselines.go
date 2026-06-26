package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type baselineBucket struct {
	HourOfWeek    int
	MeanReports   float64
	StdDevReports float64 `gorm:"column:std_dev_reports"`
	SampleCount   int
}

type probeBaselineBucket struct {
	HourOfWeek           int
	ProbeFailureRate     float64
	ProbeFailureSamples  int
	ProbeLatencyMedianMs float64 `gorm:"column:probe_latency_median_ms"`
	ProbeLatencySamples  int     `gorm:"column:probe_latency_samples"`
}

type ProbeStats struct {
	ServiceID           uint
	RecentProbeTotal    int64
	RecentProbeFailures int64 `gorm:"column:recent_probe_failures"`
}

const probeBaselineRollupQuery = `
	SELECT
		hour_of_week,
		CASE
			WHEN SUM(total_count) = 0 THEN 0
			ELSE (SUM(failure_count)::float8 / SUM(total_count))
		END AS probe_failure_rate,
		SUM(total_count)::int AS probe_failure_samples,
		0::float8 AS probe_latency_median_ms,
		SUM(success_latency_count)::int AS probe_latency_samples
	FROM probe_hourly_rollups
	WHERE service_id = ?
		AND bucket_start >= ?
		AND bucket_start + interval '1 hour' <= ?::timestamptz
	GROUP BY hour_of_week
`

type BaselineRefreshStats struct {
	ServicesScanned      int
	BaselineRowsAffected int64
}

func (s *Storage) RefreshAllBaselines(ctx context.Context, now time.Time) (BaselineRefreshStats, error) {
	var stats BaselineRefreshStats
	// We refresh every active service each cycle so API reads stay simple
	type serviceSeed struct {
		ID        uint
		CreatedAt time.Time
	}

	var services []serviceSeed
	if err := s.db.WithContext(ctx).
		Model(&structs.Service{}).
		Select("id, created_at").
		Where("active = ?", true).
		Find(&services).Error; err != nil {
		return stats, err
	}
	stats.ServicesScanned = len(services)

	for _, service := range services {
		rowsAffected, err := s.refreshServiceBaselines(ctx, service.ID, service.CreatedAt, now)
		if err != nil {
			return stats, fmt.Errorf("refresh baseline for service %d: %w", service.ID, err)
		}
		stats.BaselineRowsAffected += rowsAffected
	}

	return stats, nil
}

func (s *Storage) refreshServiceBaselines(ctx context.Context, serviceID uint, createdAt, now time.Time) (int64, error) {
	// Baselines are capped at 6 months of history and aligned to 30-minute windows
	end := floorToHalfHour(now.UTC())
	start := createdAt.UTC()
	sixMonthsAgo := end.AddDate(0, -6, 0)
	if start.Before(sixMonthsAgo) {
		start = sixMonthsAgo
	}
	start = floorToHalfHour(start)

	if start.After(end) {
		return 0, nil
	}

	// Build per-window report counts (including zero-report windows) and then roll them up
	// into hour-of-week buckets
	var userBuckets []baselineBucket
	if err := s.db.WithContext(ctx).Raw(`
		WITH windows AS (
			SELECT gs AS window_start
			FROM generate_series(?::timestamptz, ?::timestamptz, interval '30 minutes') AS gs
		),
		window_counts AS (
			SELECT
				w.window_start,
				COUNT(ur.id)::int AS report_count
			FROM windows w
			LEFT JOIN user_reports ur
				ON ur.service_id = ?
				AND ur.created_at >= w.window_start
				AND ur.created_at < w.window_start + interval '30 minutes'
			GROUP BY w.window_start
		)
		SELECT
			(EXTRACT(DOW FROM window_start)::int * 24 + EXTRACT(HOUR FROM window_start)::int) AS hour_of_week,
			AVG(report_count)::float8 AS mean_reports,
			COALESCE(STDDEV_POP(report_count), 0)::float8 AS std_dev_reports,
			COUNT(DISTINCT DATE_TRUNC('week', window_start))::int AS sample_count
		FROM window_counts
		GROUP BY hour_of_week
	`, start, end, serviceID).Scan(&userBuckets).Error; err != nil {
		return 0, err
	}

	if len(userBuckets) == 0 {
		return 0, nil
	}

	// Probe baseline uses the same hour-of-week bucket strategy, but only from
	// completed hourly rollup buckets that fit fully inside the baseline window.
	probeByHour := map[int]probeBaselineBucket{}
	probeRollupStart := ceilToHour(start)
	if !probeRollupStart.After(end) {
		var probeBuckets []probeBaselineBucket
		if err := s.db.WithContext(ctx).Raw(probeBaselineRollupQuery, serviceID, probeRollupStart, end).Scan(&probeBuckets).Error; err != nil {
			// Let report baselines continue even when probe storage is not ready yet
			if !isProbeDataUnavailable(err) {
				return 0, err
			}
		}
		for _, b := range probeBuckets {
			probeByHour[b.HourOfWeek] = b
		}
	}

	rows := make([]structs.ServiceBaseline, 0, len(userBuckets))
	for _, b := range userBuckets {
		probe := probeByHour[b.HourOfWeek]
		rows = append(rows, structs.ServiceBaseline{
			ServiceID:            serviceID,
			HourOfWeek:           b.HourOfWeek,
			MeanReports:          b.MeanReports,
			StdDevReports:        b.StdDevReports,
			SampleCount:          b.SampleCount,
			ProbeFailureRate:     probe.ProbeFailureRate,
			ProbeFailureSamples:  probe.ProbeFailureSamples,
			ProbeLatencyMedianMs: probe.ProbeLatencyMedianMs,
			ProbeLatencySamples:  probe.ProbeLatencySamples,
		})
	}

	// Upsert keeps one row per (service, hour_of_week)
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "service_id"}, {Name: "hour_of_week"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"mean_reports",
			"std_dev_reports",
			"sample_count",
			"probe_failure_rate",
			"probe_failure_samples",
			"probe_latency_median_ms",
			"probe_latency_samples",
			"updated_at",
		}),
	}).Create(&rows)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func (s *Storage) GetBaselineForServiceHour(ctx context.Context, serviceID uint, hourOfWeek int) (*structs.ServiceBaseline, error) {
	// Missing baseline is normal for newer services
	var baseline structs.ServiceBaseline
	result := s.db.WithContext(ctx).Where("service_id = ? AND hour_of_week = ?", serviceID, hourOfWeek).First(&baseline)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &baseline, nil
}

func (s *Storage) GetBaselinesForServicesHour(ctx context.Context, serviceIDs []uint, hourOfWeek int) (map[uint]structs.ServiceBaseline, error) {
	// Batch version for list/search pages
	byService := make(map[uint]structs.ServiceBaseline, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return byService, nil
	}

	var baselines []structs.ServiceBaseline
	if err := s.db.WithContext(ctx).
		Where("service_id IN ? AND hour_of_week = ?", serviceIDs, hourOfWeek).
		Find(&baselines).Error; err != nil {
		return nil, err
	}

	for _, baseline := range baselines {
		byService[baseline.ServiceID] = baseline
	}

	return byService, nil
}

func (s *Storage) GetRecentProbeStats(ctx context.Context, serviceID uint, limit int) (int64, int64, error) {
	var state structs.ServiceProbeState
	if err := s.db.WithContext(ctx).
		Select("service_id", "recent_probe_total", "recent_probe_failures").
		Where("service_id = ?", serviceID).
		First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.getRawRecentProbeStats(ctx, serviceID, limit)
		}
		if isProbeDataUnavailable(err) {
			return s.getRawRecentProbeStats(ctx, serviceID, limit)
		}
		return 0, 0, err
	}

	return state.RecentProbeTotal, state.RecentProbeFailures, nil
}

func (s *Storage) GetRecentProbeStatsForServices(ctx context.Context, serviceIDs []uint, limit int) (map[uint]ProbeStats, error) {
	byService := make(map[uint]ProbeStats, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return byService, nil
	}

	var states []structs.ServiceProbeState
	serviceIDList := toInt64Slice(serviceIDs)
	if err := s.db.WithContext(ctx).
		Select("service_id", "recent_probe_total", "recent_probe_failures").
		Where("service_id = ANY(?)", pq.Array(serviceIDList)).
		Find(&states).Error; err != nil {
		if isProbeDataUnavailable(err) {
			return s.getRawRecentProbeStatsForServices(ctx, serviceIDs, limit)
		}
		return nil, err
	}

	for _, state := range states {
		byService[state.ServiceID] = ProbeStats{
			ServiceID:           state.ServiceID,
			RecentProbeTotal:    state.RecentProbeTotal,
			RecentProbeFailures: state.RecentProbeFailures,
		}
	}

	missingServiceIDs := make([]uint, 0)
	for _, serviceID := range serviceIDs {
		if _, ok := byService[serviceID]; !ok {
			missingServiceIDs = append(missingServiceIDs, serviceID)
		}
	}
	if len(missingServiceIDs) > 0 {
		rawStats, err := s.getRawRecentProbeStatsForServices(ctx, missingServiceIDs, limit)
		if err != nil {
			if isProbeDataUnavailable(err) {
				return byService, nil
			}
			return nil, err
		}
		for serviceID, stats := range rawStats {
			byService[serviceID] = stats
		}
	}

	return byService, nil
}

func (s *Storage) getRawRecentProbeStats(ctx context.Context, serviceID uint, limit int) (int64, int64, error) {
	stats, err := s.getRawRecentProbeStatsForServices(ctx, []uint{serviceID}, limit)
	if err != nil {
		return 0, 0, err
	}
	stat := stats[serviceID]
	return stat.RecentProbeTotal, stat.RecentProbeFailures, nil
}

func (s *Storage) getRawRecentProbeStatsForServices(ctx context.Context, serviceIDs []uint, limit int) (map[uint]ProbeStats, error) {
	byService := make(map[uint]ProbeStats, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return byService, nil
	}
	if limit <= 0 {
		limit = algorithm.RecentProbeWindow
	}
	if limit > probeRecentResultsCap {
		limit = probeRecentResultsCap
	}

	var stats []ProbeStats
	err := s.db.WithContext(ctx).Raw(`
		WITH recent AS (
			SELECT
				service_id,
				success,
				ROW_NUMBER() OVER (
					PARTITION BY service_id
					ORDER BY created_at DESC, id DESC
				) AS recent_rank
			FROM probe_results
			WHERE service_id = ANY(?)
		)
		SELECT
			service_id,
			COUNT(*) AS recent_probe_total,
			COALESCE(SUM(CASE WHEN success = false THEN 1 ELSE 0 END), 0) AS recent_probe_failures
		FROM recent
		WHERE recent_rank <= ?
		GROUP BY service_id
	`, pq.Array(toInt64Slice(serviceIDs)), limit).Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	for _, stat := range stats {
		byService[stat.ServiceID] = stat
	}
	return byService, nil
}

func floorToHalfHour(t time.Time) time.Time {
	// Snap any timestamp to either :00 or :30
	minute := 0
	if t.Minute() >= 30 {
		minute = 30
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), minute, 0, 0, t.Location())
}

func ceilToHour(t time.Time) time.Time {
	t = t.UTC()
	truncated := t.Truncate(time.Hour)
	if t.Equal(truncated) {
		return truncated
	}
	return truncated.Add(time.Hour)
}

func toInt64Slice(in []uint) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}

func isProbeDataUnavailable(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}

	return pqErr.Code == "42P01" || pqErr.Code == "42703"
}
