package identity

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

type contextKey struct{}

type RequestContext struct {
	TenantID    uuid.UUID
	TenantAlias string
	UserID      string
	Roles       []string
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

func RequireTenant(ctx context.Context, requestedTenantID string) (uuid.UUID, error) {
	rc, err := RequireRequestContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	if requestedTenantID == "" ||
		requestedTenantID == rc.TenantID.String() ||
		(rc.TenantAlias != "" && requestedTenantID == rc.TenantAlias) {
		return rc.TenantID, nil
	}

	return uuid.Nil, connect.NewError(
		connect.CodePermissionDenied,
		fmt.Errorf("tenant %q is not available to the caller", requestedTenantID),
	)
}
