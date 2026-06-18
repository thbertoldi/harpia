package identity

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

func TestInternalServiceInterceptorAcceptsConfiguredToken(t *testing.T) {
	t.Parallel()

	interceptor := NewInternalServiceInterceptor(InternalServiceOptions{
		Token: "service-secret",
	})
	tenantID := uuid.New()
	called := false
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		if !IsInternalServiceCaller(ctx) {
			t.Fatal("expected internal service caller marker")
		}
		rc, ok := RequestContextFrom(ctx)
		if !ok || rc.TenantID != tenantID {
			t.Fatalf("tenant context = %+v, want %s", rc, tenantID)
		}
		return nil, nil
	}

	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer service-secret")
	req.Header().Set("X-Tenant-ID", tenantID.String())

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if err != nil {
		t.Fatalf("WrapUnary: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be invoked")
	}
}

func TestInternalServiceInterceptorRejectsUserDevToken(t *testing.T) {
	t.Parallel()

	interceptor := NewInternalServiceInterceptor(InternalServiceOptions{
		AllowDevAuth: true,
	})
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer dev-token")
	req.Header().Set("X-Tenant-ID", uuid.New().String())

	_, err := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})(context.Background(), req)
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("code = %v, want %v", got, connect.CodeUnauthenticated)
	}
}
