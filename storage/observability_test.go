package storage

import "testing"

func TestListCacheStatsSnapshotAndReset(t *testing.T) {
	store := New(nil, nil)

	store.recordListCacheHit()
	store.recordListCacheMiss()
	store.recordListCacheBypass()
	store.recordListCacheReadError()
	store.recordListCacheDecodeError()
	store.recordListCacheWriteError()
	store.recordListCacheInvalidation()

	got := store.ListCacheStatsSnapshot()
	want := ListCacheStats{
		Hit:          1,
		Miss:         1,
		Bypass:       1,
		ReadError:    1,
		DecodeError:  1,
		WriteError:   1,
		Invalidation: 1,
	}
	if got != want {
		t.Fatalf("ListCacheStatsSnapshot() = %+v, want %+v", got, want)
	}

	reset := store.ResetListCacheStats()
	if reset != want {
		t.Fatalf("ResetListCacheStats() = %+v, want %+v", reset, want)
	}

	if got := store.ListCacheStatsSnapshot(); got != (ListCacheStats{}) {
		t.Fatalf("ListCacheStatsSnapshot() after reset = %+v, want zero stats", got)
	}
}

func TestNormalizeTableNamesSkipsEmptyAndDuplicateNames(t *testing.T) {
	got := normalizeTableNames([]string{"services", "", "probe_results", "services", "service_statuses"})
	want := []string{"services", "probe_results", "service_statuses"}

	if len(got) != len(want) {
		t.Fatalf("normalizeTableNames() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeTableNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGetTableStatsSkipsEmptyTableList(t *testing.T) {
	store := New(nil, nil)

	stats, err := store.GetTableStats(t.Context(), []string{"", ""})
	if err != nil {
		t.Fatalf("GetTableStats() error = %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("GetTableStats() returned %d stats, want 0", len(stats))
	}
}

func TestDefaultObservedTableNamesIncludesRawAndOptionalDerivedTables(t *testing.T) {
	names := DefaultObservedTableNames()
	required := map[string]bool{
		"services":             false,
		"service_submissions":  false,
		"user_reports":         false,
		"probe_results":        false,
		"probe_configs":        false,
		"service_baselines":    false,
		"incidents":            false,
		"service_probe_states": false,
		"probe_recent_results": false,
		"probe_hourly_rollups": false,
		"service_statuses":     false,
	}

	for _, name := range names {
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}

	for name, seen := range required {
		if !seen {
			t.Fatalf("DefaultObservedTableNames() missing %q from %#v", name, names)
		}
	}
}
