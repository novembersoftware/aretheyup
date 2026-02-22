package utils

import (
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
)

func TestToHourOfWeek(t *testing.T) {
	// Boundary checks across start/middle/end of the weekday-hour index range [0..167].
	tests := []struct {
		name string
		at   time.Time
		want int
	}{
		{name: "sunday midnight is zero", at: time.Date(2026, time.January, 4, 0, 0, 0, 0, time.UTC), want: 0},
		{name: "monday afternoon offsets by 24", at: time.Date(2026, time.January, 5, 13, 0, 0, 0, time.UTC), want: 37},
		{name: "saturday last hour is 167", at: time.Date(2026, time.January, 10, 23, 0, 0, 0, time.UTC), want: 167},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToHourOfWeek(tt.at); got != tt.want {
				t.Fatalf("ToHourOfWeek() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDetermineStatus(t *testing.T) {
	// Ensure wrapper behavior matches intended decision inputs:
	// - mature baseline path can trigger issues at low report counts
	// - nil baseline uses cold-start behavior
	// - probe-only path can still trigger issues
	baseline := &structs.ServiceBaseline{
		MeanReports:         0,
		StdDevReports:       1,
		SampleCount:         4,
		ProbeFailureRate:    0.1,
		ProbeFailureSamples: 20,
	}

	gotWithBaseline := DetermineStatus(3, baseline, 0, 0)
	if gotWithBaseline != algorithm.StatusIssuesDetected {
		t.Fatalf("DetermineStatus(with baseline) = %q, want %q", gotWithBaseline, algorithm.StatusIssuesDetected)
	}

	gotWithoutBaseline := DetermineStatus(3, nil, 0, 0)
	if gotWithoutBaseline != algorithm.StatusOperational {
		t.Fatalf("DetermineStatus(no baseline) = %q, want %q", gotWithoutBaseline, algorithm.StatusOperational)
	}

	gotProbeOnly := DetermineStatus(0, nil, 5, 4)
	if gotProbeOnly != algorithm.StatusIssuesDetected {
		t.Fatalf("DetermineStatus(probe-only) = %q, want %q", gotProbeOnly, algorithm.StatusIssuesDetected)
	}
}
