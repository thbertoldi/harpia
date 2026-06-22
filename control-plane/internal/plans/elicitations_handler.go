package plans

import (
	"context"
	"encoding/json"
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

// watchElicitationsPollInterval controls how frequently WatchElicitations
// re-polls for new/updated pending elicitations. Connect server-streaming keeps
// the frontend aligned with the execution stream (NFR-4) without a pub/sub bus.
const watchElicitationsPollInterval = 2 * time.Second

// ElicitationStore is the persistence surface the handler depends on. *Repository
// satisfies it; tests can substitute a fake.
type ElicitationStore interface {
	GetElicitation(ctx context.Context, tenantID, elicitationID uuid.UUID) (*Elicitation, error)
	ListElicitations(ctx context.Context, filter ElicitationFilter) ([]Elicitation, error)
	MarkElicitationAnswered(ctx context.Context, tenantID, elicitationID uuid.UUID, payloadJSON json.RawMessage, responseText string, respondedBy uuid.NullUUID) (*Elicitation, error)
}

// PlanElicitationSignaler resumes a paused PlanWorkflow by delivering the
// overseer response signal. *workflow.TemporalClient satisfies it.
type PlanElicitationSignaler interface {
	SignalPlanElicitationResponse(ctx context.Context, workflowID, runID string, signal workflow.ElicitationResponseSignal) error
}

func (h *PlanHandler) ListElicitations(ctx context.Context, req *connect.Request[plansv1.ListElicitationsRequest]) (*connect.Response[plansv1.ListElicitationsResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if h.elicitations == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("elicitation store is unavailable"))
	}

	filter, limit, offset, err := h.buildElicitationFilter(ctx, tenantID, req.Msg.StepExecutionId, req.Msg.PlanExecutionId, req.Msg.Status, req.Msg.AddressedToMe, req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return nil, err
	}

	elicitations, err := h.elicitations.ListElicitations(ctx, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(elicitations) > limit
	if hasNext {
		elicitations = elicitations[:limit]
	}
	response := &plansv1.ListElicitationsResponse{
		Elicitations: make([]*plansv1.ElicitationRequest, 0, len(elicitations)),
	}
	for i := range elicitations {
		response.Elicitations = append(response.Elicitations, elicitationToProto(&elicitations[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return connect.NewResponse(response), nil
}

func (h *PlanHandler) GetElicitation(ctx context.Context, req *connect.Request[plansv1.GetElicitationRequest]) (*connect.Response[plansv1.GetElicitationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if h.elicitations == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("elicitation store is unavailable"))
	}
	elicitationID, err := uuid.Parse(req.Msg.ElicitationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	elicitation, err := h.elicitations.GetElicitation(ctx, tenantID, elicitationID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&plansv1.GetElicitationResponse{
		Elicitation: elicitationToProto(elicitation),
	}), nil
}

func (h *PlanHandler) RespondToElicitation(ctx context.Context, req *connect.Request[plansv1.RespondToElicitationRequest]) (*connect.Response[plansv1.RespondToElicitationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	if h.elicitations == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("elicitation store is unavailable"))
	}
	if h.signaler == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workflow engine is unavailable"))
	}
	elicitationID, err := uuid.Parse(req.Msg.ElicitationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	elicitation, err := h.elicitations.GetElicitation(ctx, tenantID, elicitationID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if elicitation.Status != ElicitationStatusPending {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("elicitation is not pending"))
	}
	if err := authorizeElicitationResponse(rc, elicitation); err != nil {
		return nil, err
	}

	payloadJSON, responseText, err := normalizeElicitationResponse(req.Msg.PayloadJson, req.Msg.ResponseText)
	if err != nil {
		return nil, err
	}

	respondedBy := nullableUserID(rc.UserID)
	updated, err := h.elicitations.MarkElicitationAnswered(ctx, tenantID, elicitationID, payloadJSON, responseText, respondedBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	signal := workflow.ElicitationResponseSignal{
		StepExecutionID:     updated.StepExecutionID.String(),
		ElicitationThreadID: updated.ElicitationThreadID,
		ResponseText:        signalResponseText(responseText, payloadJSON),
	}
	workflowID := workflow.PlanWorkflowID(updated.PlanExecutionID.String())
	if err := h.signaler.SignalPlanElicitationResponse(ctx, workflowID, "", signal); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	if h.chat != nil {
		execID := updated.PlanExecutionID
		configID, lookupErr := h.repo.GetPlanConfigurationIDForExecution(ctx, tenantID, updated.PlanExecutionID)
		if lookupErr == nil {
			_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ELICITATION_ANSWERED,
				Text:        "Question on step " + updated.PlanStepKey + " was answered.",
				PayloadJSON: chat.BuildElicitationAnsweredPayload(updated.ID, "answered"),
			})
		}
	}

	return connect.NewResponse(&plansv1.RespondToElicitationResponse{
		Elicitation: elicitationToProto(updated),
	}), nil
}

func (h *PlanHandler) WatchElicitations(ctx context.Context, req *connect.Request[plansv1.WatchElicitationsRequest], stream *connect.ServerStream[plansv1.WatchElicitationsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}
	if h.elicitations == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("elicitation store is unavailable"))
	}

	filter, _, _, err := h.buildElicitationFilter(ctx, tenantID, req.Msg.StepExecutionId, nil, nil, req.Msg.AddressedToMe, 0, "")
	if err != nil {
		return err
	}
	filter.Limit = 100

	send := func() error {
		elicitations, listErr := h.elicitations.ListElicitations(ctx, filter)
		if listErr != nil {
			return connect.NewError(connect.CodeInternal, listErr)
		}
		event := &plansv1.WatchElicitationsResponse{
			Elicitations: make([]*plansv1.ElicitationRequest, 0, len(elicitations)),
		}
		for i := range elicitations {
			event.Elicitations = append(event.Elicitations, elicitationToProto(&elicitations[i]))
		}
		return stream.Send(event)
	}

	if err := send(); err != nil {
		return err
	}

	ticker := time.NewTicker(watchElicitationsPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
}

func (h *PlanHandler) buildElicitationFilter(
	ctx context.Context,
	tenantID uuid.UUID,
	stepExecutionID *string,
	planExecutionID *string,
	status *plansv1.ElicitationStatus,
	addressedToMe bool,
	pageSize int32,
	pageToken string,
) (ElicitationFilter, int, int, error) {
	filter := ElicitationFilter{TenantID: tenantID}
	var err error

	if filter.StepExecutionID, err = optionalUUIDArg(stepExecutionID); err != nil {
		return filter, 0, 0, err
	}
	if filter.PlanExecutionID, err = optionalUUIDArg(planExecutionID); err != nil {
		return filter, 0, 0, err
	}
	if status != nil && *status != plansv1.ElicitationStatus_ELICITATION_STATUS_UNSPECIFIED {
		filter.Status = elicitationStatusToString(*status)
	}
	if addressedToMe {
		if filter.OverseerUserID, err = callerUserID(ctx); err != nil {
			return filter, 0, 0, err
		}
	}

	limit, offset, err := parsePagination(pageSize, pageToken)
	if err != nil {
		return filter, 0, 0, err
	}
	filter.Limit = limit + 1
	filter.Offset = offset
	return filter, limit, offset, nil
}

func optionalUUIDArg(raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(*raw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return &parsed, nil
}

func callerUserID(ctx context.Context) (*uuid.UUID, error) {
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	userID, parseErr := uuid.Parse(strings.TrimSpace(rc.UserID))
	if parseErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("caller has no resolvable user id"))
	}
	return &userID, nil
}

func parsePagination(pageSize int32, pageToken string) (int, int, error) {
	limit := int(pageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if pageToken != "" {
		parsed, parseErr := strconv.Atoi(pageToken)
		if parseErr != nil || parsed < 0 {
			return 0, 0, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid page token"))
		}
		offset = parsed
	}
	return limit, offset, nil
}

// authorizeElicitationResponse enforces that only the assigned overseer or a
// Leader (PR #177) may answer an elicitation.
func authorizeElicitationResponse(rc identity.RequestContext, elicitation *Elicitation) error {
	if isLeaderRole(rc.Roles) {
		return nil
	}
	if elicitation.OverseerUserID.Valid {
		if strings.EqualFold(strings.TrimSpace(rc.UserID), elicitation.OverseerUserID.UUID.String()) {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New("only the assigned overseer or a leader may respond to this elicitation"))
}

func isLeaderRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "leader", "admin", "owner":
			return true
		}
	}
	return false
}

func normalizeElicitationResponse(payloadJSON, responseText string) (json.RawMessage, string, error) {
	text := strings.TrimSpace(responseText)
	rawPayload := strings.TrimSpace(payloadJSON)
	if rawPayload == "" && text == "" {
		return nil, "", connect.NewError(connect.CodeInvalidArgument, errors.New("payload_json or response_text is required"))
	}
	var payload json.RawMessage
	if rawPayload != "" {
		if !json.Valid([]byte(rawPayload)) {
			return nil, "", connect.NewError(connect.CodeInvalidArgument, errors.New("payload_json is not valid JSON"))
		}
		payload = json.RawMessage(rawPayload)
	}
	return payload, text, nil
}

// signalResponseText guarantees the workflow signal carries a non-empty response
// (its validator requires response_text or response_artifact_id).
func signalResponseText(responseText string, payloadJSON json.RawMessage) string {
	if strings.TrimSpace(responseText) != "" {
		return responseText
	}
	return string(payloadJSON)
}

func nullableUserID(raw string) uuid.NullUUID {
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}
}
