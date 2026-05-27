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
	if !cfg.Enabled || cfg.Method != "GET" || cfg.IntervalSeconds != 60 || cfg.TimeoutSeconds != 10 || cfg.ExpectedStatus != 200 {
		t.Fatalf("unexpected default config values: %+v", cfg)
	}
	if !cfg.NextRunAt.Equal(now) {
		t.Fatalf("NextRunAt = %s, want %s", cfg.NextRunAt, now)
	}
}

func TestNextProbeRunAt(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	gotDue := nextProbeRunAt(now.Add(-time.Minute), now, 60)
	if want := now.Add(time.Minute); !gotDue.Equal(want) {
		t.Fatalf("nextProbeRunAt(due) = %s, want %s", gotDue, want)
	}

	gotRecovered := nextProbeRunAt(now.Add(10*time.Minute), now, 60)
	if want := now.Add(10 * time.Minute); !gotRecovered.Equal(want) {
		t.Fatalf("nextProbeRunAt(recovery) = %s, want %s", gotRecovered, want)
	}
}

func TestProbeLeaseDuration(t *testing.T) {
	if got := probeLeaseDuration(10); got != time.Minute {
		t.Fatalf("probeLeaseDuration(10) = %s, want 1m0s", got)
	}
	if got := probeLeaseDuration(45); got != 90*time.Second {
		t.Fatalf("probeLeaseDuration(45) = %s, want 1m30s", got)
	}
}
