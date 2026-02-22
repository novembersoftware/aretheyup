package structs

import "time"

// gorm schema definitions

type Service struct {
	ID           uint   `gorm:"primaryKey"`
	Slug         string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null"`
	Description  string
	Category     string `gorm:"not null;default:'other'"`
	HomepageURL  string `gorm:"not null"`
	Active       bool   `gorm:"not null;default:true"`
	UserReports  []UserReport
	ProbeResults []ProbeResult
	// One baseline row per hour-of-week bucket for this service
	Baselines   []ServiceBaseline
	Incidents   []Incident
	ProbeConfig ProbeConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ServiceBaseline struct {
	ID        uint `gorm:"primaryKey"`
	ServiceID uint `gorm:"not null;uniqueIndex:idx_service_hour"`
	// 0..167 where 0 = Sunday 00:00 UTC
	HourOfWeek          int `gorm:"not null;uniqueIndex:idx_service_hour"`
	MeanReports         float64
	StdDevReports       float64
	SampleCount         int
	ProbeFailureRate    float64
	ProbeFailureSamples int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type UserReport struct {
	ID          uint   `gorm:"primaryKey"`
	ServiceID   uint   `gorm:"not null;index"`
	Fingerprint string `gorm:"not null"`
	Region      string `gorm:"not null;default:'Unknown'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProbeResult struct {
	ID             uint   `gorm:"primaryKey"`
	ServiceID      uint   `gorm:"not null;index"`
	Region         string `gorm:"not null"` // region the probe was run from
	Success        bool   `gorm:"not null"`
	StatusCode     *int   // nil if connection failed before response
	ResponseTimeMs *int   // nil if ''
	ErrorMessage   string // populated on failure
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProbeConfig struct {
	ID              uint   `gorm:"primaryKey"`
	ServiceID       uint   `gorm:"uniqueIndex;not null"`
	Enabled         bool   `gorm:"not null;default:true"`
	URL             string `gorm:"not null"`
	Method          string `gorm:"not null;default:'GET'"`
	IntervalSeconds int    `gorm:"not null;default:60"`
	TimeoutSeconds  int    `gorm:"not null;default:10"`
	ExpectedStatus  int    `gorm:"not null;default:200"` // which code = healthy
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Incident struct {
	ID         uint       `gorm:"primaryKey"`
	ServiceID  uint       `gorm:"not null;index"`
	StartedAt  time.Time  `gorm:"not null;index"`
	ResolvedAt *time.Time // nil = currently active
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
