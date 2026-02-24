package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/utils"
	r "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type RateLimitConfig struct {
	Name      string
	Limit     int64
	Window    time.Duration
	Message   string
	KeyFunc   func(*gin.Context) string
	ScopeFunc func(*gin.Context) string
}

type ReportRateLimitState struct {
	CanReport         bool
	RetryAfterSeconds int64
}

const (
	reportRateLimitName  = "report-route"
	reportRateLimitScope = "POST:/api/service/:slug/report"
)

var rateLimitScript = r.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)

func NewRateLimit(redis *r.Client, cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if redis == nil || cfg.Limit <= 0 || cfg.Window <= 0 {
			c.Next()
			return
		}

		keyPart := stableHash(c.ClientIP())
		if cfg.KeyFunc != nil {
			custom := strings.TrimSpace(cfg.KeyFunc(c))
			if custom != "" {
				keyPart = custom
			}
		}

		scope := "all"
		if cfg.ScopeFunc != nil {
			custom := strings.TrimSpace(cfg.ScopeFunc(c))
			if custom != "" {
				scope = custom
			}
		}

		key := buildRateLimitKey(cfg.Name, scope, keyPart)
		count, ttl, err := hitRateLimit(c, redis, key, cfg.Window)
		if err != nil {
			log.Warn().Err(err).Str("rate_limit_key", key).Msg("Rate limiter fallback to allow request")
			c.Next()
			return
		}

		remaining := cfg.Limit - count
		if remaining < 0 {
			remaining = 0
		}

		resetSeconds := int64(math.Ceil(ttl.Seconds()))
		if resetSeconds <= 0 {
			resetSeconds = int64(cfg.Window.Seconds())
		}

		c.Header("X-RateLimit-Limit", strconv.FormatInt(cfg.Limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetSeconds, 10))

		if count > cfg.Limit {
			c.Header("Retry-After", strconv.FormatInt(resetSeconds, 10))

			if cfg.Name == reportRateLimitName && isHTMXRequest(c) && !wantsJSON(c) {
				triggerPayload := map[string]any{
					"report-rate-limited": map[string]any{
						"message":             cfg.Message,
						"retry_after_seconds": resetSeconds,
					},
				}
				if encoded, err := json.Marshal(triggerPayload); err == nil {
					c.Header("HX-Trigger", string(encoded))
				}

				c.Status(204)
				c.Abort()
				return
			}

			utils.Respond(c, 429, "error", gin.H{
				"error": cfg.Message,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func ReportRouteRateLimit(redis *r.Client, limit int64, window time.Duration) gin.HandlerFunc {
	return NewRateLimit(redis, RateLimitConfig{
		Name:    reportRateLimitName,
		Limit:   limit,
		Window:  window,
		Message: "We already received your report for this service.",
		KeyFunc: reportRouteKeyPart,
		ScopeFunc: func(c *gin.Context) string {
			return reportRateLimitScope
		},
	})
}

func GetReportRateLimitState(c *gin.Context, redis *r.Client, window time.Duration) (ReportRateLimitState, error) {
	state := ReportRateLimitState{CanReport: true}
	if redis == nil || window <= 0 {
		return state, nil
	}

	key := buildRateLimitKey(reportRateLimitName, reportRateLimitScope, reportRouteKeyPart(c))
	ttl, err := redis.PTTL(c.Request.Context(), key).Result()
	if err != nil {
		if errors.Is(err, r.Nil) {
			return state, nil
		}
		return state, err
	}

	if ttl <= 0 {
		return state, nil
	}

	state.CanReport = false
	retryAfter := int64(math.Ceil(ttl.Seconds()))
	if retryAfter <= 0 {
		retryAfter = int64(window.Seconds())
	}
	state.RetryAfterSeconds = retryAfter

	return state, nil
}

func hitRateLimit(c *gin.Context, redis *r.Client, key string, window time.Duration) (int64, time.Duration, error) {
	result, err := rateLimitScript.Run(c.Request.Context(), redis, []string{key}, window.Milliseconds()).Result()
	if err != nil {
		return 0, 0, err
	}

	items, ok := result.([]interface{})
	if !ok || len(items) != 2 {
		return 0, 0, fmt.Errorf("unexpected rate limit script result")
	}

	count, err := toInt64(items[0])
	if err != nil {
		return 0, 0, err
	}

	ttlMS, err := toInt64(items[1])
	if err != nil {
		return 0, 0, err
	}

	if ttlMS < 0 {
		ttlMS = window.Milliseconds()
	}

	return count, time.Duration(ttlMS) * time.Millisecond, nil
}

func reportRouteKeyPart(c *gin.Context) string {
	fingerprint := stableHash(c.ClientIP() + "|" + c.GetHeader("User-Agent") + "|" + c.GetHeader("Accept-Language"))

	return stableHash("report|" + c.Param("slug") + "|" + fingerprint)
}

func buildRateLimitKey(name string, scope string, keyPart string) string {
	return fmt.Sprintf("rate_limit:%s:%s:%s", name, scope, keyPart)
}

func stableHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported int64 type %T", value)
	}
}

func wantsJSON(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/json")
}

func isHTMXRequest(c *gin.Context) bool {
	return strings.EqualFold(c.GetHeader("HX-Request"), "true")
}
