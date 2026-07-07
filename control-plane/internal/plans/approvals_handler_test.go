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

func TestAuthorizeApprovalResponse(t *testing.T) {
	cases := []struct {
		name    string
		rc      identity.RequestContext
		wantErr bool
	}{
		{name: "leader", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"Leader"}}},
		{name: "overseer", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"Overseer"}}},
		{name: "admin alias", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"admin"}}},
		{name: "member", rc: identity.RequestContext{UserID: uuid.New().String(), Roles: []string{"Member"}}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := authorizeApprovalResponse(tc.rc)
			if tc.wantErr && err == nil {
				t.Fatal("expected permission denied")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateApprovalDecision(t *testing.T) {
	if err := validateApprovalDecision(true, ""); err != nil {
		t.Fatalf("approve without reason should be valid: %v", err)
	}
	if err := validateApprovalDecision(false, "not ready"); err != nil {
		t.Fatalf("reject with reason should be valid: %v", err)
	}
	if err := validateApprovalDecision(false, ""); err == nil {
		t.Fatal("reject without reason should fail")
	}
}

func TestRespondToApprovalRequestRejectsNonPending(t *testing.T) {
	tenantID := uuid.New()
	approvalID := "approval-1"
	store := &fakeApprovalStore{approval: &PlanApprovalRequest{
		ID:       approvalID,
		TenantID: tenantID,
		Status:   ApprovalRequestStatusApproved,
	}}
	handler := &PlanHandler{
		approvals:        store,
		approvalSignaler: &fakeApprovalSignaler{deliver: func(workflow.ApprovalDecisionSignal) {}},
	}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   uuid.New().String(),
		Roles:    []string{"Leader"},
	})
	_, err := handler.RespondToApprovalRequest(ctx, connect.NewRequest(&plansv1.RespondToApprovalRequestRequest{
		TenantId:          tenantID.String(),
		ApprovalRequestId: approvalID,
		Approved:          true,
	}))
	if err == nil {
		t.Fatal("expected failed precondition for non-pending approval")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", connect.CodeOf(err))
	}
}

type fakeApprovalStore struct {
	approval *PlanApprovalRequest
	decided  bool
}

func (f *fakeApprovalStore) GetPlanApprovalRequest(_ context.Context, _ uuid.UUID, approvalID string) (*PlanApprovalRequest, error) {
	if f.approval == nil || f.approval.ID != approvalID {
		return nil, connect.NewError(connect.CodeNotFound, errFakeNotFound)
	}
	clone := *f.approval
	return &clone, nil
}

func (f *fakeApprovalStore) ListPlanApprovalRequests(_ context.Context, _ uuid.UUID, _ ApprovalFilters) ([]*PlanApprovalRequest, error) {
	if f.approval == nil {
		return nil, nil
	}
	return []*PlanApprovalRequest{f.approval}, nil
}

func (f *fakeApprovalStore) MarkApprovalDecided(_ context.Context, _ uuid.UUID, approvalID string, approved bool, reason string) (*PlanApprovalRequest, error) {
	f.decided = true
	updated := *f.approval
	if approved {
		updated.Status = ApprovalRequestStatusApproved
	} else {
		updated.Status = ApprovalRequestStatusRejected
	}
	updated.DecisionReason = reason
	f.approval = &updated
	return &updated, nil
}

type fakeApprovalSignaler struct {
	deliver func(signal workflow.ApprovalDecisionSignal)
	called  bool
}

func (f *fakeApprovalSignaler) SignalPlanApprovalDecision(_ context.Context, _ string, _ string, signal workflow.ApprovalDecisionSignal) error {
	f.called = true
	f.deliver(signal)
	return nil
}

func TestRespondToApprovalRequestSignalsWorkflow(t *testing.T) {
	tenantID := uuid.New()
	planExecutionID := uuid.New()
	planConfigurationID := uuid.New()
	threadID := uuid.New()
	stepExecutionID := uuid.New()
	approvalID := "approval-1"
	store := &fakeApprovalStore{approval: &PlanApprovalRequest{
		ID:                  approvalID,
		TenantID:            tenantID,
		PlanExecutionID:     planExecutionID,
		PlanConfigurationID: planConfigurationID,
		ThreadID:            threadID,
		StepExecutionID:     stepExecutionID,
		Status:              ApprovalRequestStatusPending,
	}}
	var received workflow.ApprovalDecisionSignal
	signaler := &fakeApprovalSignaler{deliver: func(signal workflow.ApprovalDecisionSignal) {
		received = signal
	}}
	handler := &PlanHandler{approvals: store, approvalSignaler: signaler}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   uuid.New().String(),
		Roles:    []string{"Overseer"},
	})
	resp, err := handler.RespondToApprovalRequest(ctx, connect.NewRequest(&plansv1.RespondToApprovalRequestRequest{
		TenantId:          tenantID.String(),
		ApprovalRequestId: approvalID,
		Approved:          true,
	}))
	if err != nil {
		t.Fatalf("RespondToApprovalRequest: %v", err)
	}
	if !signaler.called {
		t.Fatal("expected workflow signal")
	}
	if received.ApprovalRequestID != approvalID {
		t.Fatalf("signal approval id = %q, want %q", received.ApprovalRequestID, approvalID)
	}
	if received.StepExecutionID != stepExecutionID.String() {
		t.Fatalf("signal step id = %q, want %q", received.StepExecutionID, stepExecutionID)
	}
	if !received.Approved {
		t.Fatal("expected approved signal")
	}
	if resp.Msg.GetApprovalRequest().GetStatus() != plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_APPROVED {
		t.Fatalf("status = %v", resp.Msg.GetApprovalRequest().GetStatus())
	}
	if resp.Msg.GetApprovalRequest().GetPlanConfigurationId() != planConfigurationID.String() {
		t.Fatalf("plan configuration id = %q, want %q", resp.Msg.GetApprovalRequest().GetPlanConfigurationId(), planConfigurationID)
	}
	if resp.Msg.GetApprovalRequest().GetThreadId() != threadID.String() {
		t.Fatalf("thread id = %q, want %q", resp.Msg.GetApprovalRequest().GetThreadId(), threadID)
	}
}
