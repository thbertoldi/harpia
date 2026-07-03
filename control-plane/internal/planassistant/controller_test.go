package planassistant_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/planassistant"
)

const testTemplateUUID = "11111111-1111-1111-1111-111111111111"
const testThreadUUID = "33333333-3333-3333-3333-333333333333"

// fakeChat records appended messages and serves them back from ListMessages
// so the controller's dedup / idempotency paths are exercised realistically.
type fakeChat struct {
	appended []chat.AppendInput
	stored   []*chatv1.ThreadMessage
}

func (f *fakeChat) AppendMessage(_ context.Context, _ uuid.UUID, in chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, in)
	msg := &chatv1.ThreadMessage{Id: uuid.NewString(), Kind: in.Kind, PayloadJson: in.PayloadJSON, Text: in.Text}
	f.stored = append(f.stored, msg)
	return msg, nil
}
func (f *fakeChat) ListMessages(_ context.Context, _ uuid.UUID, _ string, _ int64, _ int) ([]*chatv1.ThreadMessage, error) {
	return f.stored, nil
}

type fakeConfigs struct{ cur *plansv1.PlanConfiguration }

func (f *fakeConfigs) GetConfiguration(_ context.Context, _, _ uuid.UUID) (*plansv1.PlanConfiguration, error) {
	return f.cur, nil
}

type fakeCatalog struct {
	byStep map[string][]planassistant.ExecutorOption
}

func (f fakeCatalog) CandidatesForStep(_ context.Context, _ uuid.UUID, _ *plansv1.PlanTemplate, stepKey string) ([]planassistant.ExecutorOption, error) {
	return f.byStep[stepKey], nil
}

type fakeTemplates struct{ tpl *plansv1.PlanTemplate }

func (f *fakeTemplates) GetTemplateByID(_ context.Context, _ uuid.UUID) (*plansv1.PlanTemplate, error) {
	return f.tpl, nil
}

func newController(chatStore chat.Store, cfg *plansv1.PlanConfiguration, tpl *plansv1.PlanTemplate) *planassistant.Controller {
	return &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   fakeCatalog{},
		Configs:   &fakeConfigs{cur: cfg},
		Templates: &fakeTemplates{tpl: tpl},
	}
}

func newControllerWithCatalog(chatStore chat.Store, cfg *plansv1.PlanConfiguration, tpl *plansv1.PlanTemplate, catalog fakeCatalog) *planassistant.Controller {
	return &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   catalog,
		Configs:   &fakeConfigs{cur: cfg},
		Templates: &fakeTemplates{tpl: tpl},
	}
}

func TestSeedThread_EmitsConfigurationStartedThenBindingStep(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
	}
	c := newController(chatStore, cfg, tpl)
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
	if !strings.Contains(chatStore.appended[1].PayloadJSON, `"state":"BINDING_STEP"`) {
		t.Fatalf("first prompt should be the focused binding step: %s", chatStore.appended[1].PayloadJSON)
	}
	// Regression: assistant messages must be appended to the OWNING THREAD, not
	// to the plan-configuration id (caused a chat_messages FK violation).
	for i, m := range chatStore.appended {
		if m.ThreadID != testThreadUUID || m.ThreadID == cfg.Id {
			t.Fatalf("appended[%d] ThreadID = %q, want owning thread %q (not config id %q)", i, m.ThreadID, testThreadUUID, cfg.Id)
		}
	}
}

