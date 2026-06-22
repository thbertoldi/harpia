package plans

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

const watchApprovalRequestsPollInterval = 2 * time.Second

// ApprovalStore is the persistence surface the approval handler depends on.
type ApprovalStore interface {
	GetPlanApprovalRequest(ctx context.Context, tenantID uuid.UUID, approvalID string) (*PlanApprovalRequest, error)
	ListPlanApprovalRequests(ctx context.Context, tenantID uuid.UUID, filters ApprovalFilters) ([]*PlanApprovalRequest, error)
	MarkApprovalDecided(ctx context.Context, tenantID uuid.UUID, approvalID string, approved bool, reason string) (*PlanApprovalRequest, error)
}

// PlanApprovalSignaler resumes a paused PlanWorkflow after an approval decision.
type PlanApprovalSignaler interface {
	SignalPlanApprovalDecision(ctx context.Context, workflowID, runID string, signal workflow.ApprovalDecisionSignal) error
}

func (h *PlanHandler) ListApprovalRequests(
	ctx context.Context,
	req *connect.Request[plansv1.ListApprovalRequestsRequest],
) (*connect.Response[plansv1.ListApprovalRequestsResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	if h.approvals == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("approval store is unavailable"))
	}

	filter, limit, offset, err := buildApprovalFilter(req.Msg.StepExecutionId, req.Msg.PlanExecutionId, req.Msg.Status, req.Msg.GetPageSize(), req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	filter.Limit = limit + 1
	filter.Offset = offset

	rows, err := h.approvals.ListPlanApprovalRequests(ctx, tenantID, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	out := make([]*plansv1.ApprovalRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, approvalRequestToProto(planApprovalRequestToDomain(row)))
	}
	response := &plansv1.ListApprovalRequestsResponse{ApprovalRequests: out}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return connect.NewResponse(response), nil
}

func (h *PlanHandler) GetApprovalRequest(
	ctx context.Context,
	req *connect.Request[plansv1.GetApprovalRequestRequest],
) (*connect.Response[plansv1.GetApprovalRequestResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	if h.approvals == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("approval store is unavailable"))
	}
	approvalID := strings.TrimSpace(req.Msg.GetApprovalRequestId())
	if approvalID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("approval_request_id is required"))
	}
	row, err := h.approvals.GetPlanApprovalRequest(ctx, tenantID, approvalID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&plansv1.GetApprovalRequestResponse{
		ApprovalRequest: approvalRequestToProto(planApprovalRequestToDomain(row)),
	}), nil
}

func (h *PlanHandler) RespondToApprovalRequest(
	ctx context.Context,
	req *connect.Request[plansv1.RespondToApprovalRequestRequest],
) (*connect.Response[plansv1.RespondToApprovalRequestResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	if h.approvals == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("approval store is unavailable"))
	}
	if h.approvalSignaler == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workflow engine is unavailable"))
	}
	if err := validateApprovalDecision(req.Msg.GetApproved(), req.Msg.GetReason()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := authorizeApprovalResponse(rc); err != nil {
		return nil, err
	}

	approvalID := strings.TrimSpace(req.Msg.GetApprovalRequestId())
	if approvalID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("approval_request_id is required"))
	}
	row, err := h.approvals.GetPlanApprovalRequest(ctx, tenantID, approvalID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if row.Status != ApprovalRequestStatusPending {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("approval request is not pending"))
	}

	updated, err := h.approvals.MarkApprovalDecided(ctx, tenantID, approvalID, req.Msg.GetApproved(), strings.TrimSpace(req.Msg.GetReason()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	signal := workflow.ApprovalDecisionSignal{
		StepExecutionID:   updated.StepExecutionID.String(),
		ApprovalRequestID: updated.ID,
		Approved:          req.Msg.GetApproved(),
		Reason:            strings.TrimSpace(req.Msg.GetReason()),
	}
	workflowID := workflow.PlanWorkflowID(updated.PlanExecutionID.String())
	if err := h.approvalSignaler.SignalPlanApprovalDecision(ctx, workflowID, "", signal); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	if h.chat != nil {
		execID := updated.PlanExecutionID
		configID, lookupErr := h.repo.GetPlanConfigurationIDForExecution(ctx, tenantID, updated.PlanExecutionID)
		if lookupErr == nil {
			text := "Approval rejected."
			if req.Msg.GetApproved() {
				text = "Approval granted."
			}
			_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_APPROVAL_DECIDED,
				Text:        text,
				PayloadJSON: chat.BuildApprovalDecidedPayload(updated.ID, req.Msg.GetApproved()),
			})
		}
	}

	return connect.NewResponse(&plansv1.RespondToApprovalRequestResponse{
		ApprovalRequest: approvalRequestToProto(planApprovalRequestToDomain(updated)),
	}), nil
}

func (h *PlanHandler) WatchApprovalRequests(
	ctx context.Context,
	req *connect.Request[plansv1.WatchApprovalRequestsRequest],
	stream *connect.ServerStream[plansv1.WatchApprovalRequestsResponse],
) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return err
	}
	if h.approvals == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("approval store is unavailable"))
	}

	filter, _, _, err := buildApprovalFilter(req.Msg.StepExecutionId, req.Msg.PlanExecutionId, nil, 0, "")
	if err != nil {
		return err
	}
	filter.Limit = 100

	send := func() error {
		rows, listErr := h.approvals.ListPlanApprovalRequests(ctx, tenantID, filter)
		if listErr != nil {
			return connect.NewError(connect.CodeInternal, listErr)
		}
		out := make([]*plansv1.ApprovalRequest, 0, len(rows))
		for _, row := range rows {
			out = append(out, approvalRequestToProto(planApprovalRequestToDomain(row)))
		}
		return stream.Send(&plansv1.WatchApprovalRequestsResponse{ApprovalRequests: out})
	}
	if err := send(); err != nil {
		return err
	}

	ticker := time.NewTicker(watchApprovalRequestsPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
}

func buildApprovalFilter(
	stepExecutionID *string,
	planExecutionID *string,
	status *plansv1.ApprovalRequestStatus,
	pageSize int32,
	pageToken string,
) (ApprovalFilters, int, int, error) {
	filter := ApprovalFilters{}
	var err error

	if filter.StepExecutionID, err = optionalUUIDStringArg(stepExecutionID); err != nil {
		return filter, 0, 0, err
	}
	if filter.PlanExecutionID, err = optionalUUIDStringArg(planExecutionID); err != nil {
		return filter, 0, 0, err
	}
	if status != nil && *status != plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_UNSPECIFIED {
		filter.Status, err = approvalRequestStatusFromProto(*status)
		if err != nil {
			return filter, 0, 0, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	limit, offset, err := parsePagination(pageSize, pageToken)
	if err != nil {
		return filter, 0, 0, err
	}
	return filter, limit, offset, nil
}

func optionalUUIDStringArg(raw *string) (*string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	value := parsed.String()
	return &value, nil
}

func authorizeApprovalResponse(rc identity.RequestContext) error {
	if isLeaderRole(rc.Roles) {
		return nil
	}
	for _, role := range rc.Roles {
		if strings.EqualFold(strings.TrimSpace(role), "overseer") {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New("only a leader or overseer may respond to approval requests"))
}
