package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseTrustedProxiesCSV(t *testing.T) {
	got := parseTrustedProxiesCSV(" 10.0.0.0/8, 127.0.0.1 , ,::1 ")
	if len(got) != 3 {
		t.Fatalf("len(parseTrustedProxiesCSV) = %d, want 3", len(got))
	}
	if got[0] != "10.0.0.0/8" || got[1] != "127.0.0.1" || got[2] != "::1" {
		t.Fatalf("parseTrustedProxiesCSV() = %#v", got)
	}
}

func TestConfigureTrustedProxiesRejectsInvalidValue(t *testing.T) {
	r := gin.New()
	if err := configureTrustedProxies(r, "not-a-cidr"); err == nil {
		t.Fatal("expected configureTrustedProxies to fail on invalid value")
	}
}

func TestClientIPNotTakenFromXFFWhenProxyNotTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	if err := configureTrustedProxies(r, ""); err != nil {
		t.Fatalf("configureTrustedProxies() error = %v", err)
	}
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.ServeHTTP(w, req)

	if got := w.Body.String(); got != "198.51.100.10" {
		t.Fatalf("ClientIP() = %q, want 198.51.100.10", got)
	}
}

func TestClientIPUsesXFFWhenRemoteProxyTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	if err := configureTrustedProxies(r, "198.51.100.10/32"); err != nil {
		t.Fatalf("configureTrustedProxies() error = %v", err)
	}
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.ServeHTTP(w, req)

	if got := w.Body.String(); got != "203.0.113.9" {
		t.Fatalf("ClientIP() = %q, want 203.0.113.9", got)
	}
}
