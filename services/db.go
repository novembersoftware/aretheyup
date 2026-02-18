package services

import (
	_ "github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type dbService struct {
	*gorm.DB
}

var DB = &dbService{}

func (s *dbService) Connect() {
	db, err := gorm.Open(postgres.Open(config.C.DBDSN))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	s.DB = db
	log.Info().Msg("Connected to database")
}

func (s *dbService) Migrate() {
	err := s.AutoMigrate(&structs.Service{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate database")
	}
	log.Info().Msg("Database migrated")
}
