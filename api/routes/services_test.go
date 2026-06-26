package routes

import (
	"testing"

	"github.com/novembersoftware/aretheyup/api/middleware"
	"github.com/novembersoftware/aretheyup/structs"
)

func TestMergeServiceDetailRequesterStateOverridesCachedReportFields(t *testing.T) {
	cached := structs.ServiceDetailResponse{
		ID:                  42,
		Slug:                "example",
		CanReport:           true,
		ReportRetryAfterSec: 0,
	}

	got := mergeServiceDetailRequesterState(cached, middleware.ReportRateLimitState{
		CanReport:         false,
		RetryAfterSeconds: 91,
	})

	if got.ID != cached.ID || got.Slug != cached.Slug {
		t.Fatalf("merge changed neutral service fields: got %+v, cached %+v", got, cached)
	}
	if got.CanReport {
		t.Fatal("CanReport stayed true from cached payload, want live false")
	}
	if got.ReportRetryAfterSec != 91 {
		t.Fatalf("ReportRetryAfterSec = %d, want live retry value 91", got.ReportRetryAfterSec)
	}
}
