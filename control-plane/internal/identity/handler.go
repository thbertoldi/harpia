package identity

import (
	"context"

	"connectrpc.com/connect"

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
	tenant := tenantFromContext(rc)
	return connect.NewResponse(&identityv1.GetCurrentUserResponse{
		User: &identityv1.User{
			Id:          rc.UserID,
			TenantId:    tenant.Id,
			DisplayName: rc.UserID,
			Roles:       rc.Roles,
		},
		Tenants: []*identityv1.Tenant{tenant},
	}), nil
}

func (h *IdentityHandler) ListTenants(ctx context.Context, req *connect.Request[identityv1.ListTenantsRequest]) (*connect.Response[identityv1.ListTenantsResponse], error) {
	rc, err := RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&identityv1.ListTenantsResponse{
		Tenants:       []*identityv1.Tenant{tenantFromContext(rc)},
		NextPageToken: "",
	}), nil
}

func (h *IdentityHandler) GetTenant(ctx context.Context, req *connect.Request[identityv1.GetTenantRequest]) (*connect.Response[identityv1.GetTenantResponse], error) {
	if _, err := RequireTenant(ctx, req.Msg.TenantId); err != nil {
		return nil, err
	}
	rc, _ := RequestContextFrom(ctx)
	return connect.NewResponse(&identityv1.GetTenantResponse{
		Tenant: tenantFromContext(rc),
	}), nil
}

func tenantFromContext(rc RequestContext) *identityv1.Tenant {
	slug := rc.TenantAlias
	if slug == "" {
		slug = rc.TenantID.String()
	}

	name := "Tenant"
	if slug == DevTenantAlias {
		name = "Dev Workspace"
	}

	return &identityv1.Tenant{
		Id:   rc.TenantID.String(),
		Name: name,
		Slug: slug,
	}
}
