package threads

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/copilot"
	"github.com/harpia/control-plane/internal/identity"
)

type fakeThreadRepo struct {
	created []*Thread
}

func (f *fakeThreadRepo) Create(_ context.Context, input CreateInput) (*Thread, error) {
	thread := &Thread{
		ID:              uuid.New(),
		TenantID:        input.TenantID,
		Title:           input.Title,
		Status:          chatv1.ThreadStatus_THREAD_STATUS_OPEN,
		CreatedByUserID: input.CreatedByUserID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	f.created = append(f.created, thread)
	return thread, nil
}

func (f *fakeThreadRepo) Get(_ context.Context, tenantID, threadID uuid.UUID) (*Thread, error) {
	for _, thread := range f.created {
		if thread.TenantID == tenantID && thread.ID == threadID {
			return thread, nil
		}
	}
	return nil, errFakeNotFound
}

func (f *fakeThreadRepo) List(_ context.Context, input ListInput) ([]*Thread, string, error) {
	out := make([]*Thread, 0, len(f.created))
	for _, thread := range f.created {
		if thread.TenantID == input.TenantID {
			out = append(out, thread)
		}
	}
	return out, "", nil
}

func (f *fakeThreadRepo) Archive(_ context.Context, tenantID, threadID uuid.UUID) (*Thread, error) {
	thread, err := f.Get(context.Background(), tenantID, threadID)
	if err != nil {
		return nil, err
	}
	thread.Status = chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED
	return thread, nil
}

type fakeMessageStore struct {
	appended []chat.AppendInput
	messages []*chatv1.ThreadMessage
}

func (f *fakeMessageStore) AppendMessage(_ context.Context, tenantID uuid.UUID, input chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, input)
	msg := &chatv1.ThreadMessage{
		Id:             uuid.NewString(),
		TenantId:       tenantID.String(),
		ThreadId:       input.ThreadID,
		Role:           input.Role,
		Kind:           input.Kind,
		Text:           input.Text,
		PayloadJson:    input.PayloadJSON,
		SequenceNumber: int64(len(f.appended)),
		CreatedAt:      timestamppb.Now(),
	}
	if input.AuthorUserID != nil {
		msg.AuthorUserId = input.AuthorUserID.String()
	}
	if input.ExecutionID != nil {
		msg.ExecutionId = input.ExecutionID.String()
	}
	f.messages = append(f.messages, msg)
	return msg, nil
}

func (f *fakeMessageStore) ListMessages(_ context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
	out := make([]*chatv1.ThreadMessage, 0)
	for _, msg := range f.messages {
		if msg.GetTenantId() != tenantID.String() || msg.GetThreadId() != threadID || msg.GetSequenceNumber() <= sinceSeq {
			continue
		}
		out = append(out, msg)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

var errFakeNotFound = connect.NewError(connect.CodeNotFound, nil)

func requestContext(tenantID uuid.UUID, userID uuid.UUID) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   userID.String(),
	})
}

func TestCreateThreadPersistsInitialMessage(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	repo := &fakeThreadRepo{}
	messages := &fakeMessageStore{}
	handler := NewHandler(repo, messages, fakeCatalog{}, fakeClassifier{})
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   userID.String(),
	})

	resp, err := handler.CreateThread(ctx, connect.NewRequest(&chatv1.CreateThreadRequest{
		TenantId:           tenantID.String(),
		Title:              "Retail post",
		InitialMessageText: "Create a LinkedIn post about retail in Portuguese",
	}))
	if err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
	if resp.Msg.GetThread().GetId() == "" {
		t.Fatal("response thread id is empty")
	}
	initial := resp.Msg.GetInitialMessage()
	if initial == nil {
		t.Fatal("initial message is nil")
	}
	if got, want := initial.GetText(), "Create a LinkedIn post about retail in Portuguese"; got != want {
		t.Fatalf("initial message text = %q, want %q", got, want)
	}
	if len(messages.appended) != 1 {
		t.Fatalf("appended messages = %d, want 1", len(messages.appended))
	}
	if got, want := messages.appended[0].ThreadID, resp.Msg.GetThread().GetId(); got != want {
		t.Fatalf("appended ThreadID = %q, want %q", got, want)
	}
}

