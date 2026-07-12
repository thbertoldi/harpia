package identity

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	identityv1 "github.com/harpia/control-plane/gen/harpia/identity/v1"
)

func TestRequestContextInterceptorRejectsNoAuth(t *testing.T) {
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: uuid.New(), AllowDevAuth: true})
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	})

	_, err := interceptor.WrapUnary(next)(
		context.Background(),
		connect.NewRequest(&identityv1.GetTenantRequest{TenantId: DevTenantAlias}),
	)
	if err == nil || connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expected unauthenticated, got %v", err)
	}
}

func TestRequestContextInterceptorRejectsTenantMismatch(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: tenantA, AllowDevAuth: true})
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	})
	req := connect.NewRequest(&identityv1.GetTenantRequest{TenantId: tenantB.String()})
	req.Header().Set("Authorization", "Bearer dev-token")
	req.Header().Set("X-Tenant-ID", DevTenantAlias)

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err == nil || connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestRequestContextInterceptorPopulatesContext(t *testing.T) {
	tenantID := uuid.New()
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: tenantID, AllowDevAuth: true})
	wantUserID := stableUserUUIDFromSubject(DevUserSubject)
	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, ok := RequestContextFrom(ctx)
		if !ok {
			return nil, errors.New("missing request context")
		}
		if rc.TenantID != tenantID {
			t.Fatalf("tenant ID = %s, want %s", rc.TenantID, tenantID)
		}
		if rc.UserID != wantUserID {
			t.Fatalf("user ID = %q, want %q", rc.UserID, wantUserID)
		}
		if _, err := uuid.Parse(rc.UserID); err != nil {
			t.Fatalf("user ID is not a parseable UUID: %v", err)
		}
		if !rc.UsedDevAuth {
			t.Fatal("enabled dev token must be represented without exposing the token")
		}
		return connect.NewResponse(&identityv1.GetTenantResponse{}), nil
	})
	req := connect.NewRequest(&identityv1.GetTenantRequest{TenantId: DevTenantAlias})
	req.Header().Set("Authorization", "Bearer dev-token")
	req.Header().Set("X-Tenant-ID", DevTenantAlias)

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestRequestContextInterceptorRejectsDevTokenWhenDisabled(t *testing.T) {
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: uuid.New()})
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	})
	req := connect.NewRequest(&identityv1.GetTenantRequest{TenantId: DevTenantAlias})
	req.Header().Set("Authorization", "Bearer dev-token")
	req.Header().Set("X-Tenant-ID", DevTenantAlias)

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err == nil || connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expected unauthenticated, got %v", err)
	}
}

func TestRequestContextInterceptorRejectsNoTenantMembership(t *testing.T) {
	interceptor := NewRequestContextInterceptor(AuthOptions{
		Authenticator: fakeAuthenticator{user: AuthenticatedUser{Subject: "user-1", Email: "orphan@harpia.local"}},
		Memberships:   staticMemberships{"user-1": nil},
	})
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	})
	req := connect.NewRequest(&identityv1.GetTenantRequest{})
	req.Header().Set("Authorization", "Bearer valid-token")

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err == nil || connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestRequestContextInterceptorAutoSelectsSingleTenant(t *testing.T) {
	tenantID := uuid.New()
	interceptor := NewRequestContextInterceptor(AuthOptions{
		Authenticator: fakeAuthenticator{user: AuthenticatedUser{Subject: "user-1", Email: "admin@harpia.local"}},
		Memberships: staticMemberships{
			"user-1": {
				{TenantID: tenantID, Slug: "dev", Name: "Dev", Role: DefaultTenantMemberRole},
			},
		},
	})
	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, ok := RequestContextFrom(ctx)
		if !ok {
			return nil, errors.New("missing request context")
		}
		if rc.TenantID != tenantID {
			t.Fatalf("tenant ID = %s, want %s", rc.TenantID, tenantID)
		}
		return connect.NewResponse(&identityv1.GetCurrentUserResponse{}), nil
	})
	req := connect.NewRequest(&identityv1.GetCurrentUserRequest{})
	req.Header().Set("Authorization", "Bearer valid-token")

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestRequestContextInterceptorRequiresTenantMembership(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	interceptor := NewRequestContextInterceptor(AuthOptions{
		Authenticator: fakeAuthenticator{user: AuthenticatedUser{Subject: "user-1"}},
		Memberships: staticMemberships{
			"user-1": {
				{TenantID: tenantA, Slug: "tenant-a", Name: "Tenant A", Role: "member"},
			},
		},
	})
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	})
	req := connect.NewRequest(&identityv1.GetTenantRequest{TenantId: tenantB.String()})
	req.Header().Set("Authorization", "Bearer valid-token")
	req.Header().Set("X-Tenant-ID", tenantB.String())

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err == nil || connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestRequestContextInterceptorAuthorizesTenantFromMembership(t *testing.T) {
	tenantID := uuid.New()
	interceptor := NewRequestContextInterceptor(AuthOptions{
		Authenticator: fakeAuthenticator{user: AuthenticatedUser{Subject: "user-1"}},
		Memberships: staticMemberships{
			"user-1": {
				{TenantID: tenantID, Slug: "tenant-a", Name: "Tenant A", Role: "admin"},
			},
		},
	})
	wantUserID := stableUserUUIDFromSubject("user-1")
	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, ok := RequestContextFrom(ctx)
		if !ok {
			return nil, errors.New("missing request context")
		}
		if rc.UserID != wantUserID {
			t.Fatalf("user ID = %q, want %q (derived from subject 'user-1')", rc.UserID, wantUserID)
		}
		return connect.NewResponse(&identityv1.GetTenantResponse{}), nil
	})
	req := connect.NewRequest(&identityv1.GetTenantRequest{TenantId: tenantID.String()})
	req.Header().Set("Authorization", "Bearer valid-token")

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestStableUserUUIDFromSubject_PassesThroughUUIDs(t *testing.T) {
	in := "11111111-2222-3333-4444-555555555555"
	got := stableUserUUIDFromSubject(in)
	if got != in {
		t.Fatalf("UUID subject should pass through: got %q want %q", got, in)
	}
}

func TestStableUserUUIDFromSubject_HashesNonUUIDDeterministically(t *testing.T) {
	a := stableUserUUIDFromSubject(DevUserSubject)
	b := stableUserUUIDFromSubject(DevUserSubject)
	if a != b {
		t.Fatalf("dev subject should hash deterministically: %q vs %q", a, b)
	}
	if _, err := uuid.Parse(a); err != nil {
		t.Fatalf("derived value must parse as UUID: %v", err)
	}
	other := stableUserUUIDFromSubject("internal-service")
	if other == a {
		t.Fatalf("distinct subjects must hash to distinct UUIDs (collision on %q)", a)
	}
}

func TestStableUserUUIDFromSubject_EmptyInput(t *testing.T) {
	if got := stableUserUUIDFromSubject(""); got != "" {
		t.Fatalf("empty subject should yield empty string, got %q", got)
	}
	if got := stableUserUUIDFromSubject("   "); got != "" {
		t.Fatalf("whitespace-only subject should yield empty string, got %q", got)
	}
}

type staticMemberships map[string][]TenantMembership

func (s staticMemberships) ResolveMemberships(_ context.Context, user AuthenticatedUser) ([]TenantMembership, error) {
	return s[user.Subject], nil
}

type fakeAuthenticator struct {
	user AuthenticatedUser
	err  error
}

func (a fakeAuthenticator) AuthenticateBearer(context.Context, string) (AuthenticatedUser, error) {
	return a.user, a.err
}
