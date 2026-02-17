package services

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/rs/zerolog/log"
)

type dbService struct {
	*sql.DB
}

var DB = &dbService{}

func (s *dbService) Connect() {
	db, err := sql.Open("postgres", config.C.DBDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	err = db.Ping()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to ping database")
	}

	s.DB = db
	log.Info().Msg("Connected to database")
}
