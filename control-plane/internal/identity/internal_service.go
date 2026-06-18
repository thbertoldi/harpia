package identity

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

type internalServiceKey struct{}

// InternalServiceOptions configures trusted runtime authentication for
// control-plane endpoints that must not be reachable with user credentials.
type InternalServiceOptions struct {
	Token        string
	AllowDevAuth bool
}

// InternalServiceInterceptor authenticates agent-runtime and other trusted
// callers via a dedicated service token, separate from user bearer auth.
type InternalServiceInterceptor struct {
	token        string
	allowDevAuth bool
}

const devInternalServiceToken = "dev-internal-token"

// NewInternalServiceInterceptor validates HARPIA_INTERNAL_AUTH_TOKEN (or the
// dev-only fallback when AllowDevAuth is enabled).
func NewInternalServiceInterceptor(opts InternalServiceOptions) *InternalServiceInterceptor {
	return &InternalServiceInterceptor{
		token:        opts.Token,
		allowDevAuth: opts.AllowDevAuth,
	}
}

func (i *InternalServiceInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		token, ok := bearerToken(req.Header().Get("Authorization"))
		if !ok {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("missing internal service authorization"),
			)
		}
		if !i.validToken(token) {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("invalid internal service authorization"),
			)
		}

		tenantRef := req.Header().Get("X-Tenant-ID")
		if tenantRef == "" {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				errors.New("missing X-Tenant-ID header"),
			)
		}
		tenantID, err := uuid.Parse(tenantRef)
		if err != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid X-Tenant-ID %q", tenantRef),
			)
		}

		ctx = WithRequestContext(ctx, RequestContext{
			TenantID:    tenantID,
			TenantAlias: tenantRef,
			UserID:      "internal-service",
			Roles:       nil,
		})
		ctx = WithInternalServiceCaller(ctx)
		return next(ctx, req)
	}
}

func (i *InternalServiceInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *InternalServiceInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func (i *InternalServiceInterceptor) validToken(token string) bool {
	if i.token != "" && token == i.token {
		return true
	}
	return i.allowDevAuth && token == devInternalServiceToken
}

// WithInternalServiceCaller marks the context as authenticated by a trusted
// internal runtime caller.
func WithInternalServiceCaller(ctx context.Context) context.Context {
	return context.WithValue(ctx, internalServiceKey{}, true)
}

// IsInternalServiceCaller reports whether the request reached the handler via
// the internal service interceptor.
func IsInternalServiceCaller(ctx context.Context) bool {
	ok, _ := ctx.Value(internalServiceKey{}).(bool)
	return ok
}