func TestCreateThreadWithInitialMessageRequiresChatStoreBeforeCreate(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeThreadRepo{}
	handler := NewHandler(repo, nil, fakeCatalog{}, fakeClassifier{})

	_, err := handler.CreateThread(requestContext(tenantID, uuid.New()), connect.NewRequest(&chatv1.CreateThreadRequest{
		TenantId:           tenantID.String(),
		Title:              "Retail post",
		InitialMessageText: "Create a LinkedIn post about retail in Portuguese",
	}))
	if err == nil {
		t.Fatal("CreateThread returned nil error")
	}
	if got, want := connect.CodeOf(err), connect.CodeFailedPrecondition; got != want {
		t.Fatalf("error code = %v, want %v", got, want)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created threads = %d, want 0", len(repo.created))
	}
}

func TestListThreadMessagesPaginationReturnsNextTokenOnlyWhenMoreRowsExist(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.NewString()
	messages := &fakeMessageStore{
		messages: []*chatv1.ThreadMessage{
			{TenantId: tenantID.String(), ThreadId: threadID, SequenceNumber: 1, Text: "one"},
			{TenantId: tenantID.String(), ThreadId: threadID, SequenceNumber: 2, Text: "two"},
			{TenantId: tenantID.String(), ThreadId: threadID, SequenceNumber: 3, Text: "three"},
		},
	}
	handler := NewHandler(&fakeThreadRepo{}, messages, fakeCatalog{}, fakeClassifier{})

	resp, err := handler.ListThreadMessages(requestContext(tenantID, uuid.New()), connect.NewRequest(&chatv1.ListThreadMessagesRequest{
		TenantId:  tenantID.String(),
		ThreadId:  threadID,
		PageSize:  2,
		PageToken: "",
	}))
	if err != nil {
		t.Fatalf("ListThreadMessages returned error: %v", err)
	}
	if got, want := len(resp.Msg.GetMessages()), 2; got != want {
		t.Fatalf("messages = %d, want %d", got, want)
	}
	if got, want := resp.Msg.GetNextPageToken(), "2"; got != want {
		t.Fatalf("next_page_token = %q, want %q", got, want)
	}
}

func TestListThreadMessagesPaginationOmitsNextTokenForExactPage(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.NewString()
	messages := &fakeMessageStore{
		messages: []*chatv1.ThreadMessage{
			{TenantId: tenantID.String(), ThreadId: threadID, SequenceNumber: 1, Text: "one"},
			{TenantId: tenantID.String(), ThreadId: threadID, SequenceNumber: 2, Text: "two"},
		},
	}
	handler := NewHandler(&fakeThreadRepo{}, messages, fakeCatalog{}, fakeClassifier{})

	resp, err := handler.ListThreadMessages(requestContext(tenantID, uuid.New()), connect.NewRequest(&chatv1.ListThreadMessagesRequest{
		TenantId: tenantID.String(),
		ThreadId: threadID,
		PageSize: 2,
	}))
	if err != nil {
		t.Fatalf("ListThreadMessages returned error: %v", err)
	}
	if got, want := len(resp.Msg.GetMessages()), 2; got != want {
		t.Fatalf("messages = %d, want %d", got, want)
	}
	if got := resp.Msg.GetNextPageToken(); got != "" {
		t.Fatalf("next_page_token = %q, want empty", got)
	}
}

