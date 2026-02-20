package structs

import "time"

// gorm schema definitions

type Service struct {
	ID          uint         `json:"id"`
	Slug        string       `json:"slug" gorm:"unique"`
	Name        string       `json:"name" gorm:"not null"`
	HomepageURL string       `json:"homepage_url" gorm:"not null"`
	Category    string       `json:"category" gorm:"default:other"`
	Reports     []UserReport `json:"reports" gorm:"foreignKey:ServiceID"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type UserReport struct {
	ID          uint      `json:"id"`
	ServiceID   uint      `json:"service_id"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Timestamp   time.Time `json:"timestamp"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
