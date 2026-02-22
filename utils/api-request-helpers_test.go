package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestFingerprint(t *testing.T) {
	// Contract:
	// - honor explicit client fingerprint
	// - otherwise produce deterministic hash from request traits
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.10:54321"
	c.Request = req

	c.Request.Header.Set("X-Fingerprint", "client-provided")
	if got := RequestFingerprint(c); got != "client-provided" {
		t.Fatalf("RequestFingerprint(header) = %q, want client-provided", got)
	}

	c.Request.Header.Del("X-Fingerprint")
	c.Request.Header.Set("User-Agent", "test-agent")
	c.Request.Header.Set("Accept-Language", "en-US,en;q=0.9")

	expectedHash := sha256.Sum256([]byte("203.0.113.10|test-agent|en-US,en;q=0.9"))
	expected := hex.EncodeToString(expectedHash[:])
	if got := RequestFingerprint(c); got != expected {
		t.Fatalf("RequestFingerprint(fallback) = %q, want %q", got, expected)
	}
}

func TestRequestRegion(t *testing.T) {
	// Priority contract:
	// 1) trusted region headers in order
	// 2) accept-language country token fallback
	// 3) default to Unknown
	gin.SetMode(gin.TestMode)

	t.Run("uses highest-priority valid region header", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("X-Region", " us-west ")
		c.Request.Header.Set("X-Country-Code", "DE")

		if got := RequestRegion(c); got != "US-WEST" {
			t.Fatalf("RequestRegion() = %q, want US-WEST", got)
		}
	})

	t.Run("skips invalid first header and falls through", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("X-Region", "unknown")
		c.Request.Header.Set("CF-IPCountry", "ca")

		if got := RequestRegion(c); got != "CA" {
			t.Fatalf("RequestRegion() = %q, want CA", got)
		}
	})

	t.Run("falls back to language region when no headers exist", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

		if got := RequestRegion(c); got != "BR" {
			t.Fatalf("RequestRegion() = %q, want BR", got)
		}
	})

	t.Run("returns unknown when no valid signals are present", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Accept-Language", "en")

		if got := RequestRegion(c); got != "Unknown" {
			t.Fatalf("RequestRegion() = %q, want Unknown", got)
		}
	})
}

func TestRegionParsingHelpers(t *testing.T) {
	// Direct helper checks for normalization/truncation and language parsing boundaries.
	if got := sanitizeRegion(" this-value-is-way-too-long-to-keep-intact "); got != "THIS-VALUE-IS-WAY-TOO-LO" {
		t.Fatalf("sanitizeRegion(truncate) = %q, want THIS-VALUE-IS-WAY-TOO-LO", got)
	}
	if got := sanitizeRegion("t1"); got != "" {
		t.Fatalf("sanitizeRegion(t1) = %q, want empty", got)
	}
	if got := regionFromAcceptLanguage("en-US,en;q=0.9"); got != "US" {
		t.Fatalf("regionFromAcceptLanguage() = %q, want US", got)
	}
	if got := regionFromAcceptLanguage("en"); got != "" {
		t.Fatalf("regionFromAcceptLanguage(invalid) = %q, want empty", got)
	}
}
