package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

const DevTenantAlias = "dev"

type AuthOptions struct {
	DevTenantID   uuid.UUID
	AllowDevAuth  bool
	Authenticator BearerAuthenticator
	Memberships   MembershipResolver
}

type tenantRequest interface {
	GetTenantId() string
}

type RequestContextInterceptor struct {
	devTenantID   uuid.UUID
	allowDevAuth  bool
	authenticator BearerAuthenticator
	memberships   MembershipResolver
}

func NewRequestContextInterceptor(opts AuthOptions) *RequestContextInterceptor {
	return &RequestContextInterceptor{
		devTenantID:   opts.DevTenantID,
		allowDevAuth:  opts.AllowDevAuth,
		authenticator: opts.Authenticator,
		memberships:   opts.Memberships,
	}
}

func (i *RequestContextInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, err := i.resolve(ctx, req.Header())
		if err != nil {
			return nil, err
		}
		if err := i.authorizeRequestTenant(rc, req.Any()); err != nil {
			return nil, err
		}
		return next(WithRequestContext(ctx, rc), req)
	}
}

func (i *RequestContextInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *RequestContextInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		rc, err := i.resolve(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(WithRequestContext(ctx, rc), &authorizedStreamingConn{
			StreamingHandlerConn: conn,
			interceptor:          i,
			rc:                   rc,
		})
	}
}

func (i *RequestContextInterceptor) resolve(ctx context.Context, header http.Header) (RequestContext, error) {
	token, tokenOK := bearerToken(header.Get("Authorization"))
	if !tokenOK {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing authorization"),
		)
	}

	if token == "dev-token" {
		return i.resolveDevContext(header.Get("X-Tenant-ID"))
	}

	if i.authenticator == nil {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("bearer authenticator is not configured"),
		)
	}
	user, err := i.authenticator.AuthenticateBearer(ctx, token)
	if err != nil {
		return RequestContext{}, err
	}
	if i.memberships == nil {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("tenant membership resolver is not configured"),
		)
	}
	memberships, err := i.memberships.ResolveMemberships(ctx, user)
	if err != nil {
		return RequestContext{}, connect.NewError(connect.CodeInternal, err)
	}
	if len(memberships) == 0 {
		return RequestContext{}, connect.NewError(
			connect.CodePermissionDenied,
			errors.New("user is not assigned to any tenant"),
		)
	}

	selected, selectedOK, err := selectTenant(header.Get("X-Tenant-ID"), memberships)
	if err != nil {
		return RequestContext{}, err
	}
	roles := []string(nil)
	if selectedOK && selected.Role != "" {
		roles = []string{selected.Role}
	}

	return RequestContext{
		TenantID:    selected.TenantID,
		TenantAlias: selected.Slug,
		UserID:      user.Subject,
		Roles:       roles,
		Tenants:     memberships,
	}, nil
}

func (i *RequestContextInterceptor) resolveDevContext(tenantRef string) (RequestContext, error) {
	if !i.allowDevAuth || i.devTenantID == uuid.Nil {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("dev token is not enabled"),
		)
	}

	membership := TenantMembership{
		TenantID: i.devTenantID,
		Slug:     DevTenantAlias,
		Name:     "Dev Workspace",
		Role:     "admin",
	}
	if tenantRef == "" {
		tenantRef = DevTenantAlias
	}
	selected, _, err := selectTenant(tenantRef, []TenantMembership{membership})
	if err != nil {
		return RequestContext{}, err
	}
	return RequestContext{
		TenantID:    selected.TenantID,
		TenantAlias: selected.Slug,
		UserID:      "dev-user",
		Roles:       []string{selected.Role},
		Tenants:     []TenantMembership{membership},
	}, nil
}

func selectTenant(ref string, memberships []TenantMembership) (TenantMembership, bool, error) {
	if ref == "" {
		if len(memberships) == 1 {
			return memberships[0], true, nil
		}
		return TenantMembership{}, false, nil
	}

	for _, membership := range memberships {
		if ref == membership.TenantID.String() || (membership.Slug != "" && ref == membership.Slug) {
			return membership, true, nil
		}
	}

	if _, err := uuid.Parse(ref); err != nil && ref != DevTenantAlias {
		return TenantMembership{}, false, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("invalid tenant %q", ref),
		)
	}
	return TenantMembership{}, false, connect.NewError(
		connect.CodePermissionDenied,
		fmt.Errorf("tenant %q is not available to the caller", ref),
	)
}

func (i *RequestContextInterceptor) authorizeRequestTenant(rc RequestContext, msg any) error {
	req, ok := msg.(tenantRequest)
	if !ok {
		return nil
	}
	_, err := RequireTenant(WithRequestContext(context.Background(), rc), req.GetTenantId())
	return err
}

type authorizedStreamingConn struct {
	connect.StreamingHandlerConn
	interceptor *RequestContextInterceptor
	rc          RequestContext
}

func (c *authorizedStreamingConn) Receive(msg any) error {
	if err := c.StreamingHandlerConn.Receive(msg); err != nil {
		return err
	}
	return c.interceptor.authorizeRequestTenant(c.rc, msg)
}

func bearerToken(header string) (string, bool) {
	prefix, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
