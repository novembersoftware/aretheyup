package storage

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
)

func TestCompleteProbeLeaseUpdatesStateAndRecentHistory(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()
	checkedAt := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	service := createProbeTestService(t, db)
	config := createLeasedProbeConfig(t, db, service.ID, "failure-token", checkedAt)

	failureStatus := 503
	failureLatency := 820
	if err := store.CompleteProbeLease(ctx, config.ID, "failure-token", structs.ProbeResult{
		Region:         "global",
		Success:        false,
		StatusCode:     &failureStatus,
		ResponseTimeMs: &failureLatency,
		FailureType:    structs.ProbeFailureTypeHTTPStatus,
		ErrorMessage:   "unexpected status: got 503 want 200",
	}, checkedAt); err != nil {
		t.Fatalf("CompleteProbeLease(failure) error = %v", err)
	}

	successAt := checkedAt.Add(5 * time.Minute)
	leaseProbeConfig(t, db, config.ID, "success-token", successAt)
	successStatus := 200
	successLatency := 180
	if err := store.CompleteProbeLease(ctx, config.ID, "success-token", structs.ProbeResult{
		Region:         "global",
		Success:        true,
		StatusCode:     &successStatus,
		ResponseTimeMs: &successLatency,
	}, successAt); err != nil {
		t.Fatalf("CompleteProbeLease(success) error = %v", err)
	}

	var rawCount int64
	if err := db.Model(&structs.ProbeResult{}).Where("service_id = ?", service.ID).Count(&rawCount).Error; err != nil {
		t.Fatalf("count raw probe results: %v", err)
	}
	if rawCount != 2 {
		t.Fatalf("raw probe result count = %d, want 2", rawCount)
	}

	var state structs.ServiceProbeState
	if err := db.Where("service_id = ?", service.ID).First(&state).Error; err != nil {
		t.Fatalf("load service probe state: %v", err)
	}
	if state.LastCheckedAt == nil || !state.LastCheckedAt.Equal(successAt) {
		t.Fatalf("LastCheckedAt = %v, want %s", state.LastCheckedAt, successAt)
	}
	if state.LastSuccessAt == nil || !state.LastSuccessAt.Equal(successAt) {
		t.Fatalf("LastSuccessAt = %v, want %s", state.LastSuccessAt, successAt)
	}
	if state.LastFailureAt == nil || !state.LastFailureAt.Equal(checkedAt) {
		t.Fatalf("LastFailureAt = %v, want preserved %s", state.LastFailureAt, checkedAt)
	}
	if !state.LastResultSuccess {
		t.Fatalf("LastResultSuccess = false, want true after success")
	}
	if state.LastFailureStatusCode == nil || *state.LastFailureStatusCode != failureStatus {
		t.Fatalf("LastFailureStatusCode = %v, want %d", state.LastFailureStatusCode, failureStatus)
	}
	if state.RecentProbeTotal != 2 || state.RecentProbeFailures != 1 {
		t.Fatalf("recent state counts = total %d failures %d, want 2/1", state.RecentProbeTotal, state.RecentProbeFailures)
	}

	total, failures, err := store.GetRecentProbeStats(ctx, service.ID, 5)
	if err != nil {
		t.Fatalf("GetRecentProbeStats error = %v", err)
	}
	if total != 2 || failures != 1 {
		t.Fatalf("GetRecentProbeStats = %d/%d, want 2/1", total, failures)
	}

	detail, err := store.GetProbeServiceDetail(ctx, service.ID, 50)
	if err != nil {
		t.Fatalf("GetProbeServiceDetail error = %v", err)
	}
	if !detail.HasConfig || !detail.Enabled {
		t.Fatalf("detail config flags = has %t enabled %t, want true/true", detail.HasConfig, detail.Enabled)
	}
	if detail.LastFailureAt == nil || !detail.LastFailureAt.Equal(checkedAt) {
		t.Fatalf("detail LastFailureAt = %v, want %s", detail.LastFailureAt, checkedAt)
	}
	if len(detail.History) != 2 {
		t.Fatalf("detail history length = %d, want 2", len(detail.History))
	}
	if !detail.History[0].Success || detail.History[0].CheckedAt != successAt {
		t.Fatalf("latest history row = %+v, want success at %s", detail.History[0], successAt)
	}
	if detail.History[1].Success || detail.History[1].CheckedAt != checkedAt {
		t.Fatalf("second history row = %+v, want failure at %s", detail.History[1], checkedAt)
	}
}

