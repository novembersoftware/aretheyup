package storage

import (
	"os"
	"strings"
	"testing"
)

func TestSnapshotReadPathsAvoidRawStatusRecomputation(t *testing.T) {
	storageSource := mustReadSource(t, "storage.go")
	incidentSource := mustReadSource(t, "../workers/incidents.go")

	guarded := map[string]string{
		"ListServicesFromStatusSnapshots":   between(t, storageSource, "func (s *Storage) ListServicesFromStatusSnapshots", "// SearchServices"),
		"SearchServicesFromStatusSnapshots": between(t, storageSource, "func (s *Storage) SearchServicesFromStatusSnapshots", "// GetServiceBySlug"),
		"reconcileIncidentsFromSnapshots":   between(t, incidentSource, "func reconcileIncidentsFromSnapshots", "func reconcileIncidentsFromLegacy"),
	}

	for name, source := range guarded {
		for _, forbidden := range []string{"probe_results", "GetRecentProbeStats", "GetBaselinesForServicesHour"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains forbidden raw status dependency %q", name, forbidden)
			}
		}
	}

	for _, name := range []string{"ListServicesFromStatusSnapshots", "SearchServicesFromStatusSnapshots"} {
		if strings.Contains(guarded[name], "user_reports") {
			t.Fatalf("%s should read recent report counts from service_statuses, not user_reports", name)
		}
		if !strings.Contains(guarded[name], "service_statuses") {
			t.Fatalf("%s should read service_statuses", name)
		}
	}
}

func mustReadSource(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func between(t *testing.T, source, start, end string) string {
	t.Helper()

	startIndex := strings.Index(source, start)
	if startIndex == -1 {
		t.Fatalf("missing start marker %q", start)
	}
	endIndex := strings.Index(source[startIndex:], end)
	if endIndex == -1 {
		t.Fatalf("missing end marker %q", end)
	}
	return source[startIndex : startIndex+endIndex]
}
