package services

import (
	"strings"
	"testing"
)

func TestHotStatusIndexes(t *testing.T) {
	expected := []hotStatusIndex{
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
			name:      "idx_probe_results_success_cleanup_created_id",
			statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_success_cleanup_created_id ON probe_results (created_at ASC, id ASC) WHERE success = true",
		},
		{
			name:      "idx_probe_results_failure_cleanup_created_id",
			statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_failure_cleanup_created_id ON probe_results (created_at ASC, id ASC) WHERE success = false",
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
		{
			name:      "idx_service_statuses_recent_reports",
			statement: "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_service_statuses_recent_reports ON service_statuses (recent_reports DESC, service_id)",
		},
	}

	if len(hotStatusIndexes) != len(expected) {
		t.Fatalf("hotStatusIndexes has %d entries, want %d", len(hotStatusIndexes), len(expected))
	}

	seen := map[string]bool{}
	for i, got := range hotStatusIndexes {
		want := expected[i]
		if got != want {
			t.Fatalf("hotStatusIndexes[%d] = %#v, want %#v", i, got, want)
		}
		if seen[got.name] {
			t.Fatalf("duplicate index name %q", got.name)
		}
		seen[got.name] = true
		if !strings.Contains(got.statement, "CREATE INDEX CONCURRENTLY IF NOT EXISTS") {
			t.Fatalf("%s is not created concurrently and idempotently: %s", got.name, got.statement)
		}
	}
}
