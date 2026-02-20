package storage

import (
	"context"
	"time"

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
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	result := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.slug, s.name, s.homepage_url, s.category,
		       COUNT(ur.id) AS recent_report_count
		FROM services s
		LEFT JOIN user_reports ur ON ur.service_id = s.id AND ur.timestamp > ?
		GROUP BY s.id
		ORDER BY recent_report_count DESC
		LIMIT 48
	`, tenMinutesAgo).Scan(&rows)
	return rows, result.Error
}

// SearchServices returns services filtered by name (case-insensitive substring match),
// ordered by recent report count (descending)
func (s *Storage) SearchServices(ctx context.Context, query string) ([]ServiceRow, error) {
	var rows []ServiceRow
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	result := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.slug, s.name, s.homepage_url, s.category,
		       COUNT(ur.id) AS recent_report_count
		FROM services s
		LEFT JOIN user_reports ur ON ur.service_id = s.id AND ur.timestamp > ?
		WHERE LOWER(s.name) LIKE LOWER(?)
		GROUP BY s.id
		ORDER BY recent_report_count DESC
		LIMIT 48
	`, tenMinutesAgo, "%"+query+"%").Scan(&rows)
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
		Where("service_id = ? AND timestamp > ?", serviceID, since).
		Count(&count)
	return count, result.Error
}
