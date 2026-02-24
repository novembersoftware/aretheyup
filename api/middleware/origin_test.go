package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireWebsiteWriteOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := RequireWebsiteWriteOrigin("https://aretheyup.com,https://www.aretheyup.com")

	r := gin.New()
	r.POST("/api/service/:slug/report", handler, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	t.Run("allows matching origin with htmx", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/service/test/report", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("HX-Request", "true")
		req.Header.Set("Origin", "https://aretheyup.com")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("allows referer fallback when origin missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/service/test/report", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("HX-Request", "true")
		req.Header.Set("Referer", "https://www.aretheyup.com/test")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("rejects disallowed origin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/service/test/report", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("HX-Request", "true")
		req.Header.Set("Origin", "https://evil.example")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("rejects non-htmx requests", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/service/test/report", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Origin", "https://aretheyup.com")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})
}

func TestRequireWebsiteAPIOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := RequireWebsiteAPIOrigin("https://aretheyup.com")

	r := gin.New()
	r.GET("/api/services", handler, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	t.Run("allows origin without htmx", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/services", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Origin", "https://aretheyup.com")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("rejects request with no origin and no referer", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/services", nil)
		req.Header.Set("Accept", "application/json")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("rejects cross site sec fetch", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/services", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Origin", "https://aretheyup.com")
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})
}
