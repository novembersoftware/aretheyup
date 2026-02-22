package utils

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRespondJSONWhenAcceptIncludesJSON(t *testing.T) {
	// JSON should win whenever the Accept header includes application/json,
	// even when other media types are also present.
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Accept", "application/json, text/html;q=0.9")

	Respond(c, 200, "ignored", gin.H{"ok": true})

	if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if body := w.Body.String(); !strings.Contains(body, `"ok":true`) {
		t.Fatalf("body = %q, want JSON payload", body)
	}
}

func TestRespondHTMLWhenJSONNotRequested(t *testing.T) {
	// When JSON is not requested, the named HTML component should be rendered.
	gin.SetMode(gin.TestMode)

	tmpl := template.Must(template.New("service-card").Parse(`{{define "service-card"}}<div>{{.Message}}</div>{{end}}`))

	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)
	engine.SetHTMLTemplate(tmpl)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Accept", "text/html")

	Respond(c, 200, "service-card", gin.H{"Message": "hello"})

	if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	if body := w.Body.String(); !strings.Contains(body, "<div>hello</div>") {
		t.Fatalf("body = %q, want rendered HTML component", body)
	}
}
