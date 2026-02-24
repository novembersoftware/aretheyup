package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequestFingerprint(c *gin.Context) string {
	hash := sha256.Sum256([]byte(GetClientIP(c) + "|" + c.GetHeader("User-Agent") + "|" + c.GetHeader("Accept-Language")))
	return hex.EncodeToString(hash[:])
}

func RequestRegion(c *gin.Context) string {
	for _, key := range []string{
		"X-Region",
		"X-Country-Code",
		"CF-IPCountry",
		"CloudFront-Viewer-Country",
		"X-AppEngine-Country",
	} {
		if value := sanitizeRegion(c.GetHeader(key)); value != "" {
			return value
		}
	}

	if languageRegion := regionFromAcceptLanguage(c.GetHeader("Accept-Language")); languageRegion != "" {
		return languageRegion
	}

	return "Unknown"
}

func sanitizeRegion(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" || v == "XX" || v == "T1" || v == "A1" || v == "UNKNOWN" {
		return ""
	}
	if len(v) > 24 {
		return v[:24]
	}
	return v
}

func regionFromAcceptLanguage(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}

	first := strings.TrimSpace(parts[0])
	if first == "" {
		return ""
	}

	lang := strings.Split(first, ";")[0]
	chunks := strings.Split(lang, "-")
	if len(chunks) < 2 {
		return ""
	}

	return sanitizeRegion(chunks[len(chunks)-1])
}
