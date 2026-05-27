package storage

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
)

type SubmitServiceInput struct {
	Name                 string
	Category             string
	HomepageURL          string
	SubmitterEmail       string
	SubmitterFingerprint string
}

type SubmitServiceResult struct {
	Service    structs.Service
	Submission structs.ServiceSubmission
}

type SubmitServiceErrorCode string

const (
	SubmitServiceErrorInvalid   SubmitServiceErrorCode = "invalid"
	SubmitServiceErrorDuplicate SubmitServiceErrorCode = "duplicate"
)

const serviceSubmissionStatusPublishedUnverified = "published_unverified"

var allowedServiceCategories = []string{"social", "streaming", "cloud", "gaming", "finance", "shopping", "news", "other"}

var allowedServiceCategorySet = map[string]struct{}{
	"social":    {},
	"streaming": {},
	"cloud":     {},
	"gaming":    {},
	"finance":   {},
	"shopping":  {},
	"news":      {},
	"other":     {},
}

type SubmitServiceError struct {
	Code                SubmitServiceErrorCode
	Field               string
	Message             string
	ExistingServiceSlug string
}

func (e *SubmitServiceError) Error() string {
	if e == nil {
		return "submit service failed"
	}

	return e.Message
}

type normalizedSubmitServiceInput struct {
	Name                 string
	NameLower            string
	Slug                 string
	Category             string
	HomepageURL          string
	HomepageURLLower     string
	NormalizedDomain     string
	SubmitterEmail       string
	SubmitterFingerprint string
}

func AllowedServiceCategories() []string {
	out := make([]string, len(allowedServiceCategories))
	copy(out, allowedServiceCategories)
	return out
}

func (s *Storage) SubmitService(ctx context.Context, input SubmitServiceInput) (*SubmitServiceResult, error) {
	normalized, err := normalizeSubmitServiceInput(input)
	if err != nil {
		return nil, err
	}

	result := &SubmitServiceResult{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existingService, err := findDuplicateService(tx, normalized)
		if err != nil {
			return err
		}
		if existingService != nil {
			return &SubmitServiceError{
				Code:                SubmitServiceErrorDuplicate,
				Field:               "homepage_url",
				Message:             "This service already exists in our tracker.",
				ExistingServiceSlug: existingService.Slug,
			}
		}

		hasSubmission, err := hasDuplicateSubmission(tx, normalized)
		if err != nil {
			return err
		}
		if hasSubmission {
			return &SubmitServiceError{
				Code:    SubmitServiceErrorDuplicate,
				Field:   "homepage_url",
				Message: "This service was already submitted recently.",
			}
		}

		service := structs.Service{
			Slug:        normalized.Slug,
			Name:        normalized.Name,
			Category:    normalized.Category,
			HomepageURL: normalized.HomepageURL,
			Active:      true,
		}
		if err := tx.Create(&service).Error; err != nil {
			if isUniqueConstraintError(err) {
				return &SubmitServiceError{Code: SubmitServiceErrorDuplicate, Field: "name", Message: "A matching service already exists."}
			}
			return err
		}

		cfg := DefaultProbeConfig(service.ID, service.HomepageURL, time.Now().UTC())
		if err := tx.Create(&cfg).Error; err != nil {
			if isUniqueConstraintError(err) {
				return &SubmitServiceError{Code: SubmitServiceErrorDuplicate, Field: "homepage_url", Message: "A probe config already exists for this service."}
			}
			return err
		}

		submission := structs.ServiceSubmission{
			ServiceID:            &service.ID,
			Name:                 normalized.Name,
			Slug:                 normalized.Slug,
			Description:          "",
			Category:             normalized.Category,
			HomepageURL:          normalized.HomepageURL,
			NormalizedDomain:     normalized.NormalizedDomain,
			SubmitterEmail:       normalized.SubmitterEmail,
			SubmitterFingerprint: normalized.SubmitterFingerprint,
			Status:               serviceSubmissionStatusPublishedUnverified,
		}
		if err := tx.Create(&submission).Error; err != nil {
			if isUniqueConstraintError(err) {
				return &SubmitServiceError{Code: SubmitServiceErrorDuplicate, Field: "homepage_url", Message: "This service was already submitted recently."}
			}
			return err
		}

		result.Service = service
		result.Submission = submission
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.invalidateServiceListCache(ctx)
	return result, nil
}

func findDuplicateService(tx *gorm.DB, input normalizedSubmitServiceInput) (*structs.Service, error) {
	var existing structs.Service
	canonicalHost := input.NormalizedDomain
	wwwCanonicalHost := "www." + input.NormalizedDomain
	err := tx.Model(&structs.Service{}).
		Select("id, slug, name").
		Where(`LOWER(slug) = ?
			OR LOWER(name) = ?
			OR LOWER(homepage_url) = ?
			OR REPLACE(SPLIT_PART(SPLIT_PART(REPLACE(REPLACE(LOWER(homepage_url), 'https://', ''), 'http://', ''), '/', 1), ':', 1), 'www.', '') = ?
			OR SPLIT_PART(SPLIT_PART(REPLACE(REPLACE(LOWER(homepage_url), 'https://', ''), 'http://', ''), '/', 1), ':', 1) = ?
			OR SPLIT_PART(SPLIT_PART(REPLACE(REPLACE(LOWER(homepage_url), 'https://', ''), 'http://', ''), '/', 1), ':', 1) = ?`,
			input.Slug,
			input.NameLower,
			input.HomepageURLLower,
			input.NormalizedDomain,
			canonicalHost,
			wwwCanonicalHost,
		).
		First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &existing, nil
}

func hasDuplicateSubmission(tx *gorm.DB, input normalizedSubmitServiceInput) (bool, error) {
	var count int64
	err := tx.Model(&structs.ServiceSubmission{}).
		Where("LOWER(slug) = ? OR LOWER(name) = ? OR LOWER(homepage_url) = ? OR normalized_domain = ?", input.Slug, input.NameLower, input.HomepageURLLower, input.NormalizedDomain).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func normalizeSubmitServiceInput(input SubmitServiceInput) (normalizedSubmitServiceInput, error) {
	name := strings.TrimSpace(input.Name)
	nameLength := utf8.RuneCountInString(name)
	if nameLength < 2 || nameLength > 100 {
		return normalizedSubmitServiceInput{}, &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "name", Message: "Service name must be between 2 and 100 characters."}
	}

	slug := slugify(name)
	if slug == "" {
		return normalizedSubmitServiceInput{}, &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "name", Message: "Service name must include letters or numbers."}
	}

	homepageURL, normalizedDomain, err := normalizeHomepageURL(input.HomepageURL)
	if err != nil {
		return normalizedSubmitServiceInput{}, err
	}

	submitterEmail, err := normalizeSubmitterEmail(input.SubmitterEmail)
	if err != nil {
		return normalizedSubmitServiceInput{}, err
	}

	fingerprint := strings.TrimSpace(input.SubmitterFingerprint)
	if fingerprint == "" {
		return normalizedSubmitServiceInput{}, &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "fingerprint", Message: "Unable to verify request identity."}
	}

	category := normalizeServiceCategory(input.Category)

	return normalizedSubmitServiceInput{
		Name:                 name,
		NameLower:            strings.ToLower(name),
		Slug:                 slug,
		Category:             category,
		HomepageURL:          homepageURL,
		HomepageURLLower:     strings.ToLower(homepageURL),
		NormalizedDomain:     normalizedDomain,
		SubmitterEmail:       submitterEmail,
		SubmitterFingerprint: fingerprint,
	}, nil
}

