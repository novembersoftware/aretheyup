package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/utils"
)

type originValidator struct {
	allowed map[string]struct{}
}

func newOriginValidator(allowedOriginsCSV string) originValidator {
	allowed := make(map[string]struct{})
	for value := range strings.SplitSeq(allowedOriginsCSV, ",") {
		normalized := normalizeOrigin(value)
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}

	return originValidator{allowed: allowed}
}

func RequireAllowedPageOrigin(allowedOriginsCSV string) gin.HandlerFunc {
	validator := newOriginValidator(allowedOriginsCSV)

	return func(c *gin.Context) {
		if validator.allowsRequest(c) {
			c.Next()
			return
		}

		utils.Respond(c, http.StatusForbidden, "error", gin.H{"error": "Origin not allowed"})
		c.Abort()
	}
}

func (v originValidator) allowsRequest(c *gin.Context) bool {
	origin := normalizeOrigin(c.GetHeader("Origin"))
	if origin != "" {
		return v.matches(origin)
	}

	if !isHTMXRequest(c) {
		return true
	}

	refererOrigin := originFromReferer(c.GetHeader("Referer"))
	if refererOrigin != "" {
		return v.matches(refererOrigin)
	}

	return false
}

func (v originValidator) matches(origin string) bool {
	if origin == "" {
		return false
	}

	_, ok := v.allowed[origin]
	return ok
}

func normalizeOrigin(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}

	u, err := url.Parse(v)
	if err != nil {
		return ""
	}

	if u.Scheme == "" || u.Host == "" {
		return ""
	}

	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}

func originFromReferer(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}

	u, err := url.Parse(v)
	if err != nil {
		return ""
	}

	if u.Scheme == "" || u.Host == "" {
		return ""
	}

	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}
