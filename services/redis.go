package services

import (
	"context"

	"github.com/novembersoftware/aretheyup/config"
	r "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type redisService struct {
	*r.Client
}

var Redis = &redisService{}

func (s *redisService) Connect() {
	opt, err := r.ParseURL(config.C.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}
	s.Client = r.NewClient(opt)

	_, err = s.Client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to ping Redis")
	}
	log.Info().Msg("Connected to Redis")
}
