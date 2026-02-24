package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

const RequestIDContextKey = "request_id"
const maxRequestIDLength = 128

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, ok := normalizeRequestID(c.GetHeader("X-Request-ID"))
		if !ok {
			requestID = newRequestID()
		}

		c.Set(RequestIDContextKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func normalizeRequestID(value string) (string, bool) {
	v := strings.TrimSpace(value)
	if v == "" || len(v) > maxRequestIDLength {
		return "", false
	}

	for _, r := range v {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', '.', ':':
			continue
		default:
			return "", false
		}
	}

	return v, true
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}

	return strconv.FormatInt(time.Now().UTC().UnixNano(), 16)
}
