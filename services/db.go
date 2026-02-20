package services

import (
	"fmt"
	"math/rand"
	"time"

	_ "github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB opens a GORM connection to Postgres using the provided DSN and returns it
func NewDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	log.Info().Msg("Connected to database")
	return db, nil
}

// Migrate runs GORM AutoMigrate to create or update the schema
func MigrateDB(db *gorm.DB) error {
	err := db.AutoMigrate(
		// register schema structs here
		&structs.Service{},
		&structs.UserReport{},
		&structs.ProbeResult{},
		&structs.ProbeConfig{},
		&structs.Incident{},
	)
	if err != nil {
		return err
	}
	log.Info().Msg("Database migrated")
	return nil
}

func randomSlug(r *rand.Rand) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

// SeedDB populates the database with fake data for development
func SeedDB(db *gorm.DB, numServices int, clearDB bool) {
	if config.IsProd() {
		log.Warn().Msg("Seeding database in production is disabled")
		return
	}

	if clearDB {
		err := db.Exec("DELETE FROM user_reports; DELETE FROM services").Error
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to clear database")
		}
		log.Info().Msg("Database cleared")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	categories := []string{"social", "streaming", "cloud", "gaming", "finance", "shopping", "news", "other"}
	homepageURLs := []string{
		"https://twitter.com",
		"https://facebook.com",
		"https://instagram.com",
		"https://tiktok.com",
		"https://youtube.com",
		"https://netflix.com",
		"https://spotify.com",
		"https://discord.com",
		"https://slack.com",
		"https://github.com",
		"https://aws.amazon.com",
		"https://azure.microsoft.com",
		"https://google.com",
		"https://apple.com",
		"https://microsoft.com",
		"https://store.steampowered.com",
		"https://epicgames.com",
		"https://playstation.com",
		"https://xbox.com",
		"https://reddit.com",
	}

	for i := 0; i < numServices; i++ {
		category := categories[r.Intn(len(categories))]
		homepage := homepageURLs[r.Intn(len(homepageURLs))]

		service := structs.Service{
			Name:        fmt.Sprintf("Service %d", i+1),
			Slug:        randomSlug(r),
			HomepageURL: homepage,
			Category:    category,
		}

		if err := db.Create(&service).Error; err != nil {
			log.Error().Err(err).Msg("Failed to create service")
			continue
		}

		numReports := r.Intn(20) + 1
		var recentReportCount int
		switch i % 5 {
		case 0:
			recentReportCount = r.Intn(5) + 10
		case 1:
			recentReportCount = r.Intn(4) + 6
		default:
			recentReportCount = r.Intn(3)
		}

		for j := 0; j < recentReportCount; j++ {
			report := structs.UserReport{
				ServiceID:   service.ID,
				IPAddress:   fmt.Sprintf("%d.%d.%d.%d", r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256)),
				UserAgent:   "Mozilla/5.0 (Seed Data)",
				CreatedAt:   time.Now().Add(-time.Duration(r.Intn(600)) * time.Second),
				Fingerprint: fmt.Sprintf("fp-%d-%d", service.ID, j),
			}

			if err := db.Create(&report).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create report")
			}
		}

		remainingReports := numReports - recentReportCount
		for j := 0; j < remainingReports; j++ {
			report := structs.UserReport{
				ServiceID:   service.ID,
				IPAddress:   fmt.Sprintf("%d.%d.%d.%d", r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256)),
				UserAgent:   "Mozilla/5.0 (Seed Data)",
				CreatedAt:   time.Now().Add(-time.Duration(r.Intn(168)+10) * time.Hour),
				Fingerprint: fmt.Sprintf("fp-%d-%d-old", service.ID, j),
			}

			if err := db.Create(&report).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create report")
			}
		}
	}

	log.Info().Int("services", numServices).Msg("Database seeded")
}