func TestCompleteProbeLeaseCreatesAndIncrementsHourlyRollup(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()
	checkedAt := time.Date(2026, time.January, 10, 12, 10, 0, 0, time.UTC)

	service := createProbeTestService(t, db)
	config := createLeasedProbeConfig(t, db, service.ID, "failure-token", checkedAt)

	failureStatus := 503
	failureLatency := 820
	if err := store.CompleteProbeLease(ctx, config.ID, "failure-token", structs.ProbeResult{
		Region:         "global",
		Success:        false,
		StatusCode:     &failureStatus,
		ResponseTimeMs: &failureLatency,
		FailureType:    structs.ProbeFailureTypeHTTPStatus,
		ErrorMessage:   "unexpected status: got 503 want 200",
	}, checkedAt); err != nil {
		t.Fatalf("CompleteProbeLease(failure) error = %v", err)
	}

	successAt := checkedAt.Add(15 * time.Minute)
	leaseProbeConfig(t, db, config.ID, "success-token", successAt)
	successStatus := 200
	successLatency := 180
	if err := store.CompleteProbeLease(ctx, config.ID, "success-token", structs.ProbeResult{
		Region:         "global",
		Success:        true,
		StatusCode:     &successStatus,
		ResponseTimeMs: &successLatency,
	}, successAt); err != nil {
		t.Fatalf("CompleteProbeLease(success) error = %v", err)
	}

	var rollup structs.ProbeHourlyRollup
	bucketStart := checkedAt.UTC().Truncate(time.Hour)
	if err := db.Where("service_id = ? AND bucket_start = ?", service.ID, bucketStart).First(&rollup).Error; err != nil {
		t.Fatalf("load hourly rollup: %v", err)
	}
	if rollup.HourOfWeek != hourOfWeek(bucketStart) {
		t.Fatalf("HourOfWeek = %d, want %d", rollup.HourOfWeek, hourOfWeek(bucketStart))
	}
	if rollup.TotalCount != 2 || rollup.FailureCount != 1 {
		t.Fatalf("rollup counts = total %d failures %d, want 2/1", rollup.TotalCount, rollup.FailureCount)
	}
	if rollup.SuccessLatencySumMs != int64(successLatency) || rollup.SuccessLatencyCount != 1 {
		t.Fatalf("rollup latency aggregate = sum %d count %d, want %d/1", rollup.SuccessLatencySumMs, rollup.SuccessLatencyCount, successLatency)
	}
	if rollup.MinLatencyMs == nil || *rollup.MinLatencyMs != successLatency {
		t.Fatalf("rollup MinLatencyMs = %v, want %d", rollup.MinLatencyMs, successLatency)
	}
	if rollup.MaxLatencyMs == nil || *rollup.MaxLatencyMs != successLatency {
		t.Fatalf("rollup MaxLatencyMs = %v, want %d", rollup.MaxLatencyMs, successLatency)
	}
}

