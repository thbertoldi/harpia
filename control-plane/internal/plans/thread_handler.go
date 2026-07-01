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
)

const watchPlanThreadMessagesPollInterval = 2 * time.Second

// newWatchTicker returns a small wrapper over time.Ticker so handlers can be
// substituted in tests. Production uses the real ticker.
type watchTicker struct{ t *time.Ticker }

func (w *watchTicker) C() <-chan time.Time { return w.t.C }
func (w *watchTicker) Stop()               { w.t.Stop() }

func newWatchTicker(d time.Duration) *watchTicker {
	return &watchTicker{t: time.NewTicker(d)}
}

// resolvePlanThreadID maps a legacy plan_configuration_id to its owning
// threads.id. It returns connect-coded errors so callers can return the result
// directly: InvalidArgument for a malformed id, NotFound when the configuration
// has no resolvable thread. When no resolver is configured (in-memory tests) it
// falls back to the parsed configuration id.
func (h *PlanHandler) resolvePlanThreadID(ctx context.Context, tenantID uuid.UUID, configID string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(configID))
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if h.resolveThreadIDForPlanConfiguration == nil {
		return parsed, nil
	}
	threadID, err := h.resolveThreadIDForPlanConfiguration(ctx, tenantID, parsed)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeNotFound, err)
	}
	return threadID, nil
}

func (h *PlanHandler) ListPlanThreadMessages(
	ctx context.Context,
	req *connect.Request[plansv1.ListPlanThreadMessagesRequest],
) (*connect.Response[plansv1.ListPlanThreadMessagesResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	configID := strings.TrimSpace(req.Msg.GetPlanConfigurationId())
	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("plan_configuration_id is required"))
	}
	if h.chat == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 {
		limit = 100
	}
	sinceSeq := int64(0)
	if token := strings.TrimSpace(req.Msg.GetPageToken()); token != "" {
		parsed, parseErr := strconv.ParseInt(token, 10, 64)
		if parseErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid page_token"))
		}
		sinceSeq = parsed
	}
	threadID, err := h.resolvePlanThreadID(ctx, tenantID, configID)
	if err != nil {
		return nil, err
	}
	fetchLimit := limit + 1
	msgs, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), sinceSeq, fetchLimit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var nextPageToken string
	if len(msgs) > limit {
		msgs = msgs[:limit]
		if len(msgs) > 0 {
			nextPageToken = strconv.FormatInt(msgs[len(msgs)-1].SequenceNumber, 10)
		}
	}
	return connect.NewResponse(&plansv1.ListPlanThreadMessagesResponse{
		Messages:      msgs,
		NextPageToken: nextPageToken,
	}), nil
}

func (h *PlanHandler) AppendPlanThreadMessage(
	ctx context.Context,
	req *connect.Request[plansv1.AppendPlanThreadMessageRequest],
) (*connect.Response[plansv1.AppendPlanThreadMessageResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	configID := strings.TrimSpace(req.Msg.GetPlanConfigurationId())
	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("plan_configuration_id is required"))
	}
	if h.chat == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}
	threadID, err := h.resolvePlanThreadID(ctx, tenantID, configID)
	if err != nil {
		return nil, err
	}
	input := chat.AppendInput{
		ThreadID:    threadID.String(),
		Role:        req.Msg.GetRole(),
		Kind:        req.Msg.GetKind(),
		Text:        req.Msg.GetText(),
		PayloadJSON: req.Msg.GetPayloadJson(),
	}
	if execID := strings.TrimSpace(req.Msg.GetExecutionId()); execID != "" {
		parsed, err := uuid.Parse(execID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		input.ExecutionID = &parsed
	}
	if rc.UserID != "" {
		parsed, err := uuid.Parse(rc.UserID)
		if err == nil {
			input.AuthorUserID = &parsed
		}
	}
	msg, err := h.chat.AppendMessage(ctx, tenantID, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if req.Msg.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_REBOUND && h.assistant != nil {
		parsedConfigID, parseErr := uuid.Parse(configID)
		if parseErr == nil {
			_ = h.assistant.NextTurn(ctx, tenantID, parsedConfigID)
		}
	}
	return connect.NewResponse(&plansv1.AppendPlanThreadMessageResponse{
		Message: msg,
	}), nil
}

func (h *PlanHandler) WatchPlanThreadMessages(
	ctx context.Context,
	req *connect.Request[plansv1.WatchPlanThreadMessagesRequest],
	stream *connect.ServerStream[plansv1.WatchPlanThreadMessagesResponse],
) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return err
	}
	configID := strings.TrimSpace(req.Msg.GetPlanConfigurationId())
	if configID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("plan_configuration_id is required"))
	}
	if h.chat == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}
	threadID, err := h.resolvePlanThreadID(ctx, tenantID, configID)
	if err != nil {
		return err
	}

	sinceSeq := req.Msg.GetSinceSequenceNumber()
	// Initial flush: send everything since the resume point.
	initial, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), sinceSeq, 0)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if len(initial) > 0 {
		if err := stream.Send(&plansv1.WatchPlanThreadMessagesResponse{Messages: initial}); err != nil {
			return err
		}
		sinceSeq = initial[len(initial)-1].SequenceNumber
	}

	// Poll for new messages every 2 seconds (matches WatchElicitations pattern).
	ticker := newWatchTicker(watchPlanThreadMessagesPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C():
			next, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), sinceSeq, 0)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if len(next) > 0 {
				if err := stream.Send(&plansv1.WatchPlanThreadMessagesResponse{Messages: next}); err != nil {
					return err
				}
				sinceSeq = next[len(next)-1].SequenceNumber
			}
		}
	}
}
