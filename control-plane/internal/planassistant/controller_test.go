package planassistant_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/planassistant"
)

const testTemplateUUID = "11111111-1111-1111-1111-111111111111"

type fakeChat struct {
	appended []chat.AppendInput
	listing  []*chatv1.ThreadMessage
}

func (f *fakeChat) AppendMessage(_ context.Context, _ uuid.UUID, in chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, in)
	return &chatv1.ThreadMessage{Id: uuid.NewString(), Kind: in.Kind, PayloadJson: in.PayloadJSON, Text: in.Text}, nil
}
func (f *fakeChat) ListMessages(_ context.Context, _ uuid.UUID, _ string, _ int64, _ int) ([]*chatv1.ThreadMessage, error) {
	return f.listing, nil
}

type fakeConfigs struct{ cur *plansv1.PlanConfiguration }

func (f *fakeConfigs) GetConfiguration(_ context.Context, _, _ uuid.UUID) (*plansv1.PlanConfiguration, error) {
	return f.cur, nil
}
func (f *fakeConfigs) UpdateFromSelection(_ context.Context, _, _ uuid.UUID, _ planassistant.AssistantState, _ string) (*plansv1.PlanConfiguration, error) {
	return f.cur, nil
}

type fakeCatalog struct{}

func (fakeCatalog) CandidatesForStep(_ context.Context, _ uuid.UUID, _ *plansv1.PlanTemplate, _ string) ([]planassistant.ExecutorOption, error) {
	return nil, nil
}

type fakeTemplates struct{ tpl *plansv1.PlanTemplate }

func (f *fakeTemplates) GetTemplateByID(_ context.Context, _ uuid.UUID) (*plansv1.PlanTemplate, error) {
	return f.tpl, nil
}

func TestSeedThread_EmitsConfigurationStartedThenFirstPrompt(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{Id: uuid.NewString(), PlanTemplateId: testTemplateUUID}
	c := &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   fakeCatalog{},
		Configs:   &fakeConfigs{cur: cfg},
		Templates: &fakeTemplates{tpl: tpl},
	}
	if err := c.SeedThread(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(chatStore.appended))
	}
	if chatStore.appended[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED {
		t.Fatalf("first message kind: %v", chatStore.appended[0].Kind)
	}
	if chatStore.appended[1].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
		t.Fatalf("second message kind: %v", chatStore.appended[1].Kind)
	}
}

func TestNextTurn_AdvancesAfterSelection(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft", "publish")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		PlanTemplateId: testTemplateUUID,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "draft", ExecutorInstallationId: "inst"}, {StepKey: "publish", ExecutorInstallationId: "inst2"}},
	}
	c := &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   fakeCatalog{},
		Configs:   &fakeConfigs{cur: cfg},
		Templates: &fakeTemplates{tpl: tpl},
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("expected 1 new prompt, got %d", len(chatStore.appended))
	}
	if chatStore.appended[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
		t.Fatalf("expected ASSISTANT_PROMPT, got %v", chatStore.appended[0].Kind)
	}
}