func TestBackfillProbeHourlyRollupsFeedsBaselineRefresh(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()

	service := createProbeTestService(t, db)
	createdAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Model(&service).Updates(map[string]any{
		"created_at": createdAt,
		"updated_at": createdAt,
	}).Error; err != nil {
		t.Fatalf("set service created_at: %v", err)
	}

	sundayNoonA := time.Date(2026, time.January, 4, 12, 10, 0, 0, time.UTC)
	sundayNoonB := sundayNoonA.AddDate(0, 0, 7)
	mondayAfternoon := time.Date(2026, time.January, 5, 13, 5, 0, 0, time.UTC)
	rawRows := []structs.ProbeResult{
		{ServiceID: service.ID, Region: "global", Success: true, StatusCode: probeIntPtr(200), ResponseTimeMs: probeIntPtr(100), CreatedAt: sundayNoonA, UpdatedAt: sundayNoonA},
		{ServiceID: service.ID, Region: "global", Success: false, StatusCode: probeIntPtr(503), ResponseTimeMs: probeIntPtr(900), FailureType: structs.ProbeFailureTypeHTTPStatus, CreatedAt: sundayNoonA.Add(10 * time.Minute), UpdatedAt: sundayNoonA.Add(10 * time.Minute)},
		{ServiceID: service.ID, Region: "global", Success: true, StatusCode: probeIntPtr(200), CreatedAt: sundayNoonA.Add(20 * time.Minute), UpdatedAt: sundayNoonA.Add(20 * time.Minute)},
		{ServiceID: service.ID, Region: "global", Success: false, FailureType: structs.ProbeFailureTypeConnect, CreatedAt: sundayNoonB, UpdatedAt: sundayNoonB},
		{ServiceID: service.ID, Region: "global", Success: true, StatusCode: probeIntPtr(200), ResponseTimeMs: probeIntPtr(200), CreatedAt: sundayNoonB.Add(30 * time.Minute), UpdatedAt: sundayNoonB.Add(30 * time.Minute)},
		{ServiceID: service.ID, Region: "global", Success: true, StatusCode: probeIntPtr(200), ResponseTimeMs: probeIntPtr(300), CreatedAt: mondayAfternoon, UpdatedAt: mondayAfternoon},
	}
	if err := db.Create(&rawRows).Error; err != nil {
		t.Fatalf("create raw probe rows: %v", err)
	}

	start := createdAt
	end := time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC)
	if _, err := store.BackfillProbeHourlyRollups(ctx, start, end); err != nil {
		t.Fatalf("BackfillProbeHourlyRollups error = %v", err)
	}

	sundayNoonBucket := sundayNoonA.Truncate(time.Hour)
	if err := db.Model(&structs.ProbeHourlyRollup{}).
		Where("service_id = ? AND bucket_start = ?", service.ID, sundayNoonBucket).
		Updates(map[string]any{
			"total_count":   999,
			"failure_count": 999,
		}).Error; err != nil {
		t.Fatalf("mutate rollup before idempotence check: %v", err)
	}
	if _, err := store.BackfillProbeHourlyRollups(ctx, start, end); err != nil {
		t.Fatalf("BackfillProbeHourlyRollups second run error = %v", err)
	}

	var sundayRollups []structs.ProbeHourlyRollup
	if err := db.Where("service_id = ? AND hour_of_week = ?", service.ID, hourOfWeek(sundayNoonBucket)).
		Order("bucket_start ASC").
		Find(&sundayRollups).Error; err != nil {
		t.Fatalf("load Sunday noon rollups: %v", err)
	}
	if len(sundayRollups) != 2 {
		t.Fatalf("Sunday noon rollup count = %d, want 2 buckets across weeks", len(sundayRollups))
	}
	if sundayRollups[0].TotalCount != 3 || sundayRollups[0].FailureCount != 1 {
		t.Fatalf("first Sunday rollup counts = %d/%d, want total 3 failures 1", sundayRollups[0].TotalCount, sundayRollups[0].FailureCount)
	}
	if sundayRollups[1].TotalCount != 2 || sundayRollups[1].FailureCount != 1 {
		t.Fatalf("second Sunday rollup counts = %d/%d, want total 2 failures 1", sundayRollups[1].TotalCount, sundayRollups[1].FailureCount)
	}

	if _, err := store.refreshServiceBaselines(ctx, service.ID, createdAt, end); err != nil {
		t.Fatalf("refreshServiceBaselines error = %v", err)
	}

	baseline, err := store.GetBaselineForServiceHour(ctx, service.ID, hourOfWeek(sundayNoonBucket))
	if err != nil {
		t.Fatalf("GetBaselineForServiceHour error = %v", err)
	}
	if baseline == nil {
		t.Fatalf("missing Sunday noon baseline")
	}
	if baseline.ProbeFailureSamples != 5 {
		t.Fatalf("ProbeFailureSamples = %d, want raw fixture total 5", baseline.ProbeFailureSamples)
	}
	wantFailureRate := 2.0 / 5.0
	if math.Abs(baseline.ProbeFailureRate-wantFailureRate) > 0.000001 {
		t.Fatalf("ProbeFailureRate = %.6f, want %.6f", baseline.ProbeFailureRate, wantFailureRate)
	}
	if baseline.ProbeLatencySamples != 2 {
		t.Fatalf("ProbeLatencySamples = %d, want success latency sample count 2", baseline.ProbeLatencySamples)
	}
	if baseline.ProbeLatencyMedianMs != 0 {
		t.Fatalf("ProbeLatencyMedianMs = %.2f, want 0 after rollup switch", baseline.ProbeLatencyMedianMs)
	}
}