func normalizeServiceCategory(raw string) string {
	category := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := allowedServiceCategorySet[category]; !ok {
		return "other"
	}

	return category
}

func normalizeHomepageURL(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "homepage_url", Message: "Homepage URL is required."}
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "homepage_url", Message: "Homepage URL is invalid."}
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "homepage_url", Message: "Homepage URL must start with http or https."}
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "homepage_url", Message: "Homepage URL must include a host."}
	}

	normalizedDomain := strings.TrimPrefix(host, "www.")
	if normalizedDomain == "" || !strings.Contains(normalizedDomain, ".") {
		return "", "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "homepage_url", Message: "Homepage URL must include a valid domain."}
	}

	normalizedHost := host
	port := parsed.Port()
	if port != "" {
		if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			port = ""
		}
	}
	if port != "" {
		normalizedHost = fmt.Sprintf("%s:%s", host, port)
	}

	normalizedPath := strings.TrimSpace(parsed.Path)
	if normalizedPath != "" && normalizedPath != "/" {
		normalizedPath = path.Clean(normalizedPath)
		if !strings.HasPrefix(normalizedPath, "/") {
			normalizedPath = "/" + normalizedPath
		}
	} else {
		normalizedPath = ""
	}

	normalizedURL := (&url.URL{
		Scheme: scheme,
		Host:   normalizedHost,
		Path:   normalizedPath,
	}).String()

	return normalizedURL, normalizedDomain, nil
}

func normalizeSubmitterEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "email", Message: "Email is required."}
	}

	parsed, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "email", Message: "Email address is invalid."}
	}

	address := strings.ToLower(strings.TrimSpace(parsed.Address))
	if address == "" || utf8.RuneCountInString(address) > 254 {
		return "", &SubmitServiceError{Code: SubmitServiceErrorInvalid, Field: "email", Message: "Email address is invalid."}
	}

	return address, nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "unique constraint")
}
