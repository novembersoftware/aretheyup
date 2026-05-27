package storage

import (
	"testing"
	"time"
)

func TestNormalizeSubmitServiceInputSuccess(t *testing.T) {
	normalized, err := normalizeSubmitServiceInput(SubmitServiceInput{
		Name:                 "Acme Payments",
		Category:             "finance",
		HomepageURL:          "AcmePay.io",
		SubmitterEmail:       "Founder@AcmePay.io",
		SubmitterFingerprint: "fingerprint-1",
	})
	if err != nil {
		t.Fatalf("normalizeSubmitServiceInput() error = %v", err)
	}

	if normalized.Slug != "acme-payments" {
		t.Fatalf("Slug = %q, want acme-payments", normalized.Slug)
	}

	if normalized.HomepageURL != "https://acmepay.io" {
		t.Fatalf("HomepageURL = %q, want https://acmepay.io", normalized.HomepageURL)
	}

	if normalized.NormalizedDomain != "acmepay.io" {
		t.Fatalf("NormalizedDomain = %q, want acmepay.io", normalized.NormalizedDomain)
	}

	if normalized.SubmitterEmail != "founder@acmepay.io" {
		t.Fatalf("SubmitterEmail = %q, want founder@acmepay.io", normalized.SubmitterEmail)
	}
}

func TestNormalizeSubmitServiceInputMissingEmail(t *testing.T) {
	_, err := normalizeSubmitServiceInput(SubmitServiceInput{
		Name:                 "Acme Payments",
		Category:             "finance",
		HomepageURL:          "https://acmepay.io",
		SubmitterFingerprint: "fingerprint-1",
	})
	if err == nil {
		t.Fatal("expected missing email to fail")
	}

	submitErr, ok := err.(*SubmitServiceError)
	if !ok {
		t.Fatalf("error type = %T, want *SubmitServiceError", err)
	}

	if submitErr.Code != SubmitServiceErrorInvalid {
		t.Fatalf("Code = %q, want %q", submitErr.Code, SubmitServiceErrorInvalid)
	}

	if submitErr.Field != "email" {
		t.Fatalf("Field = %q, want email", submitErr.Field)
	}
}

func TestNormalizeServiceCategoryDefaultsToOther(t *testing.T) {
	if got := normalizeServiceCategory("infra"); got != "other" {
		t.Fatalf("normalizeServiceCategory(infra) = %q, want other", got)
	}
}

func TestNormalizeHomepageURLRejectsInvalidHost(t *testing.T) {
	_, _, err := normalizeHomepageURL("https://localhost")
	if err == nil {
		t.Fatal("expected invalid host to fail")
	}

	submitErr, ok := err.(*SubmitServiceError)
	if !ok {
		t.Fatalf("error type = %T, want *SubmitServiceError", err)
	}

	if submitErr.Code != SubmitServiceErrorInvalid {
		t.Fatalf("Code = %q, want %q", submitErr.Code, SubmitServiceErrorInvalid)
	}
}

func TestSubmitServiceUsesDefaultProbeConfigValues(t *testing.T) {
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	cfg := DefaultProbeConfig(7, " https://example.com/health ", now)

	if cfg.ServiceID != 7 {
		t.Fatalf("ServiceID = %d, want 7", cfg.ServiceID)
	}
	if !cfg.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if cfg.URL != "https://example.com/health" {
		t.Fatalf("URL = %q, want trimmed homepage URL", cfg.URL)
	}
	if cfg.Method != "GET" {
		t.Fatalf("Method = %q, want GET", cfg.Method)
	}
	if cfg.IntervalSeconds != 60 {
		t.Fatalf("IntervalSeconds = %d, want 60", cfg.IntervalSeconds)
	}
	if cfg.TimeoutSeconds != 10 {
		t.Fatalf("TimeoutSeconds = %d, want 10", cfg.TimeoutSeconds)
	}
	if cfg.ExpectedStatus != 200 {
		t.Fatalf("ExpectedStatus = %d, want 200", cfg.ExpectedStatus)
	}
	if !cfg.NextRunAt.Equal(now) {
		t.Fatalf("NextRunAt = %s, want %s", cfg.NextRunAt, now)
	}
}
