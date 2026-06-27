package config

import (
	"testing"

	"github.com/novembersoftware/aretheyup/structs"
)

func TestRolloutGateHelpersDefaultClosed(t *testing.T) {
	previous := C
	C = &structs.Config{}
	t.Cleanup(func() { C = previous })

	if StatusSnapshotAPIReadsEnabled() {
		t.Fatal("StatusSnapshotAPIReadsEnabled() = true, want false by default")
	}
	if StatusSnapshotIncidentReadsEnabled() {
		t.Fatal("StatusSnapshotIncidentReadsEnabled() = true, want false by default")
	}
	if RawProbeRetentionCleanupEnabled() {
		t.Fatal("RawProbeRetentionCleanupEnabled() = true, want false by default")
	}
}

func TestRolloutGateHelpersReadConfig(t *testing.T) {
	previous := C
	C = &structs.Config{
		StatusSnapshotAPIReadsEnabled:      true,
		StatusSnapshotIncidentReadsEnabled: true,
		RawProbeRetentionCleanupEnabled:    true,
	}
	t.Cleanup(func() { C = previous })

	if !StatusSnapshotAPIReadsEnabled() {
		t.Fatal("StatusSnapshotAPIReadsEnabled() = false, want true")
	}
	if !StatusSnapshotIncidentReadsEnabled() {
		t.Fatal("StatusSnapshotIncidentReadsEnabled() = false, want true")
	}
	if !RawProbeRetentionCleanupEnabled() {
		t.Fatal("RawProbeRetentionCleanupEnabled() = false, want true")
	}
}
