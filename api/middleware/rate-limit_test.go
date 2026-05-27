package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReportRouteKeyPartIgnoresUserAgentAndAcceptLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w1 := httptest.NewRecorder()
	c1, r1 := gin.CreateTestContext(w1)
	r1.POST("/api/service/:slug/report", func(c *gin.Context) {})
	c1.Params = gin.Params{{Key: "slug", Value: "service-a"}}
	req1 := httptest.NewRequest("POST", "/api/service/service-a/report", nil)
	req1.RemoteAddr = "203.0.113.10:12345"
	req1.Header.Set("User-Agent", "agent-a")
	req1.Header.Set("Accept-Language", "en-US")
	c1.Request = req1

	w2 := httptest.NewRecorder()
	c2, r2 := gin.CreateTestContext(w2)
	r2.POST("/api/service/:slug/report", func(c *gin.Context) {})
	c2.Params = gin.Params{{Key: "slug", Value: "service-a"}}
	req2 := httptest.NewRequest("POST", "/api/service/service-a/report", nil)
	req2.RemoteAddr = "203.0.113.10:54321"
	req2.Header.Set("User-Agent", "agent-b")
	req2.Header.Set("Accept-Language", "fr-CA")
	c2.Request = req2

	if gotA, gotB := reportRouteKeyPart(c1), reportRouteKeyPart(c2); gotA != gotB {
		t.Fatalf("expected same key part when only User-Agent and Accept-Language differ, got %q and %q", gotA, gotB)
	}
}

func TestReportRouteKeyPartDiffersForDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w1 := httptest.NewRecorder()
	c1, r1 := gin.CreateTestContext(w1)
	r1.POST("/api/service/:slug/report", func(c *gin.Context) {})
	c1.Params = gin.Params{{Key: "slug", Value: "service-a"}}
	req1 := httptest.NewRequest("POST", "/api/service/service-a/report", nil)
	req1.RemoteAddr = "203.0.113.10:12345"
	c1.Request = req1

	w2 := httptest.NewRecorder()
	c2, r2 := gin.CreateTestContext(w2)
	r2.POST("/api/service/:slug/report", func(c *gin.Context) {})
	c2.Params = gin.Params{{Key: "slug", Value: "service-a"}}
	req2 := httptest.NewRequest("POST", "/api/service/service-a/report", nil)
	req2.RemoteAddr = "203.0.113.11:54321"
	c2.Request = req2

	if gotA, gotB := reportRouteKeyPart(c1), reportRouteKeyPart(c2); gotA == gotB {
		t.Fatalf("expected different key parts for different IPs, got %q", gotA)
	}
}

func TestServiceSubmissionRouteKeyPartIgnoresUserAgentAndAcceptLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w1 := httptest.NewRecorder()
	c1, r1 := gin.CreateTestContext(w1)
	r1.POST("/api/services/submit", func(c *gin.Context) {})
	req1 := httptest.NewRequest("POST", "/api/services/submit", nil)
	req1.RemoteAddr = "203.0.113.10:12345"
	req1.Header.Set("User-Agent", "agent-a")
	req1.Header.Set("Accept-Language", "en-US")
	c1.Request = req1

	w2 := httptest.NewRecorder()
	c2, r2 := gin.CreateTestContext(w2)
	r2.POST("/api/services/submit", func(c *gin.Context) {})
	req2 := httptest.NewRequest("POST", "/api/services/submit", nil)
	req2.RemoteAddr = "203.0.113.10:54321"
	req2.Header.Set("User-Agent", "agent-b")
	req2.Header.Set("Accept-Language", "fr-CA")
	c2.Request = req2

	if gotA, gotB := serviceSubmissionRouteKeyPart(c1), serviceSubmissionRouteKeyPart(c2); gotA != gotB {
		t.Fatalf("expected same key part when only User-Agent and Accept-Language differ, got %q and %q", gotA, gotB)
	}
}

func TestServiceSubmissionRouteKeyPartDiffersForDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w1 := httptest.NewRecorder()
	c1, r1 := gin.CreateTestContext(w1)
	r1.POST("/api/services/submit", func(c *gin.Context) {})
	req1 := httptest.NewRequest("POST", "/api/services/submit", nil)
	req1.RemoteAddr = "203.0.113.10:12345"
	c1.Request = req1

	w2 := httptest.NewRecorder()
	c2, r2 := gin.CreateTestContext(w2)
	r2.POST("/api/services/submit", func(c *gin.Context) {})
	req2 := httptest.NewRequest("POST", "/api/services/submit", nil)
	req2.RemoteAddr = "203.0.113.11:54321"
	c2.Request = req2

	if gotA, gotB := serviceSubmissionRouteKeyPart(c1), serviceSubmissionRouteKeyPart(c2); gotA == gotB {
		t.Fatalf("expected different key parts for different IPs, got %q", gotA)
	}
}
