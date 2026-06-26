package storage

import (
	"strings"
	"testing"
)

func TestProbeBaselineRollupQueryDoesNotScanRawProbeResults(t *testing.T) {
	query := strings.ToLower(probeBaselineRollupQuery)
	if !strings.Contains(query, "probe_hourly_rollups") {
		t.Fatalf("probe baseline query should read probe_hourly_rollups: %s", probeBaselineRollupQuery)
	}
	if strings.Contains(query, "probe_results") {
		t.Fatalf("probe baseline query must not scan probe_results: %s", probeBaselineRollupQuery)
	}
	if !strings.Contains(query, "bucket_start + interval '1 hour' <= ?") {
		t.Fatalf("probe baseline query must only include completed hourly buckets: %s", probeBaselineRollupQuery)
	}
}
