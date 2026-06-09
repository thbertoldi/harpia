package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

const DevTenantAlias = "dev"

type AuthOptions struct {
	DevTenantID  uuid.UUID
	AllowDevAuth bool
	UserInfoURL  string
	Memberships  MembershipResolver
	HTTPClient   *http.Client
}

type tenantRequest interface {
	GetTenantId() string
}

type userInfo struct {
	Subject string `json:"sub"`
}

type RequestContextInterceptor struct {
	devTenantID  uuid.UUID
	allowDevAuth bool
	userInfoURL  string
	memberships  MembershipResolver
	httpClient   *http.Client
}

func NewRequestContextInterceptor(opts AuthOptions) *RequestContextInterceptor {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &RequestContextInterceptor{
		devTenantID:  opts.DevTenantID,
		allowDevAuth: opts.AllowDevAuth,
		userInfoURL:  strings.TrimRight(opts.UserInfoURL, "/"),
		memberships:  opts.Memberships,
		httpClient:   client,
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

	user, err := i.authenticateBearer(ctx, token)
	if err != nil {
		return RequestContext{}, err
	}
	if i.memberships == nil {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("tenant membership resolver is not configured"),
		)
	}
	memberships, err := i.memberships.ListMemberships(ctx, user.Subject)
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

func (i *RequestContextInterceptor) authenticateBearer(ctx context.Context, token string) (userInfo, error) {
	if i.userInfoURL == "" {
		return userInfo{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("userinfo endpoint is not configured"),
		)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.userInfoURL+"/oidc/v1/userinfo", nil)
	if err != nil {
		return userInfo{}, connect.NewError(connect.CodeInternal, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return userInfo{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return userInfo{}, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("userinfo rejected token with status %d", resp.StatusCode),
		)
	}

	var info userInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return userInfo{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if info.Subject == "" {
		return userInfo{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing user identity"),
		)
	}
	return info, nil
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
