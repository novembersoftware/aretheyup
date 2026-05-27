package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultProbeClaimBatchSize = 16
	minProbeLeaseDuration      = time.Minute
)

var errProbeLeaseNotFound = errors.New("probe lease not found")

type ProbeHistoryRow struct {
	CheckedAt      time.Time
	Success        bool
	StatusCode     *int
	ResponseTimeMs *int
	FailureType    structs.ProbeFailureType
	ErrorMessage   string
}

type ProbeServiceDetail struct {
	HasConfig     bool
	Enabled       bool
	LastCheckedAt *time.Time
	LastSuccessAt *time.Time
	LastFailureAt *time.Time
	History       []ProbeHistoryRow
}

func DefaultProbeConfig(serviceID uint, homepageURL string, now time.Time) structs.ProbeConfig {
	return structs.ProbeConfig{
		ServiceID:       serviceID,
		Enabled:         true,
		URL:             strings.TrimSpace(homepageURL),
		Method:          "GET",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		ExpectedStatus:  200,
		NextRunAt:       now.UTC(),
	}
}

func (s *Storage) EnsureDefaultProbeConfigForService(ctx context.Context, serviceID uint, homepageURL string, now time.Time) error {
	cfg := DefaultProbeConfig(serviceID, homepageURL, now)
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "service_id"}},
		DoNothing: true,
	}).Create(&cfg).Error
}

func (s *Storage) BackfillMissingProbeConfigs(ctx context.Context, now time.Time) (int64, error) {
	var services []struct {
		ID          uint
		HomepageURL string
	}

	err := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.homepage_url
		FROM services s
		LEFT JOIN probe_configs pc ON pc.service_id = s.id
		WHERE pc.id IS NULL
		ORDER BY s.id ASC
	`).Scan(&services).Error
	if err != nil {
		return 0, err
	}

	if len(services) == 0 {
		return 0, nil
	}

	configs := make([]structs.ProbeConfig, 0, len(services))
	for _, service := range services {
		configs = append(configs, DefaultProbeConfig(service.ID, service.HomepageURL, now))
	}

	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "service_id"}},
		DoNothing: true,
	}).Create(&configs)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func (s *Storage) ClaimDueProbeConfigs(ctx context.Context, now time.Time, limit int) ([]structs.ProbeConfig, error) {
	if limit <= 0 {
		limit = defaultProbeClaimBatchSize
	}

	claimed := make([]structs.ProbeConfig, 0, limit)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var configs []structs.ProbeConfig
		err := tx.Model(&structs.ProbeConfig{}).
			Joins("JOIN services ON services.id = probe_configs.service_id").
			Where("services.active = ?", true).
			Where("probe_configs.enabled = ?", true).
			Where(
				"((probe_configs.next_run_at <= ?) OR (probe_configs.lease_expires_at IS NOT NULL AND probe_configs.lease_expires_at <= ?)) AND (probe_configs.lease_expires_at IS NULL OR probe_configs.lease_expires_at <= ?)",
				now,
				now,
				now,
			).
			Order("probe_configs.next_run_at ASC").
			Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Find(&configs).Error
		if err != nil {
			return err
		}

		for i := range configs {
			token, err := newProbeLeaseToken()
			if err != nil {
				return err
			}

			leaseExpiresAt := now.Add(probeLeaseDuration(configs[i].TimeoutSeconds))
			nextRunAt := nextProbeRunAt(configs[i].NextRunAt, now, configs[i].IntervalSeconds)
			if err := tx.Model(&structs.ProbeConfig{}).
				Where("id = ?", configs[i].ID).
				Updates(map[string]any{
					"lease_token":      token,
					"lease_expires_at": leaseExpiresAt,
					"next_run_at":      nextRunAt,
				}).Error; err != nil {
				return err
			}

			configs[i].LeaseToken = token
			configs[i].LeaseExpiresAt = &leaseExpiresAt
			configs[i].NextRunAt = nextRunAt
			claimed = append(claimed, configs[i])
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

func (s *Storage) CompleteProbeLease(ctx context.Context, configID uint, leaseToken string, result structs.ProbeResult, checkedAt time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var config structs.ProbeConfig
		if err := tx.Where("id = ? AND lease_token = ?", configID, leaseToken).First(&config).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errProbeLeaseNotFound
			}
			return err
		}

		result.ServiceID = config.ServiceID
		result.CreatedAt = checkedAt
		result.UpdatedAt = checkedAt
		if err := tx.Create(&result).Error; err != nil {
			return err
		}

		updates := map[string]any{
			"lease_token":      "",
			"lease_expires_at": nil,
			"last_checked_at":  checkedAt,
		}
		if result.Success {
			updates["last_success_at"] = checkedAt
		}

		return tx.Model(&structs.ProbeConfig{}).
			Where("id = ? AND lease_token = ?", configID, leaseToken).
			Updates(updates).Error
	})
}

func (s *Storage) DeleteProbeResultsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&structs.ProbeResult{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func (s *Storage) GetProbeServiceDetail(ctx context.Context, serviceID uint, limit int) (ProbeServiceDetail, error) {
	if limit <= 0 {
		limit = 10
	}

	var detail ProbeServiceDetail
	var config structs.ProbeConfig
	if err := s.db.WithContext(ctx).Where("service_id = ?", serviceID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return detail, nil
		}
		return detail, err
	}

	detail.HasConfig = true
	detail.Enabled = config.Enabled
	detail.LastCheckedAt = config.LastCheckedAt
	detail.LastSuccessAt = config.LastSuccessAt

	var historyRows []structs.ProbeResult
	if err := s.db.WithContext(ctx).
		Where("service_id = ?", serviceID).
		Order("created_at DESC").
		Limit(limit).
		Find(&historyRows).Error; err != nil {
		return detail, err
	}

	detail.History = make([]ProbeHistoryRow, len(historyRows))
	for i, row := range historyRows {
		detail.History[i] = ProbeHistoryRow{
			CheckedAt:      row.CreatedAt.UTC(),
			Success:        row.Success,
			StatusCode:     row.StatusCode,
			ResponseTimeMs: row.ResponseTimeMs,
			FailureType:    structs.NormalizeProbeFailureType(row.Success, row.FailureType),
			ErrorMessage:   row.ErrorMessage,
		}
	}

	var lastFailure structs.ProbeResult
	if err := s.db.WithContext(ctx).
		Where("service_id = ? AND success = ?", serviceID, false).
		Order("created_at DESC").
		Limit(1).
		First(&lastFailure).Error; err == nil {
		lastFailureAt := lastFailure.CreatedAt.UTC()
		detail.LastFailureAt = &lastFailureAt
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return detail, err
	}

	return detail, nil
}

func nextProbeRunAt(currentNextRunAt, now time.Time, intervalSeconds int) time.Time {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	if currentNextRunAt.After(now) {
		return currentNextRunAt.UTC()
	}

	return now.Add(time.Duration(intervalSeconds) * time.Second).UTC()
}

func shouldClaimProbeConfig(cfg structs.ProbeConfig, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.LeaseExpiresAt != nil && cfg.LeaseExpiresAt.After(now) {
		return false
	}
	if cfg.NextRunAt.IsZero() || !cfg.NextRunAt.After(now) {
		return true
	}

	return cfg.LeaseExpiresAt != nil && !cfg.LeaseExpiresAt.After(now)
}

func probeLeaseDuration(timeoutSeconds int) time.Duration {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}

	duration := 2 * time.Duration(timeoutSeconds) * time.Second
	if duration < minProbeLeaseDuration {
		return minProbeLeaseDuration
	}

	return duration
}

func newProbeLeaseToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf[:]), nil
}
