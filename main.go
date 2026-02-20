package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/novembersoftware/aretheyup/api"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/novembersoftware/aretheyup/storage"
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
	db, err := services.NewDB(config.C.DBDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	if err := services.MigrateDB(db); err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate database")
	}

	// services.Seed(db, 50, true)

	_, err = services.NewRedis(config.C.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	store := storage.New(db)
	api.Start(store)
}
