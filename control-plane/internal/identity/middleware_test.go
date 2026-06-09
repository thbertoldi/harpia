package identity

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	tasksv1 "github.com/harpia/control-plane/gen/harpia/tasks/v1"
)

func TestRequestContextInterceptorRejectsNoAuth(t *testing.T) {
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: uuid.New(), AllowDevAuth: true})
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	})

	_, err := interceptor.WrapUnary(next)(
		context.Background(),
		connect.NewRequest(&tasksv1.GetTaskRequest{TenantId: DevTenantAlias}),
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
	req := connect.NewRequest(&tasksv1.GetTaskRequest{TenantId: tenantB.String()})
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
	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, ok := RequestContextFrom(ctx)
		if !ok {
			return nil, errors.New("missing request context")
		}
		if rc.TenantID != tenantID {
			t.Fatalf("tenant ID = %s, want %s", rc.TenantID, tenantID)
		}
		if rc.UserID != "dev-user" {
			t.Fatalf("user ID = %q, want dev-user", rc.UserID)
		}
		return connect.NewResponse(&tasksv1.GetTaskResponse{}), nil
	})
	req := connect.NewRequest(&tasksv1.GetTaskRequest{TenantId: DevTenantAlias})
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
	req := connect.NewRequest(&tasksv1.GetTaskRequest{TenantId: DevTenantAlias})
	req.Header().Set("Authorization", "Bearer dev-token")
	req.Header().Set("X-Tenant-ID", DevTenantAlias)

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err == nil || connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expected unauthenticated, got %v", err)
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
	req := connect.NewRequest(&tasksv1.GetTaskRequest{TenantId: tenantB.String()})
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
	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, ok := RequestContextFrom(ctx)
		if !ok {
			return nil, errors.New("missing request context")
		}
		if rc.UserID != "user-1" {
			t.Fatalf("user ID = %q, want user-1", rc.UserID)
		}
		return connect.NewResponse(&tasksv1.GetTaskResponse{}), nil
	})
	req := connect.NewRequest(&tasksv1.GetTaskRequest{TenantId: tenantID.String()})
	req.Header().Set("Authorization", "Bearer valid-token")

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
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
