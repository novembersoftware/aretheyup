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
)

type dbService struct {
	*gorm.DB
}

var DB = &dbService{}

func (s *dbService) Connect() {
	db, err := gorm.Open(postgres.Open(config.C.DBDSN))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	s.DB = db
	log.Info().Msg("Connected to database")
}

func (s *dbService) Migrate() {
	err := s.AutoMigrate(&structs.Service{}, &structs.UserReport{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate database")
	}
	log.Info().Msg("Database migrated")
}

func randomSlug(r *rand.Rand) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

func (s *dbService) Seed(numServices int) {
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

		if err := s.Create(&service).Error; err != nil {
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
				Timestamp:   time.Now().Add(-time.Duration(r.Intn(600)) * time.Second),
				Fingerprint: fmt.Sprintf("fp-%d-%d", service.ID, j),
			}

			if err := s.Create(&report).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create report")
			}
		}

		remainingReports := numReports - recentReportCount
		for j := 0; j < remainingReports; j++ {
			report := structs.UserReport{
				ServiceID:   service.ID,
				IPAddress:   fmt.Sprintf("%d.%d.%d.%d", r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256)),
				UserAgent:   "Mozilla/5.0 (Seed Data)",
				Timestamp:   time.Now().Add(-time.Duration(r.Intn(168)+10) * time.Hour),
				Fingerprint: fmt.Sprintf("fp-%d-%d-old", service.ID, j),
			}

			if err := s.Create(&report).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create report")
			}
		}
	}

	log.Info().Int("services", numServices).Msg("Database seeded")
}