func TestListThreadMessagesRejectsInvalidPageToken(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.NewString()
	handler := NewHandler(&fakeThreadRepo{}, &fakeMessageStore{}, fakeCatalog{}, fakeClassifier{})

	for _, token := range []string{"not-a-number", "-1"} {
		_, err := handler.ListThreadMessages(requestContext(tenantID, uuid.New()), connect.NewRequest(&chatv1.ListThreadMessagesRequest{
			TenantId:  tenantID.String(),
			ThreadId:  threadID,
			PageToken: token,
		}))
		if err == nil {
			t.Fatalf("ListThreadMessages(%q) returned nil error", token)
		}
		if got, want := connect.CodeOf(err), connect.CodeInvalidArgument; got != want {
			t.Fatalf("ListThreadMessages(%q) error code = %v, want %v", token, got, want)
		}
	}
}

func TestAppendThreadMessageRejectsUnspecifiedRoleAndKind(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.NewString()
	handler := NewHandler(&fakeThreadRepo{}, &fakeMessageStore{}, fakeCatalog{}, fakeClassifier{})

	cases := []struct {
		name string
		req  *chatv1.AppendThreadMessageRequest
	}{
		{
			name: "role",
			req: &chatv1.AppendThreadMessageRequest{
				TenantId: tenantID.String(),
				ThreadId: threadID,
				Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
			},
		},
		{
			name: "kind",
			req: &chatv1.AppendThreadMessageRequest{
				TenantId: tenantID.String(),
				ThreadId: threadID,
				Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler.AppendThreadMessage(requestContext(tenantID, uuid.New()), connect.NewRequest(tc.req))
			if err == nil {
				t.Fatal("AppendThreadMessage returned nil error")
			}
			if got, want := connect.CodeOf(err), connect.CodeInvalidArgument; got != want {
				t.Fatalf("error code = %v, want %v", got, want)
			}
		})
	}
}

func TestAppendThreadMessageSetsAuthorUserID(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	threadID := uuid.NewString()
	messages := &fakeMessageStore{}
	handler := NewHandler(&fakeThreadRepo{}, messages, fakeCatalog{}, fakeClassifier{})

	resp, err := handler.AppendThreadMessage(requestContext(tenantID, userID), connect.NewRequest(&chatv1.AppendThreadMessageRequest{
		TenantId: tenantID.String(),
		ThreadId: threadID,
		Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:     "hello",
	}))
	if err != nil {
		t.Fatalf("AppendThreadMessage returned error: %v", err)
	}
	if got, want := resp.Msg.GetMessage().GetAuthorUserId(), userID.String(); got != want {
		t.Fatalf("author_user_id = %q, want %q", got, want)
	}
	if len(messages.appended) != 1 {
		t.Fatalf("appended messages = %d, want 1", len(messages.appended))
	}
	if messages.appended[0].AuthorUserID == nil {
		t.Fatal("append input AuthorUserID is nil")
	}
	if got, want := messages.appended[0].AuthorUserID.String(), userID.String(); got != want {
		t.Fatalf("append input AuthorUserID = %q, want %q", got, want)
	}
}

func TestMapRepositoryErrorMapsNoRowsToNotFound(t *testing.T) {
	err := mapRepositoryError(errors.Join(errors.New("threads: get"), pgx.ErrNoRows))
	if got, want := connect.CodeOf(err), connect.CodeNotFound; got != want {
		t.Fatalf("no rows error code = %v, want %v", got, want)
	}

	err = mapRepositoryError(errors.New("other"))
	if got, want := connect.CodeOf(err), connect.CodeInternal; got != want {
		t.Fatalf("other error code = %v, want %v", got, want)
	}
}

type fakeCatalog struct {
	summaries []copilot.TemplateSummary
}

func (f fakeCatalog) ListTemplateSummaries(ctx context.Context) ([]copilot.TemplateSummary, error) {
	return f.summaries, nil
}

type fakeClassifier struct {
	result copilot.ClassifyResult
}

func (f fakeClassifier) Classify(ctx context.Context, in copilot.ClassifyInput) (copilot.ClassifyResult, error) {
	return f.result, nil
}

