package threads

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/gen/harpia/chat/v1/chatv1connect"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/copilot"
	"github.com/harpia/control-plane/internal/identity"
)

const (
	defaultThreadMessagePageSize = 100
	maxThreadMessagePageSize     = 100
	watchThreadMessagesInterval  = 2 * time.Second
)

var _ chatv1connect.ThreadServiceHandler = (*Handler)(nil)

type RepositoryAPI interface {
	Create(ctx context.Context, input CreateInput) (*Thread, error)
	Get(ctx context.Context, tenantID, threadID uuid.UUID) (*Thread, error)
	List(ctx context.Context, input ListInput) ([]*Thread, string, error)
	Archive(ctx context.Context, tenantID, threadID uuid.UUID) (*Thread, error)
}

// TemplateCatalog lists plan templates as router catalog entries. Implemented
// by plans.CopilotCatalog.
type TemplateCatalog interface {
	ListTemplateSummaries(ctx context.Context) ([]copilot.TemplateSummary, error)
}

type Handler struct {
	repo       RepositoryAPI
	chat       chat.Store
	catalog    TemplateCatalog
	classifier copilot.PlanClassifier
}

func NewHandler(repo RepositoryAPI, chatStore chat.Store, catalog TemplateCatalog, classifier copilot.PlanClassifier) *Handler {
	return &Handler{
		repo:       repo,
		chat:       chatStore,
		catalog:    catalog,
		classifier: classifier,
	}
}

func (h *Handler) CreateThread(
	ctx context.Context,
	req *connect.Request[chatv1.CreateThreadRequest],
) (*connect.Response[chatv1.CreateThreadResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	if h.repo == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("thread repository is unavailable"))
	}
	initialText := strings.TrimSpace(req.Msg.GetInitialMessageText())
	if initialText != "" && h.chat == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}

	input := CreateInput{
		TenantID: tenantID,
		Title:    req.Msg.GetTitle(),
	}
	if parsed, err := uuid.Parse(strings.TrimSpace(rc.UserID)); err == nil {
		input.CreatedByUserID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	thread, err := h.repo.Create(ctx, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var initial *chatv1.ThreadMessage
	if initialText != "" {
		appendInput := chat.AppendInput{
			ThreadID: thread.ID.String(),
			Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
			Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
			Text:     initialText,
		}
		if input.CreatedByUserID.Valid {
			appendInput.AuthorUserID = &input.CreatedByUserID.UUID
		}
		initial, err = h.chat.AppendMessage(ctx, tenantID, appendInput)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&chatv1.CreateThreadResponse{
		Thread:         thread.Proto(),
		InitialMessage: initial,
	}), nil
}

func (h *Handler) GetThread(
	ctx context.Context,
	req *connect.Request[chatv1.GetThreadRequest],
) (*connect.Response[chatv1.GetThreadResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	threadID, err := parseRequiredUUID(req.Msg.GetThreadId(), "thread_id")
	if err != nil {
		return nil, err
	}
	if h.repo == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("thread repository is unavailable"))
	}
	thread, err := h.repo.Get(ctx, tenantID, threadID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return connect.NewResponse(&chatv1.GetThreadResponse{Thread: thread.Proto()}), nil
}

func (h *Handler) ListThreads(
	ctx context.Context,
	req *connect.Request[chatv1.ListThreadsRequest],
	stream *connect.ServerStream[chatv1.ListThreadsResponse],
) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return err
	}
	if h.repo == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("thread repository is unavailable"))
	}
	input := ListInput{
		TenantID:  tenantID,
		PageSize:  req.Msg.GetPageSize(),
		PageToken: req.Msg.GetPageToken(),
	}
	if req.Msg.Status != nil && req.Msg.GetStatus() != chatv1.ThreadStatus_THREAD_STATUS_UNSPECIFIED {
		status := req.Msg.GetStatus()
		input.Status = &status
	}
	threads, nextPageToken, err := h.repo.List(ctx, input)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*chatv1.Thread, 0, len(threads))
	for _, thread := range threads {
		out = append(out, thread.Proto())
	}
	return stream.Send(&chatv1.ListThreadsResponse{
		Threads:       out,
		NextPageToken: nextPageToken,
	})
}

func (h *Handler) ArchiveThread(
	ctx context.Context,
	req *connect.Request[chatv1.ArchiveThreadRequest],
) (*connect.Response[chatv1.ArchiveThreadResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	threadID, err := parseRequiredUUID(req.Msg.GetThreadId(), "thread_id")
	if err != nil {
		return nil, err
	}
	if h.repo == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("thread repository is unavailable"))
	}
	thread, err := h.repo.Archive(ctx, tenantID, threadID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return connect.NewResponse(&chatv1.ArchiveThreadResponse{Thread: thread.Proto()}), nil
}

func (h *Handler) ListThreadMessages(
	ctx context.Context,
	req *connect.Request[chatv1.ListThreadMessagesRequest],
) (*connect.Response[chatv1.ListThreadMessagesResponse], error) {
	tenantID, threadID, err := h.validateMessageRequest(ctx, req.Msg.GetTenantId(), req.Msg.GetThreadId())
	if err != nil {
		return nil, err
	}
	sinceSeq, err := parseSequenceToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	limit := messagePageSize(req.Msg.GetPageSize())
	msgs, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), sinceSeq, limit+1)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var nextPageToken string
	if len(msgs) > limit {
		msgs = msgs[:limit]
		nextPageToken = strconv.FormatInt(msgs[len(msgs)-1].GetSequenceNumber(), 10)
	}
	return connect.NewResponse(&chatv1.ListThreadMessagesResponse{
		Messages:      msgs,
		NextPageToken: nextPageToken,
	}), nil
}

