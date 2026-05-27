package workers

import (
	"testing"

	"github.com/novembersoftware/aretheyup/algorithm"
)

func TestIncidentTransition(t *testing.T) {
	tests := []struct {
		name              string
		status            algorithm.Status
		hasActiveIncident bool
		wantOpen          bool
		wantResolve       bool
	}{
		{
			name:              "operational to operational does nothing",
			status:            algorithm.StatusOperational,
			hasActiveIncident: false,
		},
		{
			name:              "degraded without active incident does not open",
			status:            algorithm.StatusDegraded,
			hasActiveIncident: false,
		},
		{
			name:              "outage without active incident opens",
			status:            algorithm.StatusOutage,
			hasActiveIncident: false,
			wantOpen:          true,
		},
		{
			name:              "degraded with active incident resolves",
			status:            algorithm.StatusDegraded,
			hasActiveIncident: true,
			wantResolve:       true,
		},
		{
			name:              "operational with active incident resolves",
			status:            algorithm.StatusOperational,
			hasActiveIncident: true,
			wantResolve:       true,
		},
		{
			name:              "outage with active incident does nothing",
			status:            algorithm.StatusOutage,
			hasActiveIncident: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOpen, gotResolve := incidentTransition(tt.status, tt.hasActiveIncident)
			if gotOpen != tt.wantOpen || gotResolve != tt.wantResolve {
				t.Fatalf("incidentTransition(%q, %v) = (%v, %v), want (%v, %v)", tt.status, tt.hasActiveIncident, gotOpen, gotResolve, tt.wantOpen, tt.wantResolve)
			}
		})
	}
}