func TestProposePlanEmitsPlanProposed(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	tplID := uuid.New()
	messages := &fakeMessageStore{}
	// Seed a latest USER_TEXT in the thread.
	_, _ = messages.AppendMessage(context.Background(), tenantID, chat.AppendInput{
		ThreadID: threadID.String(),
		Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:     "Create a LinkedIn post about retail",
	})
	catalog := fakeCatalog{summaries: []copilot.TemplateSummary{{ID: tplID, Key: "linkedin", Name: "LinkedIn Post"}}}
	classifier := fakeClassifier{result: copilot.ClassifyResult{
		Candidates: []copilot.Candidate{{TemplateID: tplID, Confidence: 0.9, InputValuesJSON: `{"theme":"retail"}`}},
		Summary:    "Create a LinkedIn post about retail",
	}}
	h := NewHandler(&fakeThreadRepo{}, messages, catalog, classifier)
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})

	resp, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err != nil {
		t.Fatalf("ProposePlan: %v", err)
	}
	if resp.Msg.Message.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_PROPOSED {
		t.Fatalf("kind = %v", resp.Msg.Message.GetKind())
	}
	if !strings.Contains(resp.Msg.Message.GetPayloadJson(), "linkedin") {
		t.Fatalf("payload = %s", resp.Msg.Message.GetPayloadJson())
	}
	if !strings.Contains(resp.Msg.Message.GetPayloadJson(), "retail") {
		t.Fatalf("payload missing inferred inputs: %s", resp.Msg.Message.GetPayloadJson())
	}
	if !strings.Contains(resp.Msg.Message.GetPayloadJson(), "Create a LinkedIn post about retail") {
		t.Fatalf("payload missing summary: %s", resp.Msg.Message.GetPayloadJson())
	}
}

func TestProposePlanEmptySummaryOmittedGracefully(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	tplID := uuid.New()
	messages := &fakeMessageStore{}
	_, _ = messages.AppendMessage(context.Background(), tenantID, chat.AppendInput{
		ThreadID: threadID.String(),
		Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:     "Create a LinkedIn post",
	})
	catalog := fakeCatalog{summaries: []copilot.TemplateSummary{{ID: tplID, Key: "linkedin", Name: "LinkedIn Post"}}}
	classifier := fakeClassifier{result: copilot.ClassifyResult{
		Candidates: []copilot.Candidate{{TemplateID: tplID, Confidence: 0.9}},
		Summary:    "",
	}}
	h := NewHandler(&fakeThreadRepo{}, messages, catalog, classifier)
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})

	resp, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err != nil {
		t.Fatalf("ProposePlan: %v", err)
	}
	payload := resp.Msg.Message.GetPayloadJson()
	if strings.Contains(payload, `"summary":"`) && !strings.Contains(payload, `"summary":""`) {
		t.Fatalf("expected empty summary omitted or empty, got %s", payload)
	}
}

func TestProposePlanRequiresUserMessage(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	h := NewHandler(&fakeThreadRepo{}, &fakeMessageStore{}, fakeCatalog{}, fakeClassifier{})
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})
	_, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err == nil {
		t.Fatal("expected error when no user message exists")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", connect.CodeOf(err))
	}
}

func TestProposePlanEmptyCandidatesStillEmits(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	messages := &fakeMessageStore{}
	_, _ = messages.AppendMessage(context.Background(), tenantID, chat.AppendInput{
		ThreadID: threadID.String(),
		Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:     "something off-catalog",
	})
	h := NewHandler(&fakeThreadRepo{}, messages, fakeCatalog{}, fakeClassifier{result: copilot.ClassifyResult{}})
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})
	resp, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err != nil {
		t.Fatalf("ProposePlan: %v", err)
	}
	if resp.Msg.Message.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_PROPOSED {
		t.Fatalf("kind = %v", resp.Msg.Message.GetKind())
	}
}
