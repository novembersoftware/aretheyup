package structs

import "time"

type Service struct {
	ID          uint      `json:"id"`
	Slug        string    `gorm:"unique" json:"slug"`
	HomepageURL string    `gorm:"not null" json:"homepage_url"`
	Category    string    `gorm:"default:other" json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
