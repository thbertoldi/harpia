package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/identity"
)

func TestTenantStorePrefixesKeysFromRequestContext(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	store := NewTenantStore(&Client{})

	got, err := store.tenantKey(contextWithTenant(tenantID), "task:abc")
	if err != nil {
		t.Fatalf("tenantKey returned error: %v", err)
	}

	want := "t:11111111-1111-1111-1111-111111111111:task:abc"
	if got != want {
		t.Fatalf("tenantKey = %q, want %q", got, want)
	}
}

func TestTenantStoreRejectsCrossTenantRawKeys(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherTenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	store := NewTenantStore(&Client{})

	_, err := store.tenantKey(contextWithTenant(tenantID), "t:"+otherTenantID.String()+":task:abc")
	if !errors.Is(err, ErrTenantBoundary) {
		t.Fatalf("tenantKey error = %v, want ErrTenantBoundary", err)
	}
}

func TestTenantStoreRequiresRequestContext(t *testing.T) {
	store := NewTenantStore(&Client{})

	_, err := store.tenantKey(context.Background(), "task:abc")
	if err == nil {
		t.Fatal("tenantKey returned nil error without request context")
	}
}

func contextWithTenant(tenantID uuid.UUID) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   "user-1",
		Tenants: []identity.TenantMembership{
			{TenantID: tenantID, Slug: "tenant-a", Role: "admin"},
		},
	})
}
