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
		&structs.ServiceBaseline{},
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

func randomTimeBetween(r *rand.Rand, start, end time.Time) time.Time {
	if !end.After(start) {
		return start
	}

	seconds := int(end.Sub(start).Seconds())
	if seconds <= 1 {
		return start
	}

	return start.Add(time.Duration(r.Intn(seconds)) * time.Second)
}

// SeedDB populates the database with fake data for development
func SeedDB(db *gorm.DB, numServices int, clearDB bool) {
	if config.IsProd() {
		log.Warn().Msg("Seeding database in production is disabled")
		return
	}

	if clearDB {
		err := db.Exec("TRUNCATE TABLE services CASCADE").Error
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to clear database")
		}
		log.Info().Msg("Database cleared")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	now := time.Now().UTC()

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
	regions := []string{"US", "CA", "GB", "DE", "BR", "AU", "IN"}

	reportSequence := 0

	for i := 0; i < numServices; i++ {
		category := categories[r.Intn(len(categories))]
		homepage := homepageURLs[r.Intn(len(homepageURLs))]

		service := structs.Service{
			Name:        fmt.Sprintf("Service %d", i+1),
			Slug:        randomSlug(r),
			HomepageURL: homepage,
			Category:    category,
			Description: fmt.Sprintf("Live status and outage reports for Service %d.", i+1),
		}

		if err := db.Create(&service).Error; err != nil {
			log.Error().Err(err).Msg("Failed to create service")
			continue
		}
		log.Info().Msgf("Created service %d/%d", i+1, numServices)

		severityBand := i % 5
		var recentReportCount int
		var historicalReportCount int
		var incidentCount int

		switch severityBand {
		case 0:
			recentReportCount = r.Intn(5) + 10
			historicalReportCount = r.Intn(200) + 220
			incidentCount = r.Intn(8) + 8
		case 1:
			recentReportCount = r.Intn(4) + 6
			historicalReportCount = r.Intn(120) + 140
			incidentCount = r.Intn(6) + 4
		default:
			recentReportCount = r.Intn(3)
			historicalReportCount = r.Intn(80) + 40
			incidentCount = r.Intn(3) + 1
		}

		reports := make([]structs.UserReport, 0, recentReportCount+historicalReportCount+(incidentCount*24))
		incidents := make([]structs.Incident, 0, incidentCount)

		// Reports in the current algorithm window (last 30 min).
		for j := 0; j < recentReportCount; j++ {
			reportSequence++
			createdAt := now.Add(-time.Duration(r.Intn(30*60)) * time.Second)
			reports = append(reports, structs.UserReport{
				ServiceID:   service.ID,
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
				Fingerprint: fmt.Sprintf("fp-%d-%d", service.ID, reportSequence),
				Region:      regions[r.Intn(len(regions))],
			})
		}

		// Historical non-incident reports over the last 120 days (excluding the recent 30-minute window).
		historicalStart := now.AddDate(0, 0, -120)
		historicalEnd := now.Add(-31 * time.Minute)
		for j := 0; j < historicalReportCount; j++ {
			reportSequence++
			createdAt := randomTimeBetween(r, historicalStart, historicalEnd)
			reports = append(reports, structs.UserReport{
				ServiceID:   service.ID,
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
				Fingerprint: fmt.Sprintf("fp-%d-%d", service.ID, reportSequence),
				Region:      regions[r.Intn(len(regions))],
			})
		}

		// Historical incidents distributed through the same 120-day period.
		cursor := historicalStart
		for j := 0; j < incidentCount; j++ {
			gapHours := r.Intn(10*24) + 12
			startedAt := cursor.Add(time.Duration(gapHours) * time.Hour)
			if startedAt.After(now.Add(-90 * time.Minute)) {
				break
			}

			durationMinutes := r.Intn(240) + 20
			resolvedAtValue := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
			if resolvedAtValue.After(now) {
				resolvedAtValue = now.Add(-30 * time.Minute)
			}

			var resolvedAt *time.Time
			if severityBand == 0 && j == incidentCount-1 && r.Intn(100) < 20 {
				resolvedAt = nil
			} else {
				resolvedCopy := resolvedAtValue
				resolvedAt = &resolvedCopy
			}

			incidents = append(incidents, structs.Incident{
				ServiceID:  service.ID,
				StartedAt:  startedAt,
				ResolvedAt: resolvedAt,
				CreatedAt:  startedAt,
				UpdatedAt:  startedAt,
			})

			// Add clustered reports around each incident window to make outage periods visible.
			spikeReports := r.Intn(18) + 8
			if severityBand == 0 {
				spikeReports += r.Intn(16)
			}
			incidentEnd := resolvedAtValue
			if resolvedAt == nil {
				incidentEnd = now
			}
			for k := 0; k < spikeReports; k++ {
				reportSequence++
				createdAt := randomTimeBetween(r, startedAt.Add(-20*time.Minute), incidentEnd.Add(45*time.Minute))
				reports = append(reports, structs.UserReport{
					ServiceID:   service.ID,
					CreatedAt:   createdAt,
					UpdatedAt:   createdAt,
					Fingerprint: fmt.Sprintf("fp-%d-%d", service.ID, reportSequence),
					Region:      regions[r.Intn(len(regions))],
				})
			}

			cursor = resolvedAtValue
		}

		if len(incidents) > 0 {
			if err := db.CreateInBatches(&incidents, 100).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create incidents")
			}
		}

		if len(reports) > 0 {
			if err := db.CreateInBatches(&reports, 500).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create reports")
			}
		}
	}

	log.Info().Int("services", numServices).Msg("Database seeded")
}
