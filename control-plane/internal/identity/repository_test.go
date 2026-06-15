package identity

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/database"
)

func TestMembershipRepositoryAutoProvisionsDefaultTenant(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run membership repository integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	defaultTenantID, _, err := database.EnsureDevData(ctx, pool)
	if err != nil {
		t.Fatalf("ensure dev data: %v", err)
	}

	repo := NewMembershipRepository(pool, MembershipRepositoryOptions{
		DefaultTenantID:            defaultTenantID,
		AutoProvisionDefaultTenant: true,
	})

	user := AuthenticatedUser{
		Subject: "zitadel-admin-subject",
		Email:   "admin@harpia.local",
		Name:    "Harpia Admin",
	}

	memberships, err := repo.ResolveMemberships(ctx, user)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("memberships = %#v, want one default tenant", memberships)
	}
	if memberships[0].TenantID != defaultTenantID {
		t.Fatalf("tenant ID = %s, want %s", memberships[0].TenantID, defaultTenantID)
	}
	if memberships[0].Role != DefaultTenantMemberRole {
		t.Fatalf("role = %q, want %q", memberships[0].Role, DefaultTenantMemberRole)
	}

	repeated, err := repo.ResolveMemberships(ctx, user)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if len(repeated) != 1 {
		t.Fatalf("repeated memberships = %#v, want one without duplicates", repeated)
	}
}

func TestMembershipRepositoryDoesNotAutoProvisionWhenDisabled(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run membership repository integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	defaultTenantID, _, err := database.EnsureDevData(ctx, pool)
	if err != nil {
		t.Fatalf("ensure dev data: %v", err)
	}

	repo := NewMembershipRepository(pool, MembershipRepositoryOptions{
		DefaultTenantID:            defaultTenantID,
		AutoProvisionDefaultTenant: false,
	})

	memberships, err := repo.ResolveMemberships(ctx, AuthenticatedUser{
		Subject: uuid.NewString(),
		Email:   "orphan@harpia.local",
		Name:    "Orphan User",
	})
	if err != nil {
		t.Fatalf("resolve memberships: %v", err)
	}
	if len(memberships) != 0 {
		t.Fatalf("memberships = %#v, want none when auto-provision is disabled", memberships)
	}
}
