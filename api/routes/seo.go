package routes

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/storage"
)

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func getRobotsTxt(c *gin.Context) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.C.SiteBaseURL), "/")

	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	b.WriteString("Disallow: /api/\n")
	if baseURL != "" {
		b.WriteString("Sitemap: ")
		b.WriteString(baseURL)
		b.WriteString("/sitemap.xml\n")
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}

func getSitemapXML(c *gin.Context, store *storage.Storage) {
	rows, err := store.ListActiveServicesForSitemap(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build sitemap"})
		return
	}

	baseURL := strings.TrimRight(strings.TrimSpace(config.C.SiteBaseURL), "/")
	if baseURL == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if forwardedProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
			scheme = forwardedProto
		}
		baseURL = scheme + "://" + c.Request.Host
	}

	urls := make([]sitemapURL, 0, len(rows)+1)
	urls = append(urls, sitemapURL{Loc: baseURL + "/"})
	for _, row := range rows {
		urls = append(urls, sitemapURL{
			Loc:     baseURL + "/" + row.Slug,
			LastMod: row.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	payload := sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	bytes, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build sitemap"})
		return
	}

	result := append([]byte(xml.Header), bytes...)
	c.Data(http.StatusOK, "application/xml; charset=utf-8", result)
}
