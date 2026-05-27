package utils

import (
	"math"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
)

const (
	probeLatencyElevatedRatio = 1.25
	probeLatencyFailureRatio  = 1.75
)

type ProbePresentation struct {
	RecentTotal        int
	RecentSuccesses    int
	RecentFailures     int
	LastCheckedLabel   string
	LastSuccessLabel   string
	LastFailureLabel   string
	LastOutcome        string
	LastStatusCode     int
	LastResponseTimeMs int
	LatencyAverageMs   int
	LatencyMaxMs       int
	History            []structs.ProbeHistoryEntryResponse
}

func BuildProbePresentation(detail storage.ProbeServiceDetail) ProbePresentation {
	presentation := ProbePresentation{
		History: make([]structs.ProbeHistoryEntryResponse, len(detail.History)),
	}

	successLatencies := make([]int, 0, len(detail.History))
	for i, row := range detail.History {
		outcome := "Failure"
		if row.Success {
			outcome = "Success"
			presentation.RecentSuccesses++
			if row.ResponseTimeMs != nil {
				successLatencies = append(successLatencies, *row.ResponseTimeMs)
			}
		} else {
			presentation.RecentFailures++
		}

		entry := structs.ProbeHistoryEntryResponse{
			Label:        formatProbeTimestamp(row.CheckedAt),
			Outcome:      outcome,
			Success:      row.Success,
			FailureType:  structs.NormalizeProbeFailureType(row.Success, row.FailureType),
			ErrorMessage: row.ErrorMessage,
		}
		if row.StatusCode != nil {
			entry.StatusCode = *row.StatusCode
		}
		if row.ResponseTimeMs != nil {
			entry.ResponseTimeMs = *row.ResponseTimeMs
		}
		presentation.History[i] = entry
	}
	presentation.RecentTotal = len(detail.History)

	if len(detail.History) > 0 {
		last := detail.History[0]
		presentation.LastCheckedLabel = formatProbeTimestamp(last.CheckedAt)
		if last.Success {
			presentation.LastOutcome = "Success"
		} else {
			presentation.LastOutcome = "Failure"
		}
		if last.StatusCode != nil {
			presentation.LastStatusCode = *last.StatusCode
		}
		if last.ResponseTimeMs != nil {
			presentation.LastResponseTimeMs = *last.ResponseTimeMs
		}
	}
	if detail.LastSuccessAt != nil {
		presentation.LastSuccessLabel = formatProbeTimestamp(*detail.LastSuccessAt)
	}
	if detail.LastFailureAt != nil {
		presentation.LastFailureLabel = formatProbeTimestamp(*detail.LastFailureAt)
	}

	if len(successLatencies) > 0 {
		totalLatencyMs := 0
		for _, latencyMs := range successLatencies {
			totalLatencyMs += latencyMs
		}
		presentation.LatencyAverageMs = int(math.Round(float64(totalLatencyMs) / float64(len(successLatencies))))
	}

	maxLatencyMs := 0
	for _, entry := range presentation.History {
		if entry.ResponseTimeMs > maxLatencyMs {
			maxLatencyMs = entry.ResponseTimeMs
		}
	}
	presentation.LatencyMaxMs = maxLatencyMs

	elevatedLatencyMs := 0
	failureLatencyMs := 0
	if presentation.LatencyAverageMs > 0 {
		elevatedLatencyMs = int(math.Ceil(float64(presentation.LatencyAverageMs) * probeLatencyElevatedRatio))
		failureLatencyMs = int(math.Ceil(float64(presentation.LatencyAverageMs) * probeLatencyFailureRatio))
	}

	for i := range presentation.History {
		entry := &presentation.History[i]
		switch {
		case !entry.Success:
			entry.Level = "failure"
		case failureLatencyMs > 0 && entry.ResponseTimeMs >= failureLatencyMs:
			entry.Level = "failure"
		case elevatedLatencyMs > 0 && entry.ResponseTimeMs >= elevatedLatencyMs:
			entry.Level = "elevated"
		default:
			entry.Level = "healthy"
		}
	}

	if maxLatencyMs > 0 {
		for i := range presentation.History {
			entry := &presentation.History[i]
			if entry.ResponseTimeMs > 0 {
				pct := entry.ResponseTimeMs * 100 / maxLatencyMs
				if pct < 4 {
					pct = 4
				}
				entry.HeightPct = pct
			} else if !entry.Success {
				entry.HeightPct = 100
			} else {
				entry.HeightPct = 8
			}
		}
	} else {
		for i := range presentation.History {
			entry := &presentation.History[i]
			if !entry.Success {
				entry.HeightPct = 100
				continue
			}
			entry.HeightPct = 8
		}
	}

	return presentation
}

func formatProbeTimestamp(t time.Time) string {
	return t.UTC().Format("Jan 2, 3:04 PM UTC")
}
