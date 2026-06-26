package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/novembersoftware/aretheyup/api"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/manage"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/utils"
	"github.com/novembersoftware/aretheyup/workers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var flags utils.Flags

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

	flags = utils.ParseFlags()
}

func main() {
	db, err := services.NewDB(config.C.DBDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	if err := services.MigrateDB(db); err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate database")
	}

	if flags.Mode == utils.ModeBackfillProbeRollups {
		backfillProbeRollupsMode(storage.New(db, nil))
		return
	}

	redis, err := services.NewRedis(config.C.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	store := storage.New(db, redis)
	if inserted, err := store.BackfillMissingProbeConfigs(context.Background(), time.Now().UTC()); err != nil {
		log.Fatal().Err(err).Msg("Failed to backfill default probe configs")
	} else if inserted > 0 {
		log.Info().Int64("probe_configs_created", inserted).Msg("Backfilled missing default probe configs")
	}

	switch flags.Mode {
	case utils.ModeAPI:
		apiMode(store)
	case utils.ModeManage:
		manageMode(store)
	case utils.ModeProbe:
		probeMode(store)
	case utils.ModeSeed:
		seedMode(db)
	case utils.ModeWorker:
		workerMode(store)
	}
}

func apiMode(store *storage.Storage) {
	log.Info().Msg("Starting API mode")
	api.Start(store)
}

func manageMode(store *storage.Storage) {
	if err := manage.Start(store); err != nil {
		log.Fatal().Err(err).Msg("TUI error")
	}
}

func probeMode(store *storage.Storage) {
	log.Info().Msg("Starting probe mode")
	if err := workers.RunProbeWorker(store); err != nil {
		log.Fatal().Err(err).Msg("Probe worker stopped")
	}
}

func seedMode(db *gorm.DB) {
	services.SeedDB(db, flags.SeedCount, flags.SeedClear)
}

func backfillProbeRollupsMode(store *storage.Storage) {
	inserted, err := store.BackfillProbeHourlyRollups(context.Background(), flags.BackfillProbeRollupsStart, flags.BackfillProbeRollupsEnd)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to backfill probe hourly rollups")
	}

	log.Info().
		Int64("rows_affected", inserted).
		Time("start", flags.BackfillProbeRollupsStart).
		Time("end", flags.BackfillProbeRollupsEnd).
		Msg("Backfilled probe hourly rollups")
}

func workerMode(store *storage.Storage) {
	log.Info().Msg("Starting worker mode")
	workers.StartBaselineRefresher(store)
	workers.StartIncidentTracker(store)
	workers.StartProbeResultCleaner(store)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Info().Msg("Worker shutdown requested")
}
