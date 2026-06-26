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
	Submissions  []ServiceSubmission
	UserReports  []UserReport
	ProbeResults []ProbeResult
	// Current probe read model and bounded recent history.
	ProbeState         ServiceProbeState
	ProbeRecentResults []ProbeRecentResult
	// One baseline row per hour-of-week bucket for this service
	Baselines   []ServiceBaseline
	Incidents   []Incident
	ProbeConfig ProbeConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ServiceSubmission struct {
	ID                   uint   `gorm:"primaryKey"`
	ServiceID            *uint  `gorm:"index"`
	Name                 string `gorm:"not null"`
	Slug                 string `gorm:"not null;index"`
	Description          string `gorm:"not null"`
	Category             string `gorm:"not null;default:'other'"`
	HomepageURL          string `gorm:"not null"`
	NormalizedDomain     string `gorm:"not null;index"`
	SubmitterEmail       string `gorm:"not null;index"`
	SubmitterFingerprint string `gorm:"not null;index"`
	Status               string `gorm:"not null;index;default:'published_unverified'"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ServiceBaseline struct {
	ID        uint `gorm:"primaryKey"`
	ServiceID uint `gorm:"not null;uniqueIndex:idx_service_hour"`
	// 0..167 where 0 = Sunday 00:00 UTC
	HourOfWeek           int `gorm:"not null;uniqueIndex:idx_service_hour"`
	MeanReports          float64
	StdDevReports        float64
	SampleCount          int
	ProbeFailureRate     float64
	ProbeFailureSamples  int
	ProbeLatencyMedianMs float64
	ProbeLatencySamples  int
	CreatedAt            time.Time
	UpdatedAt            time.Time
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
	FailureType    ProbeFailureType
	ErrorMessage   string // populated on failure
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ServiceProbeState struct {
	ID                        uint `gorm:"primaryKey"`
	ServiceID                 uint `gorm:"uniqueIndex;not null"`
	LastCheckedAt             *time.Time
	LastResultSuccess         bool `gorm:"not null;default:false"`
	LastStatusCode            *int
	LastResponseTimeMs        *int
	LastResultFailureType     ProbeFailureType
	LastResultErrorMessage    string
	LastSuccessAt             *time.Time
	LastSuccessStatusCode     *int
	LastSuccessResponseTimeMs *int
	LastFailureAt             *time.Time
	LastFailureStatusCode     *int
	LastFailureResponseTimeMs *int
	LastFailureType           ProbeFailureType
	LastFailureErrorMessage   string
	RecentProbeTotal          int64 `gorm:"not null;default:0"`
	RecentProbeFailures       int64 `gorm:"not null;default:0"`
	RecentWindowUpdatedAt     *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type ProbeRecentResult struct {
	ID             uint      `gorm:"primaryKey"`
	ServiceID      uint      `gorm:"not null"`
	CheckedAt      time.Time `gorm:"not null"`
	Success        bool      `gorm:"not null"`
	StatusCode     *int
	ResponseTimeMs *int
	FailureType    ProbeFailureType
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProbeConfig struct {
	ID              uint      `gorm:"primaryKey"`
	ServiceID       uint      `gorm:"uniqueIndex;not null"`
	Enabled         bool      `gorm:"not null;default:true"`
	URL             string    `gorm:"not null"`
	Method          string    `gorm:"not null;default:'GET'"`
	IntervalSeconds int       `gorm:"not null;default:300"`
	TimeoutSeconds  int       `gorm:"not null;default:10"`
	ExpectedStatus  int       `gorm:"not null;default:200"` // which code = healthy
	NextRunAt       time.Time `gorm:"index"`
	LeaseToken      string
	LeaseExpiresAt  *time.Time `gorm:"index"`
	LastCheckedAt   *time.Time
	LastSuccessAt   *time.Time
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
