package storage

import (
	"context"
	"errors"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const serviceStatusUpsertBatchSize = 500

func (s *Storage) RefreshServiceStatus(ctx context.Context, serviceID uint, now time.Time) (*structs.ServiceStatus, error) {
	now = now.UTC()

	recentReports, err := s.CountRecentReports(ctx, serviceID, now.Add(-algorithm.ReportWindow))
	if err != nil {
		return nil, err
	}

	hourOfWeek := statusHourOfWeek(now)
	baseline, err := s.GetBaselineForServiceHour(ctx, serviceID, hourOfWeek)
	if err != nil {
		return nil, err
	}

	recentProbeTotal, recentProbeFailures, err := s.GetRecentProbeStats(ctx, serviceID, algorithm.RecentProbeWindow)
	if err != nil {
		return nil, err
	}

	status := buildServiceStatusSnapshot(serviceID, now, recentReports, baseline, recentProbeTotal, recentProbeFailures)
	if err := s.upsertServiceStatusSnapshots(ctx, []structs.ServiceStatus{status}); err != nil {
		return nil, err
	}

	s.invalidateServiceListCache(ctx)
	return &status, nil
}

func (s *Storage) RefreshAllServiceStatuses(ctx context.Context, now time.Time) (int, error) {
	serviceIDs, err := s.GetActiveServiceIDs(ctx)
	if err != nil {
		return 0, err
	}
	return s.RefreshServiceStatuses(ctx, serviceIDs, now)
}

func (s *Storage) RefreshServiceStatuses(ctx context.Context, serviceIDs []uint, now time.Time) (int, error) {
	if len(serviceIDs) == 0 {
		return 0, nil
	}

	now = now.UTC()
	reportCounts, err := s.GetRecentReportCountsForServices(ctx, serviceIDs, now.Add(-algorithm.ReportWindow))
	if err != nil {
		return 0, err
	}

	hourOfWeek := statusHourOfWeek(now)
	baselines, err := s.GetBaselinesForServicesHour(ctx, serviceIDs, hourOfWeek)
	if err != nil {
		return 0, err
	}

	probeStats, err := s.GetRecentProbeStatsForServices(ctx, serviceIDs, algorithm.RecentProbeWindow)
	if err != nil {
		return 0, err
	}

	statuses := make([]structs.ServiceStatus, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		var baseline *structs.ServiceBaseline
		if value, exists := baselines[serviceID]; exists {
			baseline = &value
		}

		probe := probeStats[serviceID]
		statuses = append(statuses, buildServiceStatusSnapshot(
			serviceID,
			now,
			reportCounts[serviceID],
			baseline,
			probe.RecentProbeTotal,
			probe.RecentProbeFailures,
		))
	}

	if err := s.upsertServiceStatusSnapshots(ctx, statuses); err != nil {
		return 0, err
	}

	s.invalidateServiceListCache(ctx)
	return len(statuses), nil
}

func (s *Storage) GetServiceStatus(ctx context.Context, serviceID uint) (*structs.ServiceStatus, error) {
	var status structs.ServiceStatus
	result := s.db.WithContext(ctx).Where("service_id = ?", serviceID).First(&status)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &status, nil
}

func (s *Storage) GetActiveServiceStatuses(ctx context.Context) ([]structs.ServiceStatus, error) {
	var statuses []structs.ServiceStatus
	err := s.db.WithContext(ctx).Raw(`
		SELECT ss.*
		FROM service_statuses ss
		JOIN services s ON s.id = ss.service_id
		WHERE s.active = true
		ORDER BY ss.service_id ASC
	`).Scan(&statuses).Error
	return statuses, err
}

func (s *Storage) upsertServiceStatusSnapshots(ctx context.Context, statuses []structs.ServiceStatus) error {
	if len(statuses) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "service_id"}},
		Where: clause.Where{Exprs: []clause.Expression{
			gorm.Expr("service_statuses.computed_at <= EXCLUDED.computed_at"),
		}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status",
			"recent_reports",
			"recent_probe_total",
			"recent_probe_failures",
			"baseline_mean_reports",
			"baseline_std_dev_reports",
			"baseline_sample_count",
			"probe_baseline_failure_rate",
			"probe_baseline_samples",
			"hour_of_week",
			"computed_at",
			"updated_at",
		}),
	}).CreateInBatches(&statuses, serviceStatusUpsertBatchSize).Error
}

func buildServiceStatusSnapshot(
	serviceID uint,
	now time.Time,
	recentReports int64,
	baseline *structs.ServiceBaseline,
	recentProbeTotal int64,
	recentProbeFailures int64,
) structs.ServiceStatus {
	now = now.UTC()
	signals := algorithm.Signals{
		RecentReports:       recentReports,
		RecentProbeTotal:    recentProbeTotal,
		RecentProbeFailures: recentProbeFailures,
	}

	status := structs.ServiceStatus{
		ServiceID:           serviceID,
		RecentReports:       recentReports,
		RecentProbeTotal:    recentProbeTotal,
		RecentProbeFailures: recentProbeFailures,
		HourOfWeek:          statusHourOfWeek(now),
		ComputedAt:          now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if baseline != nil {
		status.BaselineMeanReports = baseline.MeanReports
		status.BaselineStdDevReports = baseline.StdDevReports
		status.BaselineSampleCount = baseline.SampleCount
		status.ProbeBaselineFailureRate = baseline.ProbeFailureRate
		status.ProbeBaselineSamples = baseline.ProbeFailureSamples

		signals.ReportBaselineMean = baseline.MeanReports
		signals.ReportBaselineStdDev = baseline.StdDevReports
		signals.ReportBaselineWeeks = baseline.SampleCount
		signals.ProbeBaselineFailureRate = baseline.ProbeFailureRate
		signals.ProbeBaselineSamples = baseline.ProbeFailureSamples
	}

	status.Status = string(algorithm.DetermineStatus(signals))
	return status
}

func statusHourOfWeek(t time.Time) int {
	t = t.UTC()
	return int(t.Weekday())*24 + t.Hour()
}
