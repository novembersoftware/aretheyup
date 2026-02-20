package services

import (
	"context"

	r "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// NewRedis parses the provided URL, opens a Redis client, and verifies connectivity
func NewRedis(url string) (*r.Client, error) {
	opt, err := r.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := r.NewClient(opt)

	if _, err = client.Ping(context.Background()).Result(); err != nil {
		return nil, err
	}

	log.Info().Msg("Connected to Redis")
	return client, nil
}