func TestNextTurn_AdvancesThroughUnboundStepsThenMatrix(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("fetch-news", "write-draft")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
	}
	configs := &fakeConfigs{cur: cfg}
	c := &planassistant.Controller{
		Chat: chatStore,
		Catalog: fakeCatalog{byStep: map[string][]planassistant.ExecutorOption{
			"fetch-news":  {{StepKey: "fetch-news", InstallationID: "rss-tech", DisplayName: "Tech RSS", SkuKey: "rss-news-feed"}},
			"write-draft": {{StepKey: "write-draft", InstallationID: "writer", DisplayName: "Writer", SkuKey: "newsletter-writer-senior"}},
		}},
		Configs:   configs,
		Templates: &fakeTemplates{tpl: tpl},
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[0].PayloadJSON, `"state":"BINDING_STEP"`) ||
		!strings.Contains(chatStore.appended[0].PayloadJSON, `"step_key":"fetch-news"`) {
		t.Fatalf("expected fetch-news binding step, got %s", chatStore.appended[0].PayloadJSON)
	}

	configs.cur = &plansv1.PlanConfiguration{
		Id:             cfg.Id,
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"}},
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[1].PayloadJSON, `"state":"BINDING_STEP"`) ||
		!strings.Contains(chatStore.appended[1].PayloadJSON, `"step_key":"write-draft"`) {
		t.Fatalf("expected write-draft binding step, got %s", chatStore.appended[1].PayloadJSON)
	}

	configs.cur = &plansv1.PlanConfiguration{
		Id:             cfg.Id,
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[2].PayloadJSON, `"state":"POLICIES_STEP"`) {
		t.Fatalf("expected policies after all steps are bound, got %s", chatStore.appended[2].PayloadJSON)
	}

	configs.cur = &plansv1.PlanConfiguration{
		Id:             cfg.Id,
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
			PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
		},
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[3].PayloadJSON, `"state":"BINDING_MATRIX"`) {
		t.Fatalf("expected review matrix after policies are set, got %s", chatStore.appended[3].PayloadJSON)
	}
}

func TestNextTurn_AdvancesFromBindingsToOverseerThenMatrix(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("fetch-news", "write-draft")
	tpl.Id = testTemplateUUID
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	cfgID := uuid.NewString()
	configs := &fakeConfigs{cur: &plansv1.PlanConfiguration{
		Id:             cfgID,
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
	}}
	c := &planassistant.Controller{
		Chat: chatStore,
		Catalog: fakeCatalog{byStep: map[string][]planassistant.ExecutorOption{
			"fetch-news":  {{StepKey: "fetch-news", InstallationID: "rss-tech", DisplayName: "Tech RSS", SkuKey: "rss-news-feed"}},
			"write-draft": {{StepKey: "write-draft", InstallationID: "writer", DisplayName: "Writer", SkuKey: "newsletter-writer-senior"}},
		}},
		Configs:   configs,
		Templates: &fakeTemplates{tpl: tpl},
	}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{UserID: "user-ana"})
	if err := c.NextTurn(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[0].PayloadJSON, `"state":"OVERSEER_STEP"`) ||
		!strings.Contains(chatStore.appended[0].PayloadJSON, `"step_key":"write-draft"`) ||
		!strings.Contains(chatStore.appended[0].PayloadJSON, `"value":"user-ana"`) {
		t.Fatalf("expected write-draft overseer step with current user option, got %s", chatStore.appended[0].PayloadJSON)
	}

	configs.cur = &plansv1.PlanConfiguration{
		Id:             cfgID,
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
	}
	if err := c.NextTurn(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[1].PayloadJSON, `"state":"POLICIES_STEP"`) {
		t.Fatalf("expected policies after overseer is bound, got %s", chatStore.appended[1].PayloadJSON)
	}

	configs.cur = &plansv1.PlanConfiguration{
		Id:             cfgID,
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
			PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
		},
	}
	if err := c.NextTurn(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chatStore.appended[2].PayloadJSON, `"state":"BINDING_MATRIX"`) {
		t.Fatalf("expected review matrix after policies are set, got %s", chatStore.appended[2].PayloadJSON)
	}
}

func TestNextTurn_DedupsIdenticalMatrix(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
	}
	c := newController(chatStore, cfg, tpl)
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("identical matrix should be deduped to 1, got %d", len(chatStore.appended))
	}
}

func TestNextTurn_EmitsLandingOnPromotion(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}
	c := newController(chatStore, cfg, tpl)
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("expected 1 landing prompt, got %d", len(chatStore.appended))
	}
	if !strings.Contains(chatStore.appended[0].PayloadJSON, `"state":"landing"`) {
		t.Fatalf("expected landing payload, got %s", chatStore.appended[0].PayloadJSON)
	}
}

func TestNextTurn_LandingIsIdempotent(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		ThreadId:       testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}
	c := newController(chatStore, cfg, tpl)
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("landing should be emitted at most once, got %d", len(chatStore.appended))
	}
}
