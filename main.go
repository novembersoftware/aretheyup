package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/novembersoftware/aretheyup/api"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	_ = godotenv.Load(".env.local")
	config.Load()

	if config.IsProd() {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Caller().Logger()
	}
}

func main() {
	services.DB.Connect()
	services.DB.Migrate()
	// services.DB.Seed(50)
	services.Redis.Connect()
	api.Start()
}