func TestRefreshServiceBaselinesExcludesIncompleteRollupBucket(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()

	service := createProbeTestService(t, db)
	createdAt := time.Date(2026, time.January, 10, 11, 0, 0, 0, time.UTC)
	if err := db.Model(&service).Updates(map[string]any{
		"created_at": createdAt,
		"updated_at": createdAt,
	}).Error; err != nil {
		t.Fatalf("set service created_at: %v", err)
	}

	completeBucketStart := time.Date(2026, time.January, 10, 11, 0, 0, 0, time.UTC)
	incompleteBucketStart := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	rollups := []structs.ProbeHourlyRollup{
		{
			ServiceID:    service.ID,
			BucketStart:  completeBucketStart,
			HourOfWeek:   hourOfWeek(completeBucketStart),
			TotalCount:   10,
			FailureCount: 1,
			CreatedAt:    completeBucketStart,
			UpdatedAt:    completeBucketStart,
		},
		{
			ServiceID:    service.ID,
			BucketStart:  incompleteBucketStart,
			HourOfWeek:   hourOfWeek(incompleteBucketStart),
			TotalCount:   10,
			FailureCount: 10,
			CreatedAt:    incompleteBucketStart,
			UpdatedAt:    incompleteBucketStart,
		},
	}
	if err := db.Create(&rollups).Error; err != nil {
		t.Fatalf("create rollups: %v", err)
	}

	now := time.Date(2026, time.January, 10, 12, 55, 0, 0, time.UTC)
	if _, err := store.refreshServiceBaselines(ctx, service.ID, createdAt, now); err != nil {
		t.Fatalf("refreshServiceBaselines error = %v", err)
	}

	completeBaseline, err := store.GetBaselineForServiceHour(ctx, service.ID, hourOfWeek(completeBucketStart))
	if err != nil {
		t.Fatalf("GetBaselineForServiceHour complete bucket error = %v", err)
	}
	if completeBaseline == nil || completeBaseline.ProbeFailureSamples != 10 {
		t.Fatalf("complete bucket baseline = %+v, want 10 probe samples", completeBaseline)
	}

	incompleteBaseline, err := store.GetBaselineForServiceHour(ctx, service.ID, hourOfWeek(incompleteBucketStart))
	if err != nil {
		t.Fatalf("GetBaselineForServiceHour incomplete bucket error = %v", err)
	}
	if incompleteBaseline == nil {
		t.Fatal("missing incomplete hour report baseline")
	}
	if incompleteBaseline.ProbeFailureSamples != 0 || incompleteBaseline.ProbeFailureRate != 0 {
		t.Fatalf("incomplete bucket probe baseline = samples %d rate %.4f, want zero probe signal", incompleteBaseline.ProbeFailureSamples, incompleteBaseline.ProbeFailureRate)
	}
}

