package middleware

import "github.com/gin-gonic/gin"

const apiAllowHeaders = "Accept, Content-Type, X-Fingerprint, HX-Request, HX-Current-URL, X-Requested-With, X-Request-ID, traceparent, tracestate"
const apiExposeHeaders = "X-Request-ID, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After"

func OpenAPICORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", apiAllowHeaders)
		c.Header("Access-Control-Expose-Headers", apiExposeHeaders)
		c.Header("Access-Control-Max-Age", "600")

		if c.Request.Method == "OPTIONS" {
			c.Status(204)
			c.Abort()
			return
		}

		c.Next()
	}
}
