package storage

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/structs"
)

type ReportBucket struct {
	Start time.Time
	Count int64
}

type DailyReportCount struct {
	Day   time.Time
	Count int64
}

type RegionalReportCount struct {
	Region string
	Count  int64
}

func (s *Storage) GetReportBucketsForService(ctx context.Context, serviceID uint, since time.Time, bucketSize time.Duration) ([]ReportBucket, error) {
	bucketSeconds := int(bucketSize / time.Second)
	if bucketSeconds <= 0 {
		bucketSeconds = 1800
	}

	var rows []struct {
		BucketStart time.Time
		Count       int64
	}

	err := s.db.WithContext(ctx).Raw(`
		SELECT
			to_timestamp(FLOOR(EXTRACT(EPOCH FROM created_at) / ?) * ?) AS bucket_start,
			COUNT(*) AS count
		FROM user_reports
		WHERE service_id = ? AND created_at >= ?
		GROUP BY bucket_start
		ORDER BY bucket_start ASC
	`, bucketSeconds, bucketSeconds, serviceID, since).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	buckets := make([]ReportBucket, len(rows))
	for i, row := range rows {
		buckets[i] = ReportBucket{
			Start: row.BucketStart.UTC(),
			Count: row.Count,
		}
	}

	return buckets, nil
}

func (s *Storage) GetRecentIncidentsForService(ctx context.Context, serviceID uint, since time.Time, limit int) ([]structs.Incident, error) {
	if limit <= 0 {
		limit = 20
	}

	var incidents []structs.Incident
	err := s.db.WithContext(ctx).
		Where("service_id = ?", serviceID).
		Where("started_at >= ? OR (resolved_at IS NOT NULL AND resolved_at >= ?)", since, since).
		Order("started_at DESC").
		Limit(limit).
		Find(&incidents).Error

	return incidents, err
}

func (s *Storage) GetIncidentsOverlappingWindow(ctx context.Context, serviceID uint, windowStart, windowEnd time.Time) ([]structs.Incident, error) {
	var incidents []structs.Incident
	err := s.db.WithContext(ctx).
		Where("service_id = ?", serviceID).
		Where("started_at <= ?", windowEnd).
		Where("resolved_at IS NULL OR resolved_at >= ?", windowStart).
		Order("started_at ASC").
		Find(&incidents).Error

	return incidents, err
}

func (s *Storage) GetDailyReportCountsForService(ctx context.Context, serviceID uint, since time.Time) ([]DailyReportCount, error) {
	var rows []struct {
		Day   time.Time
		Count int64
	}

	err := s.db.WithContext(ctx).Raw(`
		SELECT DATE_TRUNC('day', created_at) AS day, COUNT(*) AS count
		FROM user_reports
		WHERE service_id = ? AND created_at >= ?
		GROUP BY day
		ORDER BY day ASC
	`, serviceID, since).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	days := make([]DailyReportCount, len(rows))
	for i, row := range rows {
		days[i] = DailyReportCount{Day: row.Day.UTC(), Count: row.Count}
	}

	return days, nil
}

func (s *Storage) GetRegionalReportCountsForService(ctx context.Context, serviceID uint, since time.Time, limit int) ([]RegionalReportCount, error) {
	if limit <= 0 {
		limit = 8
	}

	var rows []struct {
		Region string
		Count  int64
	}

	err := s.db.WithContext(ctx).Raw(`
		SELECT COALESCE(NULLIF(TRIM(region), ''), 'Unknown') AS region, COUNT(*) AS count
		FROM user_reports
		WHERE service_id = ? AND created_at >= ?
		GROUP BY region
		ORDER BY count DESC, region ASC
		LIMIT ?
	`, serviceID, since, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	counts := make([]RegionalReportCount, len(rows))
	for i, row := range rows {
		counts[i] = RegionalReportCount{Region: row.Region, Count: row.Count}
	}

	return counts, nil
}