func TestCompleteProbeLeaseCapsRecentHistoryAndStateWindow(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()
	start := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	service := createProbeTestService(t, db)
	config := createLeasedProbeConfig(t, db, service.ID, "token-0", start)

	for i := 0; i < probeRecentResultsCap+5; i++ {
		checkedAt := start.Add(time.Duration(i) * time.Minute)
		if i > 0 {
			leaseProbeConfig(t, db, config.ID, fmt.Sprintf("token-%d", i), checkedAt)
		}

		success := i%3 != 0
		status := 200
		failureType := structs.ProbeFailureType("")
		errMessage := ""
		if !success {
			status = 503
			failureType = structs.ProbeFailureTypeHTTPStatus
			errMessage = "unexpected status: got 503 want 200"
		}
		latency := 100 + i
		if err := store.CompleteProbeLease(ctx, config.ID, fmt.Sprintf("token-%d", i), structs.ProbeResult{
			Region:         "global",
			Success:        success,
			StatusCode:     &status,
			ResponseTimeMs: &latency,
			FailureType:    failureType,
			ErrorMessage:   errMessage,
		}, checkedAt); err != nil {
			t.Fatalf("CompleteProbeLease(%d) error = %v", i, err)
		}
	}

	var recentRows []structs.ProbeRecentResult
	if err := db.Where("service_id = ?", service.ID).
		Order("checked_at ASC, id ASC").
		Find(&recentRows).Error; err != nil {
		t.Fatalf("load recent rows: %v", err)
	}
	if len(recentRows) != probeRecentResultsCap {
		t.Fatalf("recent row count = %d, want %d", len(recentRows), probeRecentResultsCap)
	}
	wantOldest := start.Add(5 * time.Minute)
	if !recentRows[0].CheckedAt.Equal(wantOldest) {
		t.Fatalf("oldest retained checked_at = %s, want %s", recentRows[0].CheckedAt, wantOldest)
	}

	total, failures, err := store.GetRecentProbeStats(ctx, service.ID, 5)
	if err != nil {
		t.Fatalf("GetRecentProbeStats error = %v", err)
	}
	if total != 5 || failures != 2 {
		t.Fatalf("GetRecentProbeStats = %d/%d, want latest status window 5/2", total, failures)
	}

	detail, err := store.GetProbeServiceDetail(ctx, service.ID, 100)
	if err != nil {
		t.Fatalf("GetProbeServiceDetail error = %v", err)
	}
	if len(detail.History) != probeRecentResultsCap {
		t.Fatalf("detail history length = %d, want capped %d", len(detail.History), probeRecentResultsCap)
	}
	wantLatest := start.Add(time.Duration(probeRecentResultsCap+4) * time.Minute)
	if !detail.History[0].CheckedAt.Equal(wantLatest) {
		t.Fatalf("latest detail checked_at = %s, want %s", detail.History[0].CheckedAt, wantLatest)
	}
	if !detail.History[len(detail.History)-1].CheckedAt.Equal(wantOldest) {
		t.Fatalf("oldest detail checked_at = %s, want %s", detail.History[len(detail.History)-1].CheckedAt, wantOldest)
	}
}

func TestGetRecentProbeStatsForServicesReadsState(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()

	serviceA := createProbeTestService(t, db)
	serviceB := createProbeTestService(t, db)
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	states := []structs.ServiceProbeState{
		{ServiceID: serviceA.ID, RecentProbeTotal: 5, RecentProbeFailures: 1, LastCheckedAt: &now},
		{ServiceID: serviceB.ID, RecentProbeTotal: 3, RecentProbeFailures: 3, LastCheckedAt: &now},
	}
	if err := db.Create(&states).Error; err != nil {
		t.Fatalf("create states: %v", err)
	}

	stats, err := store.GetRecentProbeStatsForServices(ctx, []uint{serviceA.ID, serviceB.ID, serviceB.ID + 1000}, 5)
	if err != nil {
		t.Fatalf("GetRecentProbeStatsForServices error = %v", err)
	}
	if stats[serviceA.ID].RecentProbeTotal != 5 || stats[serviceA.ID].RecentProbeFailures != 1 {
		t.Fatalf("service A stats = %+v, want 5/1", stats[serviceA.ID])
	}
	if stats[serviceB.ID].RecentProbeTotal != 3 || stats[serviceB.ID].RecentProbeFailures != 3 {
		t.Fatalf("service B stats = %+v, want 3/3", stats[serviceB.ID])
	}
	if _, ok := stats[serviceB.ID+1000]; ok {
		t.Fatalf("unexpected stats for service without state")
	}
}

