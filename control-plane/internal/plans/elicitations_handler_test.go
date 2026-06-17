package plans

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

func TestAuthorizeElicitationResponse(t *testing.T) {
	overseer := uuid.New()
	elicitation := &Elicitation{OverseerUserID: uuid.NullUUID{UUID: overseer, Valid: true}}

	cases := []struct {
		name    string
		rc      identity.RequestContext
		wantErr bool
	}{
		{name: "assigned overseer", rc: identity.RequestContext{UserID: overseer.String(), Roles: []string{"Overseer"}}},
		{name: "leader", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"Leader"}}},
		{name: "admin alias", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"admin"}}},
		{name: "other overseer", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"Overseer"}}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := authorizeElicitationResponse(tc.rc, elicitation)
			if tc.wantErr && err == nil {
				t.Fatal("expected permission denied")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeElicitationResponse(t *testing.T) {
	if _, _, err := normalizeElicitationResponse("", ""); err == nil {
		t.Fatal("expected error when both payload and text empty")
	}
	if _, _, err := normalizeElicitationResponse("{not json", ""); err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
	payload, text, err := normalizeElicitationResponse(`{"tone":"warm"}`, " hi ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hi" {
		t.Fatalf("text = %q, want trimmed", text)
	}
	if string(payload) != `{"tone":"warm"}` {
		t.Fatalf("payload = %s", string(payload))
	}
}

func TestRespondToElicitationRejectsNonPending(t *testing.T) {
	tenantID := uuid.New()
	id := uuid.New()
	store := &fakeElicitationStore{elicitation: &Elicitation{
		ID:       id,
		TenantID: tenantID,
		Status:   ElicitationStatusAnswered,
	}}
	handler := &PlanHandler{elicitations: store, signaler: &fakeSignaler{deliver: func(workflow.ElicitationResponseSignal) {}}}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   uuid.New().String(),
		Roles:    []string{"Leader"},
	})
	_, err := handler.RespondToElicitation(ctx, connect.NewRequest(&plansv1.RespondToElicitationRequest{
		TenantId:      tenantID.String(),
		ElicitationId: id.String(),
		ResponseText:  "late answer",
	}))
	if err == nil {
		t.Fatal("expected failed precondition for non-pending elicitation")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", connect.CodeOf(err))
	}
}
