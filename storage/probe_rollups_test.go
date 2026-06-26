package storage

import (
	"context"
	"testing"
	"time"
)

func TestBackfillProbeHourlyRollupsRejectsPartialHourWindow(t *testing.T) {
	store := New(nil, nil)
	start := time.Date(2026, time.January, 1, 0, 30, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 1, 2, 0, 0, 0, time.UTC)

	if _, err := store.BackfillProbeHourlyRollups(context.Background(), start, end); err == nil {
		t.Fatal("BackfillProbeHourlyRollups() error = nil, want hour-boundary error for partial start")
	}

	start = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(2026, time.January, 1, 2, 30, 0, 0, time.UTC)
	if _, err := store.BackfillProbeHourlyRollups(context.Background(), start, end); err == nil {
		t.Fatal("BackfillProbeHourlyRollups() error = nil, want hour-boundary error for partial end")
	}
}

func TestBackfillProbeHourlyRollupsChunkedRejectsPartialHourChunks(t *testing.T) {
	store := New(nil, nil)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 1, 4, 0, 0, 0, time.UTC)

	if _, err := store.BackfillProbeHourlyRollupsChunked(context.Background(), start, end, 90*time.Minute); err == nil {
		t.Fatal("BackfillProbeHourlyRollupsChunked() error = nil, want whole-hour chunk error")
	}
}
