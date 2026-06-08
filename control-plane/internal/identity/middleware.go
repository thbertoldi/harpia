package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

const DevTenantAlias = "dev"

type AuthOptions struct {
	DevTenantID uuid.UUID
}

type tenantRequest interface {
	GetTenantId() string
}

type sessionCookie struct {
	Sub    string `json:"sub"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Tenant string `json:"tenant_id"`
}

type jwtClaims struct {
	Subject  string   `json:"sub"`
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
}

type RequestContextInterceptor struct {
	devTenantID uuid.UUID
}

func NewRequestContextInterceptor(opts AuthOptions) *RequestContextInterceptor {
	return &RequestContextInterceptor{devTenantID: opts.DevTenantID}
}

func (i *RequestContextInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rc, err := i.resolve(req.Header())
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
		rc, err := i.resolve(conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(WithRequestContext(ctx, rc), conn)
	}
}

func (i *RequestContextInterceptor) resolve(header http.Header) (RequestContext, error) {
	token, tokenOK := bearerToken(header.Get("Authorization"))
	session, sessionOK := parseSessionCookie(header.Get("Cookie"))
	if !tokenOK && !sessionOK {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing authorization"),
		)
	}

	userID := session.Sub
	roles := rolesFromSession(session)
	var claims jwtClaims
	if tokenOK && token != "dev-token" {
		claims = parseJWTClaims(token)
		if claims.Subject != "" {
			userID = claims.Subject
		}
		if len(claims.Roles) > 0 {
			roles = claims.Roles
		}
	}
	if token == "dev-token" && userID == "" {
		userID = "dev-user"
	}
	if userID == "" {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing user identity"),
		)
	}

	tenantRef := header.Get("X-Tenant-ID")
	if tenantRef == "" {
		tenantRef = session.Tenant
	}
	if tenantRef == "" {
		tenantRef = claims.TenantID
	}
	if tenantRef == "" {
		return RequestContext{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing tenant"),
		)
	}

	tenantID, alias, err := i.resolveTenant(tenantRef)
	if err != nil {
		return RequestContext{}, err
	}

	return RequestContext{
		TenantID:    tenantID,
		TenantAlias: alias,
		UserID:      userID,
		Roles:       roles,
	}, nil
}

func (i *RequestContextInterceptor) resolveTenant(ref string) (uuid.UUID, string, error) {
	if ref == DevTenantAlias && i.devTenantID != uuid.Nil {
		return i.devTenantID, DevTenantAlias, nil
	}

	tenantID, err := uuid.Parse(ref)
	if err != nil {
		return uuid.Nil, "", connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("invalid tenant %q", ref),
		)
	}

	return tenantID, "", nil
}

func (i *RequestContextInterceptor) authorizeRequestTenant(rc RequestContext, msg any) error {
	req, ok := msg.(tenantRequest)
	if !ok {
		return nil
	}
	_, err := RequireTenant(WithRequestContext(context.Background(), rc), req.GetTenantId())
	return err
}

func bearerToken(header string) (string, bool) {
	prefix, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func parseSessionCookie(cookieHeader string) (sessionCookie, bool) {
	if cookieHeader == "" {
		return sessionCookie{}, false
	}

	request := http.Request{Header: http.Header{"Cookie": []string{cookieHeader}}}
	cookie, err := request.Cookie("harpia_session")
	if err != nil {
		return sessionCookie{}, false
	}

	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		value = cookie.Value
	}

	var session sessionCookie
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return sessionCookie{}, false
	}
	return session, session.Sub != ""
}

func parseJWTClaims(token string) jwtClaims {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return jwtClaims{}
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtClaims{}
	}

	var claims jwtClaims
	_ = json.Unmarshal(payload, &claims)
	return claims
}

func rolesFromSession(session sessionCookie) []string {
	if session.Role == "" {
		return nil
	}
	return []string{session.Role}
}
