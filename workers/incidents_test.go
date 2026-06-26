package workers

import (
	"context"
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
)

type fakeIncidentStore struct {
	serviceIDs []uint
	statuses   []structs.ServiceStatus
	active     map[uint]structs.Incident

	reportCounts map[uint]int64
	baselines    map[uint]structs.ServiceBaseline
	probeStats   map[uint]storage.ProbeStats

	activeServiceIDCalls int
	statusCalls          int
	reportCountCalls     int
	baselineCalls        int
	probeStatsCalls      int
	opened               []uint
	resolved             []uint
}

func (s *fakeIncidentStore) GetActiveServiceIDs(_ context.Context) ([]uint, error) {
	s.activeServiceIDCalls++
	return s.serviceIDs, nil
}

func (s *fakeIncidentStore) GetRecentReportCountsForServices(_ context.Context, _ []uint, _ time.Time) (map[uint]int64, error) {
	s.reportCountCalls++
	return s.reportCounts, nil
}

func (s *fakeIncidentStore) GetBaselinesForServicesHour(_ context.Context, _ []uint, _ int) (map[uint]structs.ServiceBaseline, error) {
	s.baselineCalls++
	return s.baselines, nil
}

func (s *fakeIncidentStore) GetRecentProbeStatsForServices(_ context.Context, _ []uint, _ int) (map[uint]storage.ProbeStats, error) {
	s.probeStatsCalls++
	return s.probeStats, nil
}

func (s *fakeIncidentStore) GetActiveServiceStatuses(_ context.Context) ([]structs.ServiceStatus, error) {
	s.statusCalls++
	return s.statuses, nil
}

func (s *fakeIncidentStore) GetActiveIncidentsByServiceIDs(_ context.Context, _ []uint) (map[uint]structs.Incident, error) {
	if s.active == nil {
		return map[uint]structs.Incident{}, nil
	}
	return s.active, nil
}

func (s *fakeIncidentStore) OpenIncidentIfNoneActive(_ context.Context, serviceID uint, _ time.Time) (bool, error) {
	s.opened = append(s.opened, serviceID)
	return true, nil
}

func (s *fakeIncidentStore) ResolveActiveIncident(_ context.Context, serviceID uint, _ time.Time) (bool, error) {
	s.resolved = append(s.resolved, serviceID)
	return true, nil
}

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

func TestReconcileIncidentsUsesLegacyPathWhenSnapshotFlagDisabled(t *testing.T) {
	store := &fakeIncidentStore{
		serviceIDs: []uint{1},
		reportCounts: map[uint]int64{
			1: 15,
		},
		baselines:  map[uint]structs.ServiceBaseline{},
		probeStats: map[uint]storage.ProbeStats{},
	}

	if err := reconcileIncidents(context.Background(), store, time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC), false); err != nil {
		t.Fatalf("reconcileIncidents() error = %v", err)
	}

	if store.statusCalls != 0 {
		t.Fatalf("snapshot status calls = %d, want 0", store.statusCalls)
	}
	if store.activeServiceIDCalls != 1 || store.reportCountCalls != 1 || store.baselineCalls != 1 || store.probeStatsCalls != 1 {
		t.Fatalf("legacy calls = serviceIDs:%d reports:%d baselines:%d probes:%d, want all 1", store.activeServiceIDCalls, store.reportCountCalls, store.baselineCalls, store.probeStatsCalls)
	}
	if len(store.opened) != 1 || store.opened[0] != 1 {
		t.Fatalf("opened incidents = %v, want [1]", store.opened)
	}
}

func TestReconcileIncidentsUsesSnapshotPathWhenFlagEnabled(t *testing.T) {
	store := &fakeIncidentStore{
		statuses: []structs.ServiceStatus{
			{ServiceID: 2, Status: string(algorithm.StatusOutage)},
		},
	}

	if err := reconcileIncidents(context.Background(), store, time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC), true); err != nil {
		t.Fatalf("reconcileIncidents() error = %v", err)
	}

	if store.statusCalls != 1 {
		t.Fatalf("snapshot status calls = %d, want 1", store.statusCalls)
	}
	if store.activeServiceIDCalls != 0 || store.reportCountCalls != 0 || store.baselineCalls != 0 || store.probeStatsCalls != 0 {
		t.Fatalf("legacy calls = serviceIDs:%d reports:%d baselines:%d probes:%d, want all 0", store.activeServiceIDCalls, store.reportCountCalls, store.baselineCalls, store.probeStatsCalls)
	}
	if len(store.opened) != 1 || store.opened[0] != 2 {
		t.Fatalf("opened incidents = %v, want [2]", store.opened)
	}
}