func TestGetRecentProbeStatsFallsBackToRawRowsWhenStateIsMissing(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()

	service := createProbeTestService(t, db)
	start := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	rows := []structs.ProbeResult{
		{ServiceID: service.ID, Region: "global", Success: false, FailureType: structs.ProbeFailureTypeHTTPStatus, CreatedAt: start, UpdatedAt: start},
		{ServiceID: service.ID, Region: "global", Success: true, CreatedAt: start.Add(time.Minute), UpdatedAt: start.Add(time.Minute)},
		{ServiceID: service.ID, Region: "global", Success: false, FailureType: structs.ProbeFailureTypeConnect, CreatedAt: start.Add(2 * time.Minute), UpdatedAt: start.Add(2 * time.Minute)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create raw probe rows: %v", err)
	}

	total, failures, err := store.GetRecentProbeStats(ctx, service.ID, 2)
	if err != nil {
		t.Fatalf("GetRecentProbeStats error = %v", err)
	}
	if total != 2 || failures != 1 {
		t.Fatalf("GetRecentProbeStats fallback = %d/%d, want latest 2 total and 1 failure", total, failures)
	}

	stats, err := store.GetRecentProbeStatsForServices(ctx, []uint{service.ID}, 3)
	if err != nil {
		t.Fatalf("GetRecentProbeStatsForServices error = %v", err)
	}
	if stats[service.ID].RecentProbeTotal != 3 || stats[service.ID].RecentProbeFailures != 2 {
		t.Fatalf("batch fallback stats = %+v, want 3 total and 2 failures", stats[service.ID])
	}
}

func newProbeIntegrationStore(t *testing.T) (*Storage, *gorm.DB) {
	t.Helper()

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN is not set; skipping Postgres-backed storage test")
	}

	db, err := services.NewDB(dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	schema := fmt.Sprintf("org8_test_%d", time.Now().UnixNano())
	quotedSchema := pq.QuoteIdentifier(schema)
	if err := db.Exec("CREATE SCHEMA " + quotedSchema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := db.Exec("SET search_path TO " + quotedSchema).Error; err != nil {
		t.Fatalf("set test search_path: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Exec("DROP SCHEMA IF EXISTS " + quotedSchema + " CASCADE").Error
		_ = sqlDB.Close()
	})

	if err := services.MigrateDB(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return New(db, nil), db
}

func createProbeTestService(t *testing.T, db *gorm.DB) structs.Service {
	t.Helper()

	now := time.Now().UTC()
	service := structs.Service{
		Slug:        fmt.Sprintf("service-%d", now.UnixNano()),
		Name:        "Probe Test Service",
		Category:    "test",
		HomepageURL: "https://example.com",
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}

func createLeasedProbeConfig(t *testing.T, db *gorm.DB, serviceID uint, token string, checkedAt time.Time) structs.ProbeConfig {
	t.Helper()

	config := DefaultProbeConfig(serviceID, "https://example.com", checkedAt)
	config.LeaseToken = token
	leaseExpiresAt := checkedAt.Add(time.Hour)
	config.LeaseExpiresAt = &leaseExpiresAt
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("create probe config: %v", err)
	}
	return config
}

func leaseProbeConfig(t *testing.T, db *gorm.DB, configID uint, token string, checkedAt time.Time) {
	t.Helper()

	leaseExpiresAt := checkedAt.Add(time.Hour)
	if err := db.Model(&structs.ProbeConfig{}).
		Where("id = ?", configID).
		Updates(map[string]any{
			"lease_token":      token,
			"lease_expires_at": leaseExpiresAt,
		}).Error; err != nil {
		t.Fatalf("lease probe config: %v", err)
	}
}

func probeIntPtr(v int) *int {
	return &v
}
