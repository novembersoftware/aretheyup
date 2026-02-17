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
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Caller().Logger()
	_ = godotenv.Load(".env.local")
	config.Load()
}

func main() {
	services.DB.Connect()
	services.Redis.Connect()
	api.Start()
}
