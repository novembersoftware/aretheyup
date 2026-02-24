package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReportRouteKeyPartIgnoresClientFingerprintHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w1 := httptest.NewRecorder()
	c1, r1 := gin.CreateTestContext(w1)
	r1.POST("/api/service/:slug/report", func(c *gin.Context) {})
	c1.Params = gin.Params{{Key: "slug", Value: "service-a"}}
	req1 := httptest.NewRequest("POST", "/api/service/service-a/report", nil)
	req1.RemoteAddr = "203.0.113.10:12345"
	req1.Header.Set("User-Agent", "agent")
	req1.Header.Set("Accept-Language", "en-US")
	req1.Header.Set("X-Fingerprint", "attacker-a")
	c1.Request = req1

	w2 := httptest.NewRecorder()
	c2, r2 := gin.CreateTestContext(w2)
	r2.POST("/api/service/:slug/report", func(c *gin.Context) {})
	c2.Params = gin.Params{{Key: "slug", Value: "service-a"}}
	req2 := httptest.NewRequest("POST", "/api/service/service-a/report", nil)
	req2.RemoteAddr = "203.0.113.10:54321"
	req2.Header.Set("User-Agent", "agent")
	req2.Header.Set("Accept-Language", "en-US")
	req2.Header.Set("X-Fingerprint", "attacker-b")
	c2.Request = req2

	if gotA, gotB := reportRouteKeyPart(c1), reportRouteKeyPart(c2); gotA != gotB {
		t.Fatalf("expected same key part when only X-Fingerprint differs, got %q and %q", gotA, gotB)
	}
}
