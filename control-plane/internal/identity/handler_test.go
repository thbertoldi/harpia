package identity

import (
	"testing"

	"github.com/google/uuid"
)

func TestThemeKeyFromMembership(t *testing.T) {
	t.Parallel()

	if themeKeyFromMembership(TenantMembership{}) != nil {
		t.Fatal("expected nil theme key for empty membership")
	}

	got := themeKeyFromMembership(TenantMembership{
		TenantID: uuid.New(),
		ThemeKey: "tenant-base",
	})
	if got == nil || *got != "tenant-base" {
		t.Fatalf("theme key = %#v, want tenant-base", got)
	}
}

func TestTenantFromMembershipIncludesThemeKey(t *testing.T) {
	t.Parallel()

	tenant := tenantFromMembership(TenantMembership{
		TenantID: uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		Slug:     "acme",
		Name:     "Acme",
		ThemeKey: "aiuna",
	})

	if tenant.GetThemeKey() != "aiuna" {
		t.Fatalf("theme key = %q, want aiuna", tenant.GetThemeKey())
	}
}
