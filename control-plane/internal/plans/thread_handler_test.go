package plans

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/identity"
)

// fakeChatStore implements chat.Store in-memory for handler tests.
type fakeChatStore struct {
	appended []chat.AppendInput
	messages map[string][]*chatv1.ThreadMessage
}

func (f *fakeChatStore) AppendMessage(ctx context.Context, tenantID uuid.UUID, input chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, input)
	if f.messages == nil {
		f.messages = make(map[string][]*chatv1.ThreadMessage)
	}
	msg := &chatv1.ThreadMessage{
		Id:             uuid.New().String(),
		TenantId:       tenantID.String(),
		ThreadId:       input.ThreadID,
		Role:           input.Role,
		Kind:           input.Kind,
		Text:           input.Text,
		PayloadJson:    input.PayloadJSON,
		SequenceNumber: int64(len(f.messages[input.ThreadID]) + 1),
	}
	if input.ExecutionID != nil {
		msg.ExecutionId = input.ExecutionID.String()
	}
	f.messages[input.ThreadID] = append(f.messages[input.ThreadID], msg)
	return msg, nil
}

func (f *fakeChatStore) ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
	all := f.messages[threadID]
	out := make([]*chatv1.ThreadMessage, 0, len(all))
	for _, m := range all {
		if m.SequenceNumber > sinceSeq {
			out = append(out, m)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestAppendPlanThreadMessage_PersistsOverseerText(t *testing.T) {
	tenantID := uuid.New()
	configID := uuid.New()
	store := &fakeChatStore{}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.AppendPlanThreadMessageRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
		Role:                chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:                chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:                "remember to update the news source URL next week",
	})

	resp, err := h.AppendPlanThreadMessage(ctx, req)
	if err != nil {
		t.Fatalf("AppendPlanThreadMessage returned error: %v", err)
	}
	if got, want := len(store.appended), 1; got != want {
		t.Fatalf("store.AppendMessage call count = %d, want %d", got, want)
	}
	if got := store.appended[0].ThreadID; got != configID.String() {
		t.Fatalf("ThreadID = %q, want %q", got, configID.String())
	}
	if resp.Msg.Message.SequenceNumber != 1 {
		t.Fatalf("response sequence_number = %d, want 1", resp.Msg.Message.SequenceNumber)
	}
}

func TestAppendPlanThreadMessage_RejectsMissingConfigID(t *testing.T) {
	tenantID := uuid.New()
	store := &fakeChatStore{}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.AppendPlanThreadMessageRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: "",
		Role:                chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:                chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:                "hi",
	})

	_, err := h.AppendPlanThreadMessage(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing plan_configuration_id, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", got)
	}
}

func TestListPlanThreadMessages_FiltersBySinceSequence(t *testing.T) {
	tenantID := uuid.New()
	configID := uuid.New()
	store := &fakeChatStore{messages: map[string][]*chatv1.ThreadMessage{
		configID.String(): {
			{SequenceNumber: 1, Text: "first"},
			{SequenceNumber: 2, Text: "second"},
			{SequenceNumber: 3, Text: "third"},
		},
	}}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.ListPlanThreadMessagesRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
	})

	resp, err := h.ListPlanThreadMessages(ctx, req)
	if err != nil {
		t.Fatalf("ListPlanThreadMessages returned error: %v", err)
	}
	if got, want := len(resp.Msg.Messages), 3; got != want {
		t.Fatalf("messages count = %d, want %d", got, want)
	}
}

func TestListPlanThreadMessages_PaginatesWithPageToken(t *testing.T) {
	tenantID := uuid.New()
	configID := uuid.New()
	store := &fakeChatStore{messages: map[string][]*chatv1.ThreadMessage{
		configID.String(): {
			{SequenceNumber: 1, Text: "first"},
			{SequenceNumber: 2, Text: "second"},
			{SequenceNumber: 3, Text: "third"},
		},
	}}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})

	page1, err := h.ListPlanThreadMessages(ctx, connect.NewRequest(&plansv1.ListPlanThreadMessagesRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
		PageSize:            2,
	}))
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if got, want := len(page1.Msg.Messages), 2; got != want {
		t.Fatalf("page 1 count = %d, want %d", got, want)
	}
	if got, want := page1.Msg.Messages[0].Text, "first"; got != want {
		t.Fatalf("page 1 first = %q, want %q", got, want)
	}
	if got, want := page1.Msg.NextPageToken, "2"; got != want {
		t.Fatalf("page 1 next token = %q, want %q", got, want)
	}

	page2, err := h.ListPlanThreadMessages(ctx, connect.NewRequest(&plansv1.ListPlanThreadMessagesRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
		PageSize:            2,
		PageToken:           page1.Msg.NextPageToken,
	}))
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if got, want := len(page2.Msg.Messages), 1; got != want {
		t.Fatalf("page 2 count = %d, want %d", got, want)
	}
	if got, want := page2.Msg.Messages[0].Text, "third"; got != want {
		t.Fatalf("page 2 first = %q, want %q", got, want)
	}
	if page2.Msg.NextPageToken != "" {
		t.Fatalf("page 2 next token = %q, want empty", page2.Msg.NextPageToken)
	}
}

func TestListPlanThreadMessages_RejectsInvalidPageToken(t *testing.T) {
	tenantID := uuid.New()
	configID := uuid.New()
	h := &PlanHandler{chat: &fakeChatStore{}}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.ListPlanThreadMessagesRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
		PageToken:           "not-a-number",
	})

	_, err := h.ListPlanThreadMessages(ctx, req)
	if err == nil {
		t.Fatal("expected error for invalid page_token, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", got)
	}
}
