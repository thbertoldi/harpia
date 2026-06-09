package identity

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

type contextKey struct{}

type TenantMembership struct {
	TenantID uuid.UUID
	Slug     string
	Name     string
	Role     string
}

type RequestContext struct {
	TenantID    uuid.UUID
	TenantAlias string
	UserID      string
	Roles       []string
	Tenants     []TenantMembership
}

func WithRequestContext(ctx context.Context, rc RequestContext) context.Context {
	return context.WithValue(ctx, contextKey{}, rc)
}

func RequestContextFrom(ctx context.Context) (RequestContext, bool) {
	rc, ok := ctx.Value(contextKey{}).(RequestContext)
	return rc, ok
}

func RequireRequestContext(ctx context.Context) (RequestContext, error) {
	rc, ok := RequestContextFrom(ctx)
	if !ok {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing request context"),
		)
	}
	return rc, nil
}

func RequireSelectedTenant(ctx context.Context) (uuid.UUID, error) {
	rc, err := RequireRequestContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if rc.TenantID == uuid.Nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("missing tenant"),
		)
	}
	return rc.TenantID, nil
}

func RequireTenant(ctx context.Context, requestedTenantID string) (uuid.UUID, error) {
	rc, err := RequireRequestContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	if requestedTenantID == "" {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("missing tenant"),
		)
	}

	if requestedTenantID == rc.TenantID.String() ||
		(rc.TenantAlias != "" && requestedTenantID == rc.TenantAlias) {
		return rc.TenantID, nil
	}

	for _, tenant := range rc.Tenants {
		if requestedTenantID == tenant.TenantID.String() ||
			(tenant.Slug != "" && requestedTenantID == tenant.Slug) {
			return tenant.TenantID, nil
		}
	}

	return uuid.Nil, connect.NewError(
		connect.CodePermissionDenied,
		fmt.Errorf("tenant %q is not available to the caller", requestedTenantID),
	)
}
