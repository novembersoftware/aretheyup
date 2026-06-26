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

type hotStatusIndex struct {
	name      string
	statement string
}

var hotStatusIndexes = []hotStatusIndex{
	{
		name:      "idx_probe_results_service_created_desc",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_service_created_desc ON probe_results (service_id, created_at DESC)",
	},
	{
		name:      "idx_probe_results_service_failed_created_desc",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_service_failed_created_desc ON probe_results (service_id, created_at DESC) WHERE success = false",
	},
	{
		name:      "idx_probe_results_created_at",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_created_at ON probe_results (created_at)",
	},
	{
		name:      "idx_probe_recent_results_service_checked_id_desc",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_recent_results_service_checked_id_desc ON probe_recent_results (service_id, checked_at DESC, id DESC)",
	},
	{
		name:      "idx_probe_hourly_rollups_service_hour_bucket",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_hourly_rollups_service_hour_bucket ON probe_hourly_rollups (service_id, hour_of_week, bucket_start)",
	},
	{
		name:      "idx_user_reports_service_created_desc",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_reports_service_created_desc ON user_reports (service_id, created_at DESC)",
	},
	{
		name:      "idx_incidents_active_by_service",
		statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_incidents_active_by_service ON incidents (service_id) WHERE resolved_at IS NULL",
	},
}

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
		&structs.ServiceSubmission{},
		&structs.UserReport{},
		&structs.ProbeResult{},
		&structs.ServiceProbeState{},
		&structs.ProbeRecentResult{},
		&structs.ProbeHourlyRollup{},
		&structs.ProbeConfig{},
		&structs.ServiceBaseline{},
		&structs.Incident{},
	)
	if err != nil {
		return err
	}
	if err := ensureHotStatusIndexes(db); err != nil {
		return err
	}
	log.Info().Msg("Database migrated")
	return nil
}

func ensureHotStatusIndexes(db *gorm.DB) error {
	session := db.Session(&gorm.Session{SkipDefaultTransaction: true})
	for _, index := range hotStatusIndexes {
		if err := session.Exec(index.statement).Error; err != nil {
			return fmt.Errorf("create hot status index %s: %w", index.name, err)
		}
	}
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

func intPtr(v int) *int {
	return &v
}

func timePtr(t time.Time) *time.Time {
	tt := t.UTC()
	return &tt
}

func incidentActiveAt(incidents []structs.Incident, now, at time.Time) bool {
	for _, incident := range incidents {
		end := now
		if incident.ResolvedAt != nil {
			end = incident.ResolvedAt.UTC()
		}
		if (at.Equal(incident.StartedAt.UTC()) || at.After(incident.StartedAt.UTC())) && at.Before(end.Add(time.Second)) {
			return true
		}
	}

	return false
}

func buildSeedProbeData(
	r *rand.Rand,
	serviceID uint,
	severityBand int,
	now time.Time,
	incidents []structs.Incident,
) (structs.ProbeConfig, []structs.ProbeResult) {
	const (
		intervalSeconds = 5 * 60
		timeoutSeconds  = 10
		expectedStatus  = 200
	)

	start := now.AddDate(0, 0, -28).UTC().Truncate(15 * time.Minute)
	results := make([]structs.ProbeResult, 0, int(now.Sub(start)/(15*time.Minute))+1)

	lastCheckedAt := start
	var lastSuccessAt *time.Time

	for ts := start; !ts.After(now.UTC()); ts = ts.Add(15 * time.Minute) {
		failProbability := 0.03
		slowProbability := 0.08
		baseLatencyMs := 180

		switch severityBand {
		case 0:
			failProbability = 0.06
			slowProbability = 0.18
			baseLatencyMs = 240
		case 1:
			failProbability = 0.04
			slowProbability = 0.22
			baseLatencyMs = 220
		}

		if incidentActiveAt(incidents, now, ts) {
			failProbability = 0.75
			slowProbability = 0.1
			baseLatencyMs = 650
		}

		if ts.After(now.Add(-75 * time.Minute)) {
			switch severityBand {
			case 0:
				failProbability = 0.85
				slowProbability = 0.1
				baseLatencyMs = 800
			case 1:
				failProbability = 0.15
				slowProbability = 0.8
				baseLatencyMs = 700
			}
		}

		result := structs.ProbeResult{
			ServiceID: serviceID,
			Region:    "global",
			CreatedAt: ts,
			UpdatedAt: ts,
		}

		if r.Float64() < failProbability {
			result.Success = false
			if r.Float64() < 0.35 {
				result.FailureType = structs.ProbeFailureTypeConnect
				result.ErrorMessage = "dial tcp: connection refused"
			} else {
				result.FailureType = structs.ProbeFailureTypeHTTPStatus
				result.StatusCode = intPtr(503)
				result.ResponseTimeMs = intPtr(baseLatencyMs + r.Intn(180))
				result.ErrorMessage = "unexpected status: got 503 want 200"
			}
		} else {
			latencyMs := baseLatencyMs + r.Intn(120)
			if r.Float64() < slowProbability {
				latencyMs += 350 + r.Intn(250)
			}

			result.Success = true
			result.StatusCode = intPtr(expectedStatus)
			result.ResponseTimeMs = intPtr(latencyMs)
			lastSuccessAt = timePtr(ts)
		}

		lastCheckedAt = ts
		results = append(results, result)
	}

	probeConfig := structs.ProbeConfig{
		ServiceID:       serviceID,
		Enabled:         true,
		URL:             fmt.Sprintf("https://%s/", randomSlug(r)),
		Method:          "GET",
		IntervalSeconds: intervalSeconds,
		TimeoutSeconds:  timeoutSeconds,
		ExpectedStatus:  expectedStatus,
		NextRunAt:       now.UTC().Add(time.Duration(r.Intn(intervalSeconds)) * time.Second),
		LastCheckedAt:   timePtr(lastCheckedAt),
		LastSuccessAt:   lastSuccessAt,
	}

	return probeConfig, results
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

		probeConfig, probeResults := buildSeedProbeData(r, service.ID, severityBand, now, incidents)
		probeConfig.URL = fmt.Sprintf("%s/health", service.HomepageURL)
		if err := db.Create(&probeConfig).Error; err != nil {
			log.Error().Err(err).Msg("Failed to create probe config")
		}
		if len(probeResults) > 0 {
			if err := db.CreateInBatches(&probeResults, 1000).Error; err != nil {
				log.Error().Err(err).Msg("Failed to create probe results")
			}
		}
	}

	log.Info().Int("services", numServices).Msg("Database seeded")
}
