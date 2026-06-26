package workers

import (
	"context"
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/algorithm"
	"github.com/novembersoftware/aretheyup/structs"
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

func TestReconcileIncidentsUsesSnapshots(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	store := &fakeIncidentStore{
		statuses: []structs.ServiceStatus{
			{ServiceID: 1, Status: string(algorithm.StatusOutage)},
			{ServiceID: 2, Status: string(algorithm.StatusDegraded)},
			{ServiceID: 3, Status: string(algorithm.StatusOperational)},
		},
		active: map[uint]structs.Incident{
			2: {ServiceID: 2, StartedAt: now.Add(-time.Hour)},
			3: {ServiceID: 3, StartedAt: now.Add(-time.Hour)},
			4: {ServiceID: 4, StartedAt: now.Add(-time.Hour)},
		},
	}

	stats, err := reconcileIncidents(context.Background(), store, now)
	if err != nil {
		t.Fatalf("reconcileIncidents() error = %v", err)
	}

	if stats.ServicesScanned != 3 || stats.Opened != 1 || stats.Resolved != 2 {
		t.Fatalf("stats = %+v, want services/opened/resolved 3/1/2", stats)
	}
	if len(store.opened) != 1 || store.opened[0] != 1 {
		t.Fatalf("opened = %v, want [1]", store.opened)
	}
	if len(store.resolved) != 2 || store.resolved[0] != 2 || store.resolved[1] != 3 {
		t.Fatalf("resolved = %v, want [2 3]", store.resolved)
	}
	if store.requestedActiveIDs[0] != 1 || store.requestedActiveIDs[1] != 2 || store.requestedActiveIDs[2] != 3 {
		t.Fatalf("active incident lookup IDs = %v, want [1 2 3]", store.requestedActiveIDs)
	}
}

type fakeIncidentStore struct {
	statuses           []structs.ServiceStatus
	active             map[uint]structs.Incident
	requestedActiveIDs []uint
	opened             []uint
	resolved           []uint
}

func (f *fakeIncidentStore) GetActiveServiceStatuses(ctx context.Context) ([]structs.ServiceStatus, error) {
	return f.statuses, nil
}

func (f *fakeIncidentStore) GetActiveIncidentsByServiceIDs(ctx context.Context, serviceIDs []uint) (map[uint]structs.Incident, error) {
	f.requestedActiveIDs = append([]uint(nil), serviceIDs...)
	out := make(map[uint]structs.Incident, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		if incident, ok := f.active[serviceID]; ok {
			out[serviceID] = incident
		}
	}
	return out, nil
}

func (f *fakeIncidentStore) OpenIncidentIfNoneActive(ctx context.Context, serviceID uint, startedAt time.Time) (bool, error) {
	f.opened = append(f.opened, serviceID)
	return true, nil
}

func (f *fakeIncidentStore) ResolveActiveIncident(ctx context.Context, serviceID uint, resolvedAt time.Time) (bool, error) {
	f.resolved = append(f.resolved, serviceID)
	return true, nil
}
