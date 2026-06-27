package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/novembersoftware/aretheyup/structs"
	r "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	cacheKeyVersion        = "v3"
	serviceCacheTTL        = 10 * time.Second
	defaultListCacheLimit  = 49
	defaultListCacheOffset = 0
)

func (s *Storage) GetCachedServiceListResponses(ctx context.Context, limit, offset int) ([]structs.ServiceResponse, bool) {
	if !shouldCacheServiceList(limit, offset) {
		s.recordListCacheBypass()
		return nil, false
	}

	return getCachedJSON[[]structs.ServiceResponse](s, ctx, serviceListResponsesCacheKey(limit, offset), true)
}

func (s *Storage) SetCachedServiceListResponses(ctx context.Context, limit, offset int, response []structs.ServiceResponse) {
	if !shouldCacheServiceList(limit, offset) {
		return
	}

	setCachedJSON(s, ctx, serviceListResponsesCacheKey(limit, offset), response, serviceCacheTTL, true)
}

func (s *Storage) GetCachedServiceDetailResponse(ctx context.Context, slug string) (structs.ServiceDetailResponse, bool) {
	return getCachedJSON[structs.ServiceDetailResponse](s, ctx, serviceDetailResponseCacheKey(slug), false)
}

func (s *Storage) SetCachedServiceDetailResponse(ctx context.Context, slug string, response structs.ServiceDetailResponse) {
	setCachedJSON(s, ctx, serviceDetailResponseCacheKey(slug), response, serviceCacheTTL, false)
}

func (s *Storage) InvalidateServiceCaches(ctx context.Context, slugs ...string) {
	s.recordListCacheInvalidation()
	s.deleteCacheKeys(ctx, serviceCacheKeysForInvalidation(slugs...)...)
}

func (s *Storage) invalidateServiceCachesByID(ctx context.Context, serviceID uint) {
	slug := s.serviceSlugForCacheInvalidation(ctx, serviceID)
	s.InvalidateServiceCaches(ctx, slug)
}

func (s *Storage) serviceSlugForCacheInvalidation(ctx context.Context, serviceID uint) string {
	var service structs.Service
	if err := s.db.WithContext(ctx).
		Model(&structs.Service{}).
		Select("slug").
		Where("id = ?", serviceID).
		First(&service).Error; err != nil {
		log.Debug().Err(err).Uint("service_id", serviceID).Msg("Failed to load service slug for cache invalidation")
		return ""
	}

	return service.Slug
}

func getCachedJSON[T any](s *Storage, ctx context.Context, key string, recordListStats bool) (T, bool) {
	var zero T
	if s.redis == nil {
		if recordListStats {
			s.recordListCacheBypass()
		}
		return zero, false
	}

	payload, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, r.Nil) {
			if recordListStats {
				s.recordListCacheMiss()
			}
			log.Debug().Str("cache_key", key).Msg("Redis cache miss")
		} else {
			if recordListStats {
				s.recordListCacheReadError()
			}
			log.Debug().Err(err).Str("cache_key", key).Msg("Redis cache read failure")
		}
		return zero, false
	}

	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		if recordListStats {
			s.recordListCacheDecodeError()
			s.recordListCacheInvalidation()
		}
		log.Debug().Err(err).Str("cache_key", key).Msg("Redis cache decode failure")
		s.deleteCacheKeys(ctx, key)
		return zero, false
	}

	if recordListStats {
		s.recordListCacheHit()
	}
	log.Debug().Str("cache_key", key).Msg("Redis cache hit")
	return value, true
}

func setCachedJSON(s *Storage, ctx context.Context, key string, value any, ttl time.Duration, recordListStats bool) {
	if s.redis == nil {
		return
	}

	payload, err := json.Marshal(value)
	if err != nil {
		log.Debug().Err(err).Str("cache_key", key).Msg("Redis cache encode failure")
		return
	}

	if err := s.redis.Set(ctx, key, payload, ttl).Err(); err != nil {
		if recordListStats {
			s.recordListCacheWriteError()
		}
		log.Debug().Err(err).Str("cache_key", key).Msg("Redis cache write failure")
	}
}

func (s *Storage) deleteCacheKeys(ctx context.Context, keys ...string) {
	if s.redis == nil || len(keys) == 0 {
		return
	}

	if err := s.redis.Del(ctx, keys...).Err(); err != nil {
		log.Debug().Err(err).Strs("cache_keys", keys).Msg("Redis cache delete failure")
	}
}

func shouldCacheServiceList(limit, offset int) bool {
	return limit == defaultListCacheLimit && offset == defaultListCacheOffset
}

func serviceListResponsesCacheKey(limit, offset int) string {
	return fmt.Sprintf("services:list:%s:limit:%d:offset:%d", cacheKeyVersion, limit, offset)
}

func serviceDetailResponseCacheKey(slug string) string {
	return fmt.Sprintf("services:detail:%s:slug:%s", cacheKeyVersion, slug)
}

func serviceCacheKeysForInvalidation(slugs ...string) []string {
	keys := []string{serviceListResponsesCacheKey(defaultListCacheLimit, defaultListCacheOffset)}
	seen := map[string]bool{}
	for _, slug := range slugs {
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		keys = append(keys, serviceDetailResponseCacheKey(slug))
	}
	return keys
}
