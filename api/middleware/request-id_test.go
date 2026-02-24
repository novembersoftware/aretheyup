package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) {
		c.Status(204)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("expected generated X-Request-ID")
	}
}

func TestRequestIDUsesValidIncomingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) {
		c.Status(204)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "abc-123:trace")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "abc-123:trace" {
		t.Fatalf("X-Request-ID = %q, want abc-123:trace", got)
	}
}

func TestRequestIDReplacesInvalidIncomingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) {
		c.Status(204)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", strings.Repeat("a", maxRequestIDLength+1))
	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("expected generated X-Request-ID for invalid input")
	}
	if requestID == strings.Repeat("a", maxRequestIDLength+1) {
		t.Fatal("expected invalid incoming request ID to be replaced")
	}
}
