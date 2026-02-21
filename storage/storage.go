package storage

import (
	"context"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
)

// Storage is the data access layer. It holds connections to all backing stores
// and exposes methods for every data operation
type Storage struct {
	db *gorm.DB
	// redis *redis.Client
}

// New returns a Storage backed by the provided Postgres connection
func New(db *gorm.DB) *Storage {
	return &Storage{db: db}
}

// ServiceRow is the result of a services query that includes aggregated report counts
type ServiceRow struct {
	ID                uint
	Slug              string
	Name              string
	HomepageURL       string
	Category          string
	RecentReportCount int64
}

// ListServices returns all services ordered by recent report count (descending)
func (s *Storage) ListServices(ctx context.Context) ([]ServiceRow, error) {
	var rows []ServiceRow
	// Keep this in sync with the algorithm's report window.
	reportWindowStart := time.Now().Add(-algorithm.ReportWindow)
	result := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.slug, s.name, s.homepage_url, s.category,
		       COUNT(ur.id) AS recent_report_count
		FROM services s
		LEFT JOIN user_reports ur ON ur.service_id = s.id AND ur.created_at > ?
		GROUP BY s.id
		ORDER BY recent_report_count DESC
		LIMIT 48
	`, reportWindowStart).Scan(&rows)
	return rows, result.Error
}

// SearchServices returns services filtered by name (case-insensitive substring match),
// ordered by recent report count (descending)
func (s *Storage) SearchServices(ctx context.Context, query string) ([]ServiceRow, error) {
	var rows []ServiceRow
	// Same window as list/detail status checks.
	reportWindowStart := time.Now().Add(-algorithm.ReportWindow)
	result := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.slug, s.name, s.homepage_url, s.category,
		       COUNT(ur.id) AS recent_report_count
		FROM services s
		LEFT JOIN user_reports ur ON ur.service_id = s.id AND ur.created_at > ?
		WHERE LOWER(s.name) LIKE LOWER(?)
		GROUP BY s.id
		ORDER BY recent_report_count DESC
		LIMIT 48
	`, reportWindowStart, "%"+query+"%").Scan(&rows)
	return rows, result.Error
}

// GetServiceBySlug returns a single service by its slug, or an error if not found
func (s *Storage) GetServiceBySlug(ctx context.Context, slug string) (*structs.Service, error) {
	var service structs.Service
	result := s.db.WithContext(ctx).Where("slug = ?", slug).First(&service)
	if result.Error != nil {
		return nil, result.Error
	}
	return &service, nil
}

// CountRecentReports returns the number of user reports for a service submitted after since
func (s *Storage) CountRecentReports(ctx context.Context, serviceID uint, since time.Time) (int64, error) {
	var count int64
	result := s.db.WithContext(ctx).Model(&structs.UserReport{}).
		Where("service_id = ? AND created_at > ?", serviceID, since).
		Count(&count)
	return count, result.Error
}

// --- Manage TUI methods ---

// ManageServiceRow is a row returned for the manage TUI list, including probe status.
type ManageServiceRow struct {
	structs.Service
	HasProbeConfig bool
	ProbeEnabled   bool
}

// GetAllServicesForManage returns all services with probe config status, ordered by name.
func (s *Storage) GetAllServicesForManage(ctx context.Context) ([]ManageServiceRow, error) {
	var services []structs.Service
	result := s.db.WithContext(ctx).Order("name ASC").Find(&services)
	if result.Error != nil {
		return nil, result.Error
	}

	rows := make([]ManageServiceRow, len(services))
	for i, svc := range services {
		var pc structs.ProbeConfig
		err := s.db.WithContext(ctx).Where("service_id = ?", svc.ID).First(&pc).Error
		row := ManageServiceRow{Service: svc}
		if err == nil {
			row.HasProbeConfig = true
			row.ProbeEnabled = pc.Enabled
		}
		rows[i] = row
	}
	return rows, nil
}

// GetServiceByID returns a single service by its primary key.
func (s *Storage) GetServiceByID(ctx context.Context, id uint) (*structs.Service, error) {
	var service structs.Service
	result := s.db.WithContext(ctx).First(&service, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &service, nil
}

// CreateService inserts a new service record and returns the created service.
func (s *Storage) CreateService(ctx context.Context, service *structs.Service) error {
	return s.db.WithContext(ctx).Create(service).Error
}

// UpdateService saves all fields of an existing service (must have a valid ID).
func (s *Storage) UpdateService(ctx context.Context, service *structs.Service) error {
	return s.db.WithContext(ctx).Save(service).Error
}

// DeleteService removes a service by its primary key.
func (s *Storage) DeleteService(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&structs.Service{}, id).Error
}

// GetProbeConfig returns the probe config for a service, or nil if none exists.
func (s *Storage) GetProbeConfig(ctx context.Context, serviceID uint) (*structs.ProbeConfig, error) {
	var pc structs.ProbeConfig
	result := s.db.WithContext(ctx).Where("service_id = ?", serviceID).First(&pc)
	if result.Error != nil {
		return nil, result.Error
	}
	return &pc, nil
}

// UpsertProbeConfig creates or updates the probe config for a service.
func (s *Storage) UpsertProbeConfig(ctx context.Context, pc *structs.ProbeConfig) error {
	if pc.ID == 0 {
		return s.db.WithContext(ctx).Create(pc).Error
	}
	return s.db.WithContext(ctx).Save(pc).Error
}
