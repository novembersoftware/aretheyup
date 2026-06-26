package storage

import (
	"context"
	"testing"

	"github.com/novembersoftware/aretheyup/structs"
)

func TestServiceCacheKeysAreVersionedAndSpecific(t *testing.T) {
	if got, want := serviceListResponsesCacheKey(49, 0), "services:list:v3:limit:49:offset:0"; got != want {
		t.Fatalf("serviceListResponsesCacheKey() = %q, want %q", got, want)
	}

	if got, want := serviceDetailResponseCacheKey("example-service"), "services:detail:v3:slug:example-service"; got != want {
		t.Fatalf("serviceDetailResponseCacheKey() = %q, want %q", got, want)
	}
}

func TestShouldCacheServiceListOnlyAllowsDefaultFirstPage(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		offset int
		want   bool
	}{
		{name: "default first page", limit: 49, offset: 0, want: true},
		{name: "visible page size without lookahead", limit: 48, offset: 0, want: false},
		{name: "second page", limit: 49, offset: 48, want: false},
		{name: "search shape", limit: 48, offset: 12, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCacheServiceList(tt.limit, tt.offset); got != tt.want {
				t.Fatalf("shouldCacheServiceList(%d, %d) = %t, want %t", tt.limit, tt.offset, got, tt.want)
			}
		})
	}
}

func TestServiceCacheKeysForInvalidationIncludesDistinctDetailSlugs(t *testing.T) {
	got := serviceCacheKeysForInvalidation("old-slug", "new-slug", "old-slug", "")
	want := []string{
		serviceListResponsesCacheKey(defaultListCacheLimit, defaultListCacheOffset),
		serviceDetailResponseCacheKey("old-slug"),
		serviceDetailResponseCacheKey("new-slug"),
	}

	if len(got) != len(want) {
		t.Fatalf("serviceCacheKeysForInvalidation() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("serviceCacheKeysForInvalidation()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCacheHelpersTreatMissingRedisAsMiss(t *testing.T) {
	store := New(nil, nil)
	ctx := context.Background()

	if got, ok := store.GetCachedServiceListResponses(ctx, 49, 0); ok || got != nil {
		t.Fatalf("GetCachedServiceListResponses without Redis = (%+v, %t), want nil miss", got, ok)
	}

	if got, ok := store.GetCachedServiceDetailResponse(ctx, "example"); ok || got.ID != 0 {
		t.Fatalf("GetCachedServiceDetailResponse without Redis = (%+v, %t), want zero miss", got, ok)
	}

	store.SetCachedServiceListResponses(ctx, 49, 0, []structs.ServiceResponse{{ID: 1}})
	store.SetCachedServiceDetailResponse(ctx, "example", structs.ServiceDetailResponse{ID: 1})
	store.InvalidateServiceCaches(ctx, "example")
}