func (h *Handler) WatchThreadMessages(
	ctx context.Context,
	req *connect.Request[chatv1.WatchThreadMessagesRequest],
	stream *connect.ServerStream[chatv1.WatchThreadMessagesResponse],
) error {
	tenantID, threadID, err := h.validateMessageRequest(ctx, req.Msg.GetTenantId(), req.Msg.GetThreadId())
	if err != nil {
		return err
	}
	sinceSeq := req.Msg.GetSinceSequenceNumber()
	initial, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), sinceSeq, 0)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if len(initial) > 0 {
		if err := stream.Send(&chatv1.WatchThreadMessagesResponse{Messages: initial}); err != nil {
			return err
		}
		sinceSeq = initial[len(initial)-1].GetSequenceNumber()
	}

	ticker := time.NewTicker(watchThreadMessagesInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			next, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), sinceSeq, 0)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if len(next) == 0 {
				continue
			}
			if err := stream.Send(&chatv1.WatchThreadMessagesResponse{Messages: next}); err != nil {
				return err
			}
			sinceSeq = next[len(next)-1].GetSequenceNumber()
		}
	}
}

func (h *Handler) AppendThreadMessage(
	ctx context.Context,
	req *connect.Request[chatv1.AppendThreadMessageRequest],
) (*connect.Response[chatv1.AppendThreadMessageResponse], error) {
	tenantID, threadID, err := h.validateMessageRequest(ctx, req.Msg.GetTenantId(), req.Msg.GetThreadId())
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetRole() == chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	if req.Msg.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind is required"))
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
	if parsed, err := uuid.Parse(strings.TrimSpace(rc.UserID)); err == nil {
		input.AuthorUserID = &parsed
	}
	msg, err := h.chat.AppendMessage(ctx, tenantID, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&chatv1.AppendThreadMessageResponse{Message: msg}), nil
}

func (h *Handler) validateMessageRequest(ctx context.Context, tenant string, thread string) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := identity.RequireTenant(ctx, tenant)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	threadID, err := parseRequiredUUID(thread, "thread_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if h.chat == nil {
		return uuid.Nil, uuid.Nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}
	return tenantID, threadID, nil
}

func parseRequiredUUID(value string, field string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New(field+" is required"))
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return parsed, nil
}

func parseSequenceToken(token string) (int64, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return 0, nil
	}
	seq, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid page_token"))
	}
	if seq < 0 {
		return 0, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid page_token"))
	}
	return seq, nil
}

func messagePageSize(pageSize int32) int {
	limit := int(pageSize)
	if limit <= 0 {
		return defaultThreadMessagePageSize
	}
	if limit > maxThreadMessagePageSize {
		return maxThreadMessagePageSize
	}
	return limit
}

func mapRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

const planProposalConfidenceThreshold = 0.6

// ProposePlan classifies the thread's latest user message against the template
// catalog and appends a PLAN_PROPOSED message. It always succeeds when a user
// message exists, even if the router returns no candidates (empty proposal).
func (h *Handler) ProposePlan(ctx context.Context, req *connect.Request[chatv1.ProposePlanRequest]) (*connect.Response[chatv1.ProposePlanResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	threadID, err := parseRequiredUUID(req.Msg.GetThreadId(), "thread_id")
	if err != nil {
		return nil, err
	}

	// Find the latest USER_TEXT message in the thread.
	msgs, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), 0, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var latestUser *chatv1.ThreadMessage
	for _, m := range msgs {
		if m.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT {
			latestUser = m
		}
	}
	if latestUser == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no user message to propose from"))
	}

	// Load the catalog and classify. Both degrade to an empty proposal.
	var summaries []copilot.TemplateSummary
	if h.catalog != nil {
		if s, catErr := h.catalog.ListTemplateSummaries(ctx); catErr == nil {
			summaries = s
		}
	}
	var candidates []copilot.Candidate
	var summary string
	if h.classifier != nil && len(summaries) > 0 {
		if result, clsErr := h.classifier.Classify(ctx, copilot.ClassifyInput{
			TenantID:  tenantID,
			Text:      latestUser.GetText(),
			Templates: summaries,
		}); clsErr == nil {
			candidates = result.Candidates
			summary = result.Summary
		}
	}

	nameByID := map[string]string{}
	keyByID := map[string]string{}
	for _, s := range summaries {
		nameByID[s.ID.String()] = s.Name
		keyByID[s.ID.String()] = s.Key
	}

	payloadCandidates := make([]chat.PlanProposalCandidate, 0, len(candidates))
	for _, c := range candidates {
		payloadCandidates = append(payloadCandidates, chat.PlanProposalCandidate{
			TemplateID:      c.TemplateID.String(),
			TemplateKey:     keyByID[c.TemplateID.String()],
			TemplateName:    nameByID[c.TemplateID.String()],
			Confidence:      c.Confidence,
			InputValuesJSON: c.InputValuesJSON,
		})
	}

	msg, err := h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    threadID.String(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_PROPOSED,
		Text:        "Proposed a plan.",
		PayloadJSON: chat.BuildPlanProposedPayload(latestUser.GetId(), summary, payloadCandidates),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&chatv1.ProposePlanResponse{Message: msg}), nil
}
