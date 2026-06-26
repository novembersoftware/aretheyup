package storage

import (
	"context"
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
)

func TestBuildServiceStatusSnapshotMatchesDetermineStatus(t *testing.T) {
	now := time.Date(2026, time.January, 4, 3, 15, 0, 0, time.UTC)
	baseline := &structs.ServiceBaseline{
		ServiceID:            7,
		HourOfWeek:           statusHourOfWeek(now),
		MeanReports:          1,
		StdDevReports:        1,
		SampleCount:          4,
		ProbeFailureRate:     0.1,
		ProbeFailureSamples:  20,
		ProbeLatencyMedianMs: 120,
		ProbeLatencySamples:  20,
	}

	tests := []struct {
		name                string
		recentReports       int64
		baseline            *structs.ServiceBaseline
		recentProbeTotal    int64
		recentProbeFailures int64
	}{
		{
			name:          "report outage",
			recentReports: 5,
			baseline:      baseline,
		},
		{
			name:                "probe degraded",
			baseline:            nil,
			recentProbeTotal:    5,
			recentProbeFailures: 4,
		},
		{
			name:          "cold start operational",
			recentReports: 3,
			baseline:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildServiceStatusSnapshot(7, now, tt.recentReports, tt.baseline, tt.recentProbeTotal, tt.recentProbeFailures)
			want := determineExpectedStatus(tt.recentReports, tt.baseline, tt.recentProbeTotal, tt.recentProbeFailures)
			if got.Status != string(want) {
				t.Fatalf("snapshot status = %q, want %q", got.Status, want)
			}
			if got.ServiceID != 7 {
				t.Fatalf("ServiceID = %d, want 7", got.ServiceID)
			}
			if got.RecentReports != tt.recentReports {
				t.Fatalf("RecentReports = %d, want %d", got.RecentReports, tt.recentReports)
			}
			if !got.ComputedAt.Equal(now) {
				t.Fatalf("ComputedAt = %s, want %s", got.ComputedAt, now)
			}
			if got.HourOfWeek != statusHourOfWeek(now) {
				t.Fatalf("HourOfWeek = %d, want %d", got.HourOfWeek, statusHourOfWeek(now))
			}
		})
	}
}

func TestUpsertServiceStatusSnapshotsDoesNotOverwriteNewerSnapshot(t *testing.T) {
	store, db := newProbeIntegrationStore(t)
	ctx := context.Background()

	service := createProbeTestService(t, db)
	newerComputedAt := time.Date(2026, time.January, 10, 12, 5, 0, 0, time.UTC)
	newer := structs.ServiceStatus{
		ServiceID:     service.ID,
		Status:        string(algorithm.StatusOutage),
		RecentReports: 99,
		HourOfWeek:    statusHourOfWeek(newerComputedAt),
		ComputedAt:    newerComputedAt,
		CreatedAt:     newerComputedAt,
		UpdatedAt:     newerComputedAt,
	}
	if err := db.Create(&newer).Error; err != nil {
		t.Fatalf("create newer status snapshot: %v", err)
	}

	olderComputedAt := newerComputedAt.Add(-time.Minute)
	older := buildServiceStatusSnapshot(service.ID, olderComputedAt, 0, nil, 0, 0)
	if err := store.upsertServiceStatusSnapshots(ctx, []structs.ServiceStatus{older}); err != nil {
		t.Fatalf("upsert older status snapshot: %v", err)
	}

	got, err := store.GetServiceStatus(ctx, service.ID)
	if err != nil {
		t.Fatalf("GetServiceStatus error = %v", err)
	}
	if got == nil {
		t.Fatal("GetServiceStatus returned nil")
	}
	if !got.ComputedAt.Equal(newerComputedAt) {
		t.Fatalf("ComputedAt = %s, want preserved newer %s", got.ComputedAt, newerComputedAt)
	}
	if got.RecentReports != newer.RecentReports || got.Status != newer.Status {
		t.Fatalf("status snapshot = %+v, want newer snapshot preserved", got)
	}
}

func determineExpectedStatus(recentReports int64, baseline *structs.ServiceBaseline, recentProbeTotal, recentProbeFailures int64) algorithm.Status {
	signals := algorithm.Signals{
		RecentReports:       recentReports,
		RecentProbeTotal:    recentProbeTotal,
		RecentProbeFailures: recentProbeFailures,
	}
	if baseline != nil {
		signals.ReportBaselineMean = baseline.MeanReports
		signals.ReportBaselineStdDev = baseline.StdDevReports
		signals.ReportBaselineWeeks = baseline.SampleCount
		signals.ProbeBaselineFailureRate = baseline.ProbeFailureRate
		signals.ProbeBaselineSamples = baseline.ProbeFailureSamples
	}
	return algorithm.DetermineStatus(signals)
}
