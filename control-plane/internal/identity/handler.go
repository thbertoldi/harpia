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
	return connect.NewResponse(&identityv1.GetCurrentUserResponse{
		User: &identityv1.User{
			Id:          "unknown",
			DisplayName: "Unknown User",
		},
		Tenants: []*identityv1.Tenant{},
	}), nil
}

func (h *IdentityHandler) ListTenants(ctx context.Context, req *connect.Request[identityv1.ListTenantsRequest]) (*connect.Response[identityv1.ListTenantsResponse], error) {
	return connect.NewResponse(&identityv1.ListTenantsResponse{
		Tenants:       []*identityv1.Tenant{},
		NextPageToken: "",
	}), nil
}

func (h *IdentityHandler) GetTenant(ctx context.Context, req *connect.Request[identityv1.GetTenantRequest]) (*connect.Response[identityv1.GetTenantResponse], error) {
	return connect.NewResponse(&identityv1.GetTenantResponse{
		Tenant: &identityv1.Tenant{
			Id:   req.Msg.TenantId,
			Name: "Unknown Tenant",
			Slug: "unknown",
		},
	}), nil
}
