package utils

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/config"
	"github.com/novembersoftware/aretheyup/structs"
)

// Respond will respond with HTML or JSON depending on the request
func Respond(c *gin.Context, status int, component string, data interface{}) {
	responseType := strings.ToLower(c.GetHeader("Accept"))
	if strings.Contains(responseType, "application/json") {
		c.JSON(status, data)
		return
	}

	c.HTML(status, component, data)
}

func BuildMeta(c *gin.Context, input structs.MetaInput) *structs.Meta {
	siteName := strings.TrimSpace(input.SiteName)
	if siteName == "" {
		siteName = "Are they up?"
	}

	locale := strings.TrimSpace(input.Locale)
	if locale == "" {
		locale = "en_US"
	}

	title := strings.TrimSpace(input.Title)
	if title == "" && input.ServiceName != "" {
		title = fmt.Sprintf("%s Status: Is %s Down Right Now? | %s", input.ServiceName, input.ServiceName, siteName)
	}
	if title == "" {
		title = fmt.Sprintf("%s | Live Service Status & Outage Reports", siteName)
	}

	description := strings.TrimSpace(input.Description)
	if description == "" && input.ServiceName != "" {
		if input.Status != "" {
			description = fmt.Sprintf("Live %s status: %s. Check user-reported outages and recent uptime signals.", input.ServiceName, input.Status)
		} else {
			description = fmt.Sprintf("Check if %s is down right now. Live user-reported outages and reliability insights.", input.ServiceName)
		}
	}
	if description == "" {
		description = "Monitor live user-reported outages and service reliability in real time."
	}

	robots := strings.TrimSpace(input.Robots)
	if robots == "" {
		robots = "index,follow"
	}

	themeColor := strings.TrimSpace(input.ThemeColor)
	if themeColor == "" {
		themeColor = "#ff555d"
	}

	canonicalURL := buildAbsoluteURL(c, input.CanonicalPath)
	imageURL := buildAbsoluteURL(c, input.ImageURL)
	prevURL := buildAbsoluteURL(c, input.PrevURL)
	nextURL := buildAbsoluteURL(c, input.NextURL)

	meta := &structs.Meta{
		Title:         title,
		Description:   description,
		Keywords:      input.Keywords,
		CanonicalURL:  canonicalURL,
		Robots:        robots,
		ThemeColor:    themeColor,
		Author:        strings.TrimSpace(input.Author),
		SiteName:      siteName,
		Locale:        locale,
		PublishedTime: strings.TrimSpace(input.PublishedTime),
		ModifiedTime:  strings.TrimSpace(input.ModifiedTime),
		ImageURL:      imageURL,
		ImageAlt:      strings.TrimSpace(input.ImageAlt),
		JSONLD:        input.JSONLD,
		PrevURL:       prevURL,
		NextURL:       nextURL,
	}

	meta.OpenGraph = structs.OpenGraphMeta{
		Title:       title,
		Description: description,
		Type:        "website",
		URL:         canonicalURL,
		SiteName:    siteName,
		Locale:      locale,
		ImageURL:    imageURL,
		ImageAlt:    strings.TrimSpace(input.ImageAlt),
	}

	meta.Twitter = structs.TwitterMeta{
		Card:        "summary_large_image",
		Title:       title,
		Description: description,
		ImageURL:    imageURL,
		ImageAlt:    strings.TrimSpace(input.ImageAlt),
	}

	return meta
}

func buildAbsoluteURL(c *gin.Context, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if u, err := url.Parse(raw); err == nil && u.IsAbs() {
		return u.String()
	}

	baseURL := strings.TrimSpace(config.C.SiteBaseURL)
	if baseURL == "" && c != nil {
		scheme := "http"
		if c.Request != nil && c.Request.TLS != nil {
			scheme = "https"
		}
		if forwardedProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
			scheme = forwardedProto
		}

		host := ""
		if c.Request != nil {
			host = c.Request.Host
		}
		if host != "" {
			baseURL = fmt.Sprintf("%s://%s", scheme, host)
		}
	}

	if baseURL == "" {
		return raw
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return raw
	}

	if strings.HasPrefix(raw, "/") {
		base.Path = path.Clean(raw)
		base.RawQuery = ""
		base.Fragment = ""
		return base.String()
	}

	resolved, err := base.Parse(raw)
	if err != nil {
		return raw
	}
	return resolved.String()
}
