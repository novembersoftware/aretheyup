package storage

import (
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/structs"
)

func TestShouldClaimProbeConfig(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	activeLease := now.Add(time.Minute)

	tests := []struct {
		name string
		cfg  structs.ProbeConfig
		want bool
	}{
		{
			name: "due and unlocked config is claimable",
			cfg: structs.ProbeConfig{
				Enabled:   true,
				NextRunAt: now.Add(-time.Second),
			},
			want: true,
		},
		{
			name: "disabled config is never claimable",
			cfg: structs.ProbeConfig{
				Enabled:   false,
				NextRunAt: now.Add(-time.Hour),
			},
			want: false,
		},
		{
			name: "future config with active lease is not claimable",
			cfg: structs.ProbeConfig{
				Enabled:        true,
				NextRunAt:      now.Add(5 * time.Minute),
				LeaseExpiresAt: &activeLease,
			},
			want: false,
		},
		{
			name: "expired lease is reclaimable even when next run is already in the future",
			cfg: structs.ProbeConfig{
				Enabled:        true,
				NextRunAt:      now.Add(5 * time.Minute),
				LeaseExpiresAt: &expired,
			},
			want: true,
		},
		{
			name: "zero next run is claimable for backfilled configs",
			cfg: structs.ProbeConfig{
				Enabled: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldClaimProbeConfig(tt.cfg, now); got != tt.want {
				t.Fatalf("shouldClaimProbeConfig() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestDefaultProbeConfig(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	cfg := DefaultProbeConfig(42, " https://example.com ", now)

	if cfg.ServiceID != 42 {
		t.Fatalf("ServiceID = %d, want 42", cfg.ServiceID)
	}
	if cfg.URL != "https://example.com" {
		t.Fatalf("URL = %q, want trimmed homepage URL", cfg.URL)
	}
	if !cfg.Enabled || cfg.Method != "GET" || cfg.IntervalSeconds != GlobalProbeIntervalSeconds || cfg.TimeoutSeconds != 10 || cfg.ExpectedStatus != 200 {
		t.Fatalf("unexpected default config values: %+v", cfg)
	}
	if want := initialProbeRunAt(42, now); !cfg.NextRunAt.Equal(want) {
		t.Fatalf("NextRunAt = %s, want jittered %s", cfg.NextRunAt, want)
	}
}

func TestDefaultProbeConfigJittersInitialRunsAcrossInterval(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	seenOffsets := make(map[time.Duration]bool)

	for serviceID := uint(1); serviceID <= 100; serviceID++ {
		cfg := DefaultProbeConfig(serviceID, "https://example.com", now)
		offset := cfg.NextRunAt.Sub(now)
		if offset < 0 || offset >= GlobalProbeInterval {
			t.Fatalf("service %d jitter offset = %s, want within [0, %s)", serviceID, offset, GlobalProbeInterval)
		}
		seenOffsets[offset] = true
	}

	if len(seenOffsets) < 50 {
		t.Fatalf("unique jitter offsets = %d, want broad spread across the interval", len(seenOffsets))
	}
}

func TestNextProbeRunAt(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		currentNextRunAt time.Time
		serviceID        uint
		want             time.Time
	}{
		{
			name:             "due run advances from stored cadence phase",
			currentNextRunAt: now.Add(-time.Minute),
			serviceID:        42,
			want:             now.Add(4 * time.Minute),
		},
		{
			name:             "exactly due run advances one global interval",
			currentNextRunAt: now,
			serviceID:        42,
			want:             now.Add(GlobalProbeInterval),
		},
		{
			name:             "missed intervals preserve stored cadence phase",
			currentNextRunAt: now.Add(-16 * time.Minute),
			serviceID:        42,
			want:             now.Add(4 * time.Minute),
		},
		{
			name:             "future run is kept",
			currentNextRunAt: now.Add(10 * time.Minute),
			serviceID:        42,
			want:             now.Add(10 * time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextProbeRunAt(tt.currentNextRunAt, now, tt.serviceID); !got.Equal(tt.want) {
				t.Fatalf("nextProbeRunAt() = %s, want %s", got, tt.want)
			}
		})
	}

	t.Run("zero next run gets jittered after one global interval", func(t *testing.T) {
		got := nextProbeRunAt(time.Time{}, now, 42)
		min := now.Add(GlobalProbeInterval)
		max := now.Add(2 * GlobalProbeInterval)
		if got.Before(min) || !got.Before(max) {
			t.Fatalf("nextProbeRunAt(zero) = %s, want within [%s, %s)", got, min, max)
		}
	})
}

func TestProbeLeaseDuration(t *testing.T) {
	if got := probeLeaseDuration(10); got != time.Minute {
		t.Fatalf("probeLeaseDuration(10) = %s, want 1m0s", got)
	}
	if got := probeLeaseDuration(45); got != 90*time.Second {
		t.Fatalf("probeLeaseDuration(45) = %s, want 1m30s", got)
	}
}
