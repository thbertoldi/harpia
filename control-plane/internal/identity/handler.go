package identity

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	identityv1 "github.com/harpia/control-plane/gen/harpia/identity/v1"
)

type IdentityHandler struct{}

func NewIdentityHandler() *IdentityHandler {
	return &IdentityHandler{}
}

func (h *IdentityHandler) GetCurrentUser(ctx context.Context, req *connect.Request[identityv1.GetCurrentUserRequest]) (*connect.Response[identityv1.GetCurrentUserResponse], error) {
	rc, err := RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	tenant := selectedTenant(rc)
	return connect.NewResponse(&identityv1.GetCurrentUserResponse{
		User: &identityv1.User{
			Id:          rc.UserID,
			TenantId:    tenantIDFromTenant(tenant),
			DisplayName: rc.UserID,
			Roles:       rc.Roles,
		},
		Tenants: tenantsFromContext(rc),
	}), nil
}

func (h *IdentityHandler) ListTenants(ctx context.Context, req *connect.Request[identityv1.ListTenantsRequest]) (*connect.Response[identityv1.ListTenantsResponse], error) {
	rc, err := RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&identityv1.ListTenantsResponse{
		Tenants:       tenantsFromContext(rc),
		NextPageToken: "",
	}), nil
}

func (h *IdentityHandler) GetTenant(ctx context.Context, req *connect.Request[identityv1.GetTenantRequest]) (*connect.Response[identityv1.GetTenantResponse], error) {
	if _, err := RequireTenant(ctx, req.Msg.TenantId); err != nil {
		return nil, err
	}
	rc, _ := RequestContextFrom(ctx)
	return connect.NewResponse(&identityv1.GetTenantResponse{
		Tenant: tenantByID(rc, req.Msg.TenantId),
	}), nil
}

func tenantsFromContext(rc RequestContext) []*identityv1.Tenant {
	tenants := make([]*identityv1.Tenant, 0, len(rc.Tenants))
	for _, membership := range rc.Tenants {
		name := membership.Name
		if name == "" {
			name = "Tenant"
		}
		slug := membership.Slug
		if slug == "" {
			slug = membership.TenantID.String()
		}
		tenants = append(tenants, &identityv1.Tenant{
			Id:       membership.TenantID.String(),
			Name:     name,
			Slug:     slug,
			ThemeKey: themeKeyFromMembership(membership),
		})
	}
	return tenants
}

func selectedTenant(rc RequestContext) *identityv1.Tenant {
	if rc.TenantID == uuid.Nil && len(rc.Tenants) > 0 {
		return tenantFromMembership(rc.Tenants[0])
	}
	for _, membership := range rc.Tenants {
		if membership.TenantID == rc.TenantID {
			return tenantFromMembership(membership)
		}
	}
	return nil
}

func tenantByID(rc RequestContext, tenantID string) *identityv1.Tenant {
	for _, membership := range rc.Tenants {
		if tenantID == membership.TenantID.String() || (membership.Slug != "" && tenantID == membership.Slug) {
			return tenantFromMembership(membership)
		}
	}
	return selectedTenant(rc)
}

func tenantFromMembership(membership TenantMembership) *identityv1.Tenant {
	name := membership.Name
	if name == "" {
		name = "Tenant"
	}
	slug := membership.Slug
	if slug == "" {
		slug = membership.TenantID.String()
	}
	return &identityv1.Tenant{
		Id:       membership.TenantID.String(),
		Name:     name,
		Slug:     slug,
		ThemeKey: themeKeyFromMembership(membership),
	}
}

func themeKeyFromMembership(membership TenantMembership) *string {
	if membership.ThemeKey == "" {
		return nil
	}
	key := membership.ThemeKey
	return &key
}

func tenantIDFromTenant(tenant *identityv1.Tenant) string {
	if tenant == nil {
		return ""
	}
	return tenant.Id
}
