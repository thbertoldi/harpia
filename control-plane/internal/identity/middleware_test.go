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
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: uuid.New()})
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
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: tenantA})
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
	interceptor := NewRequestContextInterceptor(AuthOptions{DevTenantID: tenantID})
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
