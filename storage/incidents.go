package storage

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
)

func (s *Storage) GetActiveServiceIDs(ctx context.Context) ([]uint, error) {
	var serviceIDs []uint
	err := s.db.WithContext(ctx).
		Model(&structs.Service{}).
		Where("active = ?", true).
		Pluck("id", &serviceIDs).Error
	if err != nil {
		return nil, err
	}
	return serviceIDs, nil
}

func (s *Storage) GetRecentReportCountsForServices(ctx context.Context, serviceIDs []uint, since time.Time) (map[uint]int64, error) {
	countsByService := make(map[uint]int64, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return countsByService, nil
	}

	// Pull grouped counts in one query so the incident worker avoids N+1 calls
	var rows []struct {
		ServiceID uint
		Count     int64
	}

	err := s.db.WithContext(ctx).
		Model(&structs.UserReport{}).
		Select("service_id, COUNT(*) AS count").
		Where("service_id IN ? AND created_at > ?", serviceIDs, since).
		Group("service_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		countsByService[row.ServiceID] = row.Count
	}

	return countsByService, nil
}

func (s *Storage) GetActiveIncidentsByServiceIDs(ctx context.Context, serviceIDs []uint) (map[uint]structs.Incident, error) {
	activeByService := make(map[uint]structs.Incident, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return activeByService, nil
	}

	var incidents []structs.Incident
	err := s.db.WithContext(ctx).
		Where("service_id IN ? AND resolved_at IS NULL", serviceIDs).
		Order("started_at ASC").
		Find(&incidents).Error
	if err != nil {
		return nil, err
	}

	for _, incident := range incidents {
		// Keep the oldest active incident if duplicates ever exist
		if _, exists := activeByService[incident.ServiceID]; exists {
			continue
		}
		activeByService[incident.ServiceID] = incident
	}

	return activeByService, nil
}

func (s *Storage) OpenIncidentIfNoneActive(ctx context.Context, serviceID uint, startedAt time.Time) (bool, error) {
	created := false

	// Transaction keeps check and create in one unit of work
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		err := tx.Model(&structs.Incident{}).
			Where("service_id = ? AND resolved_at IS NULL", serviceID).
			Count(&activeCount).Error
		if err != nil {
			return err
		}

		if activeCount > 0 {
			return nil
		}

		incident := structs.Incident{
			ServiceID: serviceID,
			StartedAt: startedAt,
		}

		if err := tx.Create(&incident).Error; err != nil {
			return err
		}

		created = true
		return nil
	})
	if err != nil {
		return false, err
	}

	return created, nil
}

func (s *Storage) ResolveActiveIncident(ctx context.Context, serviceID uint, resolvedAt time.Time) (bool, error) {
	// Resolve all active rows for this service so stale duplicates get cleaned up
	result := s.db.WithContext(ctx).
		Model(&structs.Incident{}).
		Where("service_id = ? AND resolved_at IS NULL", serviceID).
		Update("resolved_at", resolvedAt)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}
