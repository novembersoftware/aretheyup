package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	HourOfWeek          int
	ProbeFailureRate    float64
	ProbeFailureSamples int
}

type ProbeStats struct {
	ServiceID           uint
	RecentProbeTotal    int64
	RecentProbeFailures int64 `gorm:"column:recent_probe_failures"`
}

func (s *Storage) RefreshAllBaselines(ctx context.Context, now time.Time) error {
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
		return err
	}

	for _, service := range services {
		if err := s.refreshServiceBaselines(ctx, service.ID, service.CreatedAt, now); err != nil {
			return fmt.Errorf("refresh baseline for service %d: %w", service.ID, err)
		}
	}

	return nil
}

func (s *Storage) refreshServiceBaselines(ctx context.Context, serviceID uint, createdAt, now time.Time) error {
	// Baselines are capped at 6 months of history and aligned to 30-minute windows
	end := floorToHalfHour(now.UTC())
	start := createdAt.UTC()
	sixMonthsAgo := end.AddDate(0, -6, 0)
	if start.Before(sixMonthsAgo) {
		start = sixMonthsAgo
	}
	start = floorToHalfHour(start)

	if start.After(end) {
		return nil
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
		return err
	}

	if len(userBuckets) == 0 {
		return nil
	}

	// Probe baseline uses the same hour-of-week bucket strategy, but based on failures
	var probeBuckets []probeBaselineBucket
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			(EXTRACT(DOW FROM created_at)::int * 24 + EXTRACT(HOUR FROM created_at)::int) AS hour_of_week,
			(SUM(CASE WHEN success = false THEN 1 ELSE 0 END)::float8 / COUNT(*)) AS probe_failure_rate,
			COUNT(*)::int AS probe_failure_samples
		FROM probe_results
		WHERE service_id = ?
			AND created_at >= ?
			AND created_at <= ?
		GROUP BY hour_of_week
	`, serviceID, start, end).Scan(&probeBuckets).Error; err != nil {
		return err
	}

	probeByHour := make(map[int]probeBaselineBucket, len(probeBuckets))
	for _, b := range probeBuckets {
		probeByHour[b.HourOfWeek] = b
	}

	rows := make([]structs.ServiceBaseline, 0, len(userBuckets))
	for _, b := range userBuckets {
		probe := probeByHour[b.HourOfWeek]
		rows = append(rows, structs.ServiceBaseline{
			ServiceID:           serviceID,
			HourOfWeek:          b.HourOfWeek,
			MeanReports:         b.MeanReports,
			StdDevReports:       b.StdDevReports,
			SampleCount:         b.SampleCount,
			ProbeFailureRate:    probe.ProbeFailureRate,
			ProbeFailureSamples: probe.ProbeFailureSamples,
		})
	}

	// Upsert keeps one row per (service, hour_of_week)
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "service_id"}, {Name: "hour_of_week"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"mean_reports",
			"std_dev_reports",
			"sample_count",
			"probe_failure_rate",
			"probe_failure_samples",
			"updated_at",
		}),
	}).Create(&rows).Error
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
	// Pull only the latest N rows for this service and aggregate in SQL
	var stat ProbeStats
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			? AS service_id,
			COUNT(*) AS recent_probe_total,
			COALESCE(SUM(CASE WHEN recent.success = false THEN 1 ELSE 0 END), 0) AS recent_probe_failures
		FROM (
			SELECT success
			FROM probe_results
			WHERE service_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		) AS recent
	`, serviceID, serviceID, limit).Scan(&stat).Error; err != nil {
		return 0, 0, err
	}

	return stat.RecentProbeTotal, stat.RecentProbeFailures, nil
}

func (s *Storage) GetRecentProbeStatsForServices(ctx context.Context, serviceIDs []uint, limit int) (map[uint]ProbeStats, error) {
	// Same as above, but batched using a window function to avoid N+1 queries
	byService := make(map[uint]ProbeStats, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return byService, nil
	}

	var stats []ProbeStats
	if err := s.db.WithContext(ctx).Raw(`
		WITH ranked AS (
			SELECT
				service_id,
				success,
				ROW_NUMBER() OVER (PARTITION BY service_id ORDER BY created_at DESC) AS rn
			FROM probe_results
			WHERE service_id IN ?
		),
		recent AS (
			SELECT service_id, success
			FROM ranked
			WHERE rn <= ?
		)
		SELECT
			service_id,
			COUNT(*) AS recent_probe_total,
			COALESCE(SUM(CASE WHEN success = false THEN 1 ELSE 0 END), 0) AS recent_probe_failures
		FROM recent
		GROUP BY service_id
	`, serviceIDs, limit).Scan(&stats).Error; err != nil {
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
