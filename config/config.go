package config

import (
	"log"

	"github.com/caarlos0/env"
	"github.com/novembersoftware/aretheyup/structs"
)

var C *structs.Config

func Load() {
	C = &structs.Config{}
	if err := env.Parse(C); err != nil {
		log.Fatal(err)
	}
}

func IsProd() bool {
	return C.Env == "prod"
}

func StatusSnapshotAPIReadsEnabled() bool {
	return C != nil && C.StatusSnapshotAPIReadsEnabled
}

func StatusSnapshotIncidentReadsEnabled() bool {
	return C != nil && C.StatusSnapshotIncidentReadsEnabled
}

func RawProbeRetentionCleanupEnabled() bool {
	return C != nil && C.RawProbeRetentionCleanupEnabled
}
