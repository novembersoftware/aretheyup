package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersPresentOnJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	assertHeader(t, w, "X-Content-Type-Options")
	assertHeader(t, w, "X-Frame-Options")
	assertHeader(t, w, "Referrer-Policy")
	assertHeader(t, w, "Permissions-Policy")
	assertHeader(t, w, "Cross-Origin-Opener-Policy")
	assertHeader(t, w, "Cross-Origin-Resource-Policy")
	assertHeader(t, w, "Content-Security-Policy-Report-Only")
}

func TestSecurityHeadersSetsHSTSWhenForwardedHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/", func(c *gin.Context) {
		c.Status(204)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	assertHeader(t, w, "Strict-Transport-Security")
}

func assertHeader(t *testing.T, w *httptest.ResponseRecorder, key string) {
	t.Helper()
	if value := w.Header().Get(key); value == "" {
		t.Fatalf("expected header %s to be set", key)
	}
}
