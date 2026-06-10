package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/identity"
)

func TestTenantObjectStorePrefixesPathsFromRequestContext(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	store := NewTenantObjectStore("harpia")

	got, err := store.ObjectPath(contextWithTenant(tenantID), "/tasks/abc/result.json")
	if err != nil {
		t.Fatalf("ObjectPath returned error: %v", err)
	}

	want := "tenant/11111111-1111-1111-1111-111111111111/tasks/abc/result.json"
	if got != want {
		t.Fatalf("ObjectPath = %q, want %q", got, want)
	}
}

func TestTenantObjectStoreRejectsCrossTenantRawPaths(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherTenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	store := NewTenantObjectStore("harpia")

	_, err := store.ObjectPath(contextWithTenant(tenantID), "tenant/"+otherTenantID.String()+"/tasks/abc/result.json")
	if !errors.Is(err, ErrTenantBoundary) {
		t.Fatalf("ObjectPath error = %v, want ErrTenantBoundary", err)
	}
}

func TestTenantObjectStoreAssertTenantPathRejectsOtherTenant(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherTenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	store := NewTenantObjectStore("harpia")

	err := store.AssertTenantPath(contextWithTenant(tenantID), "tenant/"+otherTenantID.String()+"/tasks/abc/result.json")
	if !errors.Is(err, ErrTenantBoundary) {
		t.Fatalf("AssertTenantPath error = %v, want ErrTenantBoundary", err)
	}
}

func TestTenantObjectStoreRequiresRequestContext(t *testing.T) {
	store := NewTenantObjectStore("harpia")

	_, err := store.ObjectPath(context.Background(), "tasks/abc/result.json")
	if err == nil {
		t.Fatal("ObjectPath returned nil error without request context")
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
