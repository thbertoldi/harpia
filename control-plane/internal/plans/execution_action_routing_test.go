package plans

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

type routingReviewStore struct {
	requests map[uuid.UUID]*PlanReviewRequest
}

func (s *routingReviewStore) GetPlanReviewRequest(_ context.Context, tenantID, reviewID uuid.UUID) (*PlanReviewRequest, error) {
	request := s.requests[reviewID]
	if request == nil || request.TenantID != tenantID {
		return nil, errors.New("review request not found")
	}
	copy := *request
	return &copy, nil
}

func (s *routingReviewStore) ListPlanReviewRequests(context.Context, uuid.UUID, ReviewFilters) ([]*PlanReviewRequest, error) {
	return nil, nil
}

type capturedReviewSignal struct {
	workflowID string
	signal     workflow.ReviewDecisionSignal
}

type routingReviewSignaler struct{ signals []capturedReviewSignal }

func (s *routingReviewSignaler) SignalPlanReviewDecision(_ context.Context, workflowID, _ string, signal workflow.ReviewDecisionSignal) error {
	s.signals = append(s.signals, capturedReviewSignal{workflowID: workflowID, signal: signal})
	return nil
}

func TestRespondToReviewRequestRoutesConcurrentSameStepToExactWorkflow(t *testing.T) {
	tenantID := uuid.New()
	stepA, stepB := uuid.New(), uuid.New()
	executionA, executionB := uuid.New(), uuid.New()
	reviewA, reviewB := uuid.New(), uuid.New()
	store := &routingReviewStore{requests: map[uuid.UUID]*PlanReviewRequest{
		reviewA: {ID: reviewA, TenantID: tenantID, PlanExecutionID: executionA, StepExecutionID: stepA, PlanStepKey: "publish", Status: ReviewRequestStatusPending},
		reviewB: {ID: reviewB, TenantID: tenantID, PlanExecutionID: executionB, StepExecutionID: stepB, PlanStepKey: "publish", Status: ReviewRequestStatusPending},
	}}
	signaler := &routingReviewSignaler{}
	handler := &PlanHandler{reviews: store, reviewSignaler: signaler}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{TenantID: tenantID, UserID: uuid.NewString(), Roles: []string{"leader"}})

	response, err := handler.RespondToReviewRequest(ctx, connect.NewRequest(&plansv1.RespondToReviewRequestRequest{
		TenantId: tenantID.String(), ReviewRequestId: reviewA.String(), Decision: &plansv1.ReviewDecision{Kind: plansv1.ReviewDecisionKind_REVIEW_DECISION_KIND_ACCEPT},
	}))
	if err != nil {
		t.Fatalf("RespondToReviewRequest: %v", err)
	}
	if response.Msg.GetReviewRequest().GetStatus() != plansv1.ReviewRequestStatus_REVIEW_REQUEST_STATUS_PENDING {
		t.Fatal("handler must not persist the terminal review decision")
	}
	if got := len(signaler.signals); got != 1 {
		t.Fatalf("signals = %d, want 1", got)
	}
	got := signaler.signals[0]
	if got.workflowID != workflow.PlanWorkflowID(executionA.String()) || got.signal.StepExecutionID != stepA.String() || got.signal.ReviewRequestID != reviewA.String() {
		t.Fatalf("signal routed to %+v, want execution A / step A / review A", got)
	}
	if got.workflowID == workflow.PlanWorkflowID(executionB.String()) || got.signal.StepExecutionID == stepB.String() || got.signal.ReviewRequestID == reviewB.String() {
		t.Fatal("request A must not target concurrent execution B with the same step key")
	}
}

func TestRespondToReviewRequestCrossTenantLookupIsNotFound(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	reviewID := uuid.New()
	handler := &PlanHandler{reviews: &routingReviewStore{requests: map[uuid.UUID]*PlanReviewRequest{
		reviewID: {ID: reviewID, TenantID: tenantA, Status: ReviewRequestStatusPending},
	}}, reviewSignaler: &routingReviewSignaler{}}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{TenantID: tenantB, UserID: uuid.NewString(), Roles: []string{"leader"}})
	_, err := handler.RespondToReviewRequest(ctx, connect.NewRequest(&plansv1.RespondToReviewRequestRequest{
		TenantId: tenantB.String(), ReviewRequestId: reviewID.String(), Decision: &plansv1.ReviewDecision{Kind: plansv1.ReviewDecisionKind_REVIEW_DECISION_KIND_ACCEPT},
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("cross-tenant lookup code = %s, want not found (err=%v)", connect.CodeOf(err), err)
	}
}
