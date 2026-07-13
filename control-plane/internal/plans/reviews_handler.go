package plans

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

type ReviewStore interface {
	GetPlanReviewRequest(context.Context, uuid.UUID, uuid.UUID) (*PlanReviewRequest, error)
	ListPlanReviewRequests(context.Context, uuid.UUID, ReviewFilters) ([]*PlanReviewRequest, error)
}

type PlanReviewSignaler interface {
	SignalPlanReviewDecision(context.Context, string, string, workflow.ReviewDecisionSignal) error
}

func (h *PlanHandler) ListReviewRequests(ctx context.Context, req *connect.Request[plansv1.ListReviewRequestsRequest]) (*connect.Response[plansv1.ListReviewRequestsResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := h.reviews.ListPlanReviewRequests(ctx, tenantID, ReviewFilters{Limit: int(req.Msg.GetPageSize())})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*plansv1.ReviewRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewToProto(row))
	}
	return connect.NewResponse(&plansv1.ListReviewRequestsResponse{ReviewRequests: out}), nil
}

func (h *PlanHandler) GetReviewRequest(ctx context.Context, req *connect.Request[plansv1.GetReviewRequestRequest]) (*connect.Response[plansv1.GetReviewRequestResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GetReviewRequestId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	row, err := h.reviews.GetPlanReviewRequest(ctx, tenantID, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&plansv1.GetReviewRequestResponse{ReviewRequest: reviewToProto(row)}), nil
}

func (h *PlanHandler) RespondToReviewRequest(ctx context.Context, req *connect.Request[plansv1.RespondToReviewRequestRequest]) (*connect.Response[plansv1.RespondToReviewRequestResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GetReviewRequestId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	row, err := h.reviews.GetPlanReviewRequest(ctx, tenantID, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if row.Status != ReviewRequestStatusPending {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("review request is not pending"))
	}
	if !isLeaderRole(rc.Roles) && (!row.OverseerUserID.Valid || row.OverseerUserID.UUID.String() != strings.TrimSpace(rc.UserID)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only the assigned overseer or a leader may decide this review"))
	}
	decision := req.Msg.GetDecision()
	if decision == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("review decision is required"))
	}
	kind := "accept"
	if decision.GetKind() == plansv1.ReviewDecisionKind_REVIEW_DECISION_KIND_REVISE {
		kind = "revise"
		if strings.TrimSpace(decision.GetFeedback()) == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("feedback is required when requesting revision"))
		}
	} else if decision.GetKind() != plansv1.ReviewDecisionKind_REVIEW_DECISION_KIND_ACCEPT {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported review decision"))
	}
	if h.reviewSignaler == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workflow engine is unavailable"))
	}
	err = h.reviewSignaler.SignalPlanReviewDecision(ctx, workflow.PlanWorkflowID(row.PlanExecutionID.String()), "", workflow.ReviewDecisionSignal{StepExecutionID: row.StepExecutionID.String(), ReviewRequestID: row.ID.String(), Decision: kind, Feedback: strings.TrimSpace(decision.GetFeedback())})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	// Temporal's ResolveReviewRequestActivity is the sole terminal persistence
	// owner. The command response intentionally remains the still-pending row
	// until that exact workflow has accepted and resolved the signal.
	return connect.NewResponse(&plansv1.RespondToReviewRequestResponse{ReviewRequest: reviewToProto(row)}), nil
}

func (h *PlanHandler) WatchReviewRequests(ctx context.Context, req *connect.Request[plansv1.WatchReviewRequestsRequest], stream *connect.ServerStream[plansv1.WatchReviewRequestsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return err
	}
	for {
		rows, listErr := h.reviews.ListPlanReviewRequests(ctx, tenantID, ReviewFilters{Limit: 100})
		if listErr != nil {
			return connect.NewError(connect.CodeInternal, listErr)
		}
		out := make([]*plansv1.ReviewRequest, 0, len(rows))
		for _, row := range rows {
			out = append(out, reviewToProto(row))
		}
		if err := stream.Send(&plansv1.WatchReviewRequestsResponse{ReviewRequests: out}); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
}

func reviewToProto(row *PlanReviewRequest) *plansv1.ReviewRequest {
	if row == nil {
		return nil
	}
	out := &plansv1.ReviewRequest{Id: row.ID.String(), TenantId: row.TenantID.String(), StepExecutionId: row.StepExecutionID.String(), PlanExecutionId: row.PlanExecutionID.String(), PlanStepKey: row.PlanStepKey, SubjectArtifactRef: &artifactsv1.ArtifactRef{ArtifactId: row.SubjectArtifactID.String(), ArtifactVersionId: row.SubjectArtifactVersionID.String(), ArtifactTypeKey: row.SubjectArtifactTypeKey, ContentHash: row.SubjectContentHash}, RevisionFeedback: row.RevisionFeedback, RequestedAt: row.RequestedAt.UTC().Format(time.RFC3339)}
	if row.Status == ReviewRequestStatusAccepted {
		out.Status = plansv1.ReviewRequestStatus_REVIEW_REQUEST_STATUS_ACCEPTED
	} else if row.Status == ReviewRequestStatusRevisionRequested {
		out.Status = plansv1.ReviewRequestStatus_REVIEW_REQUEST_STATUS_REVISION_REQUESTED
	} else {
		out.Status = plansv1.ReviewRequestStatus_REVIEW_REQUEST_STATUS_PENDING
	}
	if row.OverseerUserID.Valid {
		out.OverseerUserId = row.OverseerUserID.UUID.String()
	}
	return out
}
