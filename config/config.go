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
