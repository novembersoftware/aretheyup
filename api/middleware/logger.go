package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func Logger(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	raw := c.Request.URL.RawQuery

	c.Next()

	if raw != "" {
		path = path + "?" + raw
	}

	event := log.Info()
	if len(c.Errors) > 0 {
		event = log.Error().Err(c.Errors.Last())
	}

	event.
		Str("request_id", c.GetString(RequestIDContextKey)).
		Str("method", c.Request.Method).
		Str("path", path).
		Str("client_ip", c.ClientIP()).
		Int("status", c.Writer.Status()).
		Dur("latency", time.Since(start)).
		Msg("Request completed")
}
