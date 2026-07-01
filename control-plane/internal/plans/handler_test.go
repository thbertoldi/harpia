package plans

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
)

// TestCreatePlanConfiguration_RequiresThreadID guards the Path B contract: a
// plan configuration must be created against an owning thread. thread_id is
// validated before any repository access, so this exercises the handler without
// a database.
func TestCreatePlanConfiguration_RequiresThreadID(t *testing.T) {
	tenantID := uuid.New()
	h := &PlanHandler{}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})

	_, err := h.CreatePlanConfiguration(ctx, connect.NewRequest(&plansv1.CreatePlanConfigurationRequest{
		TenantId:       tenantID.String(),
		PlanTemplateId: uuid.New().String(),
		ThreadId:       "",
	}))
	if err == nil {
		t.Fatal("expected error for missing thread_id, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", got)
	}
}
