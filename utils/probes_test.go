package utils

import (
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
)

func TestBuildProbePresentation(t *testing.T) {
	lastChecked := time.Date(2026, time.January, 10, 12, 5, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	lastFailure := time.Date(2026, time.January, 10, 11, 45, 0, 0, time.UTC)

	history := []storage.ProbeHistoryRow{
		{CheckedAt: lastChecked, Success: false, StatusCode: intPtr(503), ResponseTimeMs: intPtr(820), FailureType: structs.ProbeFailureTypeHTTPStatus, ErrorMessage: "unexpected status: got 503 want 200"},
		{CheckedAt: lastChecked.Add(-5 * time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(640)},
		{CheckedAt: lastChecked.Add(-10 * time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(700)},
		{CheckedAt: lastChecked.Add(-15 * time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(680)},
	}

	presentation := BuildProbePresentation(storage.ProbeServiceDetail{
		HasConfig:     true,
		Enabled:       true,
		LastCheckedAt: &lastChecked,
		LastSuccessAt: &lastSuccess,
		LastFailureAt: &lastFailure,
		History:       history,
	})

	if presentation.RecentTotal != 4 || presentation.RecentSuccesses != 3 || presentation.RecentFailures != 1 {
		t.Fatalf("recent summary = %+v, want total=4 successes=3 failures=1", presentation)
	}
	if presentation.LastOutcome != "Failure" {
		t.Fatalf("LastOutcome = %q, want Failure", presentation.LastOutcome)
	}
	if presentation.LastStatusCode != 503 {
		t.Fatalf("LastStatusCode = %d, want 503", presentation.LastStatusCode)
	}
	if presentation.LatencyAverageMs != 673 {
		t.Fatalf("LatencyAverageMs = %d, want 673", presentation.LatencyAverageMs)
	}
	if presentation.LastCheckedLabel == "" || presentation.LastSuccessLabel == "" || presentation.LastFailureLabel == "" {
		t.Fatalf("timestamp labels should be populated: %+v", presentation)
	}
	if len(presentation.History) != 4 {
		t.Fatalf("len(History) = %d, want 4", len(presentation.History))
	}
	if presentation.History[0].Outcome != "Failure" || presentation.History[1].Outcome != "Success" {
		t.Fatalf("unexpected history outcomes: %+v", presentation.History)
	}
	if presentation.History[0].FailureType != structs.ProbeFailureTypeHTTPStatus {
		t.Fatalf("FailureType = %q, want %q", presentation.History[0].FailureType, structs.ProbeFailureTypeHTTPStatus)
	}
	if presentation.History[1].FailureType != "" {
		t.Fatalf("success FailureType = %q, want empty", presentation.History[1].FailureType)
	}
	if presentation.History[0].Level != "failure" {
		t.Fatalf("failure Level = %q, want failure", presentation.History[0].Level)
	}
	if presentation.History[1].Level != "healthy" {
		t.Fatalf("640ms Level = %q, want healthy for this fixture", presentation.History[1].Level)
	}
	if presentation.History[3].Level != "healthy" {
		t.Fatalf("680ms Level = %q, want healthy because it matches the fixture median", presentation.History[3].Level)
	}
}

func TestBuildProbePresentationLegacyFailureTypeDefaultsUnknown(t *testing.T) {
	presentation := BuildProbePresentation(storage.ProbeServiceDetail{
		HasConfig: true,
		Enabled:   true,
		History: []storage.ProbeHistoryRow{
			{CheckedAt: time.Date(2026, time.January, 10, 12, 5, 0, 0, time.UTC), Success: false, ErrorMessage: "legacy failure"},
		},
	})

	if len(presentation.History) != 1 {
		t.Fatalf("len(History) = %d, want 1", len(presentation.History))
	}
	if presentation.History[0].FailureType != structs.ProbeFailureTypeUnknown {
		t.Fatalf("legacy FailureType = %q, want %q", presentation.History[0].FailureType, structs.ProbeFailureTypeUnknown)
	}
}

func TestBuildProbePresentationLatencyColorBands(t *testing.T) {
	at := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	presentation := BuildProbePresentation(storage.ProbeServiceDetail{
		HasConfig: true,
		Enabled:   true,
		History: []storage.ProbeHistoryRow{
			{CheckedAt: at, Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(100)},
			{CheckedAt: at.Add(-time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(200)},
			{CheckedAt: at.Add(-2 * time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(300)},
			{CheckedAt: at.Add(-3 * time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(400)},
			{CheckedAt: at.Add(-4 * time.Minute), Success: true, StatusCode: intPtr(200), ResponseTimeMs: intPtr(600)},
			{CheckedAt: at.Add(-4 * time.Minute), Success: false, FailureType: structs.ProbeFailureTypeConnect, ErrorMessage: "dial tcp: connection refused"},
		},
	})

	if presentation.LatencyAverageMs != 320 {
		t.Fatalf("LatencyAverageMs = %d, want 320", presentation.LatencyAverageMs)
	}
	if presentation.History[0].Level != "healthy" {
		t.Fatalf("100ms Level = %q, want healthy", presentation.History[0].Level)
	}
	if presentation.History[3].Level != "elevated" {
		t.Fatalf("400ms Level = %q, want elevated", presentation.History[3].Level)
	}
	if presentation.History[4].Level != "failure" {
		t.Fatalf("600ms Level = %q, want failure", presentation.History[4].Level)
	}
	if presentation.History[5].Level != "failure" {
		t.Fatalf("failed probe Level = %q, want failure", presentation.History[5].Level)
	}
	if presentation.History[5].HeightPct != 100 {
		t.Fatalf("failed probe HeightPct = %d, want 100", presentation.History[5].HeightPct)
	}
}

func TestBuildProbePresentationFailuresStayRedWithoutLatencySamples(t *testing.T) {
	at := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	presentation := BuildProbePresentation(storage.ProbeServiceDetail{
		HasConfig: true,
		Enabled:   true,
		History: []storage.ProbeHistoryRow{
			{CheckedAt: at, Success: false, FailureType: structs.ProbeFailureTypeTimeout, ErrorMessage: "context deadline exceeded"},
			{CheckedAt: at.Add(-time.Minute), Success: false, FailureType: structs.ProbeFailureTypeDNS, ErrorMessage: "lookup example.com: no such host"},
		},
	})

	for i, entry := range presentation.History {
		if entry.Level != "failure" {
			t.Fatalf("History[%d].Level = %q, want failure", i, entry.Level)
		}
		if entry.HeightPct != 100 {
			t.Fatalf("History[%d].HeightPct = %d, want 100", i, entry.HeightPct)
		}
	}
}

func intPtr(v int) *int {
	return &v
}
