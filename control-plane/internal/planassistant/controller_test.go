package planassistant_test

import (
	"context"
	"fmt"
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
		OriginThreadId: testThreadUUID,
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
	withPolicyParams(tpl, "publish_approval_mode", "elicitation_timeout_behavior")
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
	withPolicyParams(tpl, "publish_approval_mode", "elicitation_timeout_behavior")
	cfgID := uuid.NewString()
	configs := &fakeConfigs{cur: &plansv1.PlanConfiguration{
		Id:             cfgID,
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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
		OriginThreadId: testThreadUUID,
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

// storedAssistantPrompt builds an ASSISTANT_PROMPT row with a hand-crafted
// payload so tests can seed the fake chat store with prompts that have the
// same fingerprint but a different body than what the controller would derive.
func storedAssistantPrompt(configurationID, state, stepKey, bodySuffix string) *chatv1.ThreadMessage {
	body := fmt.Sprintf(
		`{"configuration_id":%q,"state":%q,"step_key":%q,"options":[{"id":"x","label":%q,"value":"x"}]}`,
		configurationID, state, stepKey, bodySuffix,
	)
	return &chatv1.ThreadMessage{
		Id:          uuid.NewString(),
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
		PayloadJson: body,
	}
}

// TestNextTurn_DedupsByFingerprintNotBytes is the core regression: a stored
// prompt with the SAME (configuration_id, state, step_key) but a DIFFERENT
// candidate body must suppress re-emission. This is exactly what happened
// after a page reload — NextTurn re-derived the same state and produced a
// byte-different but semantically identical prompt.
func TestNextTurn_DedupsByFingerprintNotBytes(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfgID := uuid.NewString()
	cfg := &plansv1.PlanConfiguration{
		Id:             cfgID,
		OriginThreadId: testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
	}
	// Same config/state/step, but a stale candidate label the controller will
	// not reproduce — under the old byte-comparison this would NOT dedup.
	chatStore.stored = []*chatv1.ThreadMessage{
		storedAssistantPrompt(cfgID, "BINDING_STEP", "draft", "stale-candidate-label"),
	}
	c := newController(chatStore, cfg, tpl)
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 0 {
		t.Fatalf("same (config,state,step) fingerprint must suppress even when body differs; appended %d: %+v",
			len(chatStore.appended), chatStore.appended)
	}
}

// TestNextTurn_DoesNotDedupAcrossConfigurations guards the multi-plan thread
// case: two configs in the same thread, both in BINDING_STEP for the same
// step_key, must each get their own prompt.
func TestNextTurn_DoesNotDedupAcrossConfigurations(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfgID := uuid.NewString()
	cfg := &plansv1.PlanConfiguration{
		Id:             cfgID,
		OriginThreadId: testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
	}
	// Same state/step, DIFFERENT configuration_id — must NOT suppress.
	chatStore.stored = []*chatv1.ThreadMessage{
		storedAssistantPrompt(uuid.NewString(), "BINDING_STEP", "draft", "other-config"),
	}
	c := newController(chatStore, cfg, tpl)
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("different configuration_id must not suppress; appended %d", len(chatStore.appended))
	}
	want := fmt.Sprintf(`"configuration_id":%q`, cfgID)
	if !strings.Contains(chatStore.appended[0].PayloadJSON, want) {
		t.Fatalf("emitted prompt must stamp configuration_id=%s, got %s", cfgID, chatStore.appended[0].PayloadJSON)
	}
}

// TestNextTurn_DedupIsStepScoped confirms the same config in the same state
// but a DIFFERENT step_key emits a fresh prompt (advancement is preserved).
func TestNextTurn_DedupIsStepScoped(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("fetch-news", "write-draft")
	tpl.Id = testTemplateUUID
	cfgID := uuid.NewString()
	cfg := &plansv1.PlanConfiguration{
		Id:             cfgID,
		OriginThreadId: testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"}},
	}
	// Already prompted for fetch-news; we're now asking about write-draft.
	chatStore.stored = []*chatv1.ThreadMessage{
		storedAssistantPrompt(cfgID, "BINDING_STEP", "fetch-news", "earlier"),
	}
	c := newControllerWithCatalog(chatStore, cfg, tpl, fakeCatalog{byStep: map[string][]planassistant.ExecutorOption{
		"fetch-news":  {{StepKey: "fetch-news", InstallationID: "rss-tech", DisplayName: "Tech RSS", SkuKey: "rss-news-feed"}},
		"write-draft": {{StepKey: "write-draft", InstallationID: "writer", DisplayName: "Writer", SkuKey: "newsletter-writer-senior"}},
	}})
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("different step_key must emit; appended %d", len(chatStore.appended))
	}
	if !strings.Contains(chatStore.appended[0].PayloadJSON, `"step_key":"write-draft"`) {
		t.Fatalf("expected write-draft prompt, got %s", chatStore.appended[0].PayloadJSON)
	}
}

// TestNextTurn_LandingIsConfigScoped guards the multi-plan thread case for
// landings: a landing for config A in a shared thread must not block config
// B's landing.
func TestNextTurn_LandingIsConfigScoped(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	tpl.Id = testTemplateUUID
	cfgID := uuid.NewString()
	cfg := &plansv1.PlanConfiguration{
		Id:             cfgID,
		OriginThreadId: testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}
	// A landing for a DIFFERENT config in the same thread — old thread-global
	// check suppressed us; the config-scoped check must not.
	chatStore.stored = []*chatv1.ThreadMessage{
		storedAssistantPrompt(uuid.NewString(), "landing", "", "other-config"),
	}
	c := newController(chatStore, cfg, tpl)
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("landing for another config must not suppress; appended %d", len(chatStore.appended))
	}
	if !strings.Contains(chatStore.appended[0].PayloadJSON, `"state":"landing"`) {
		t.Fatalf("expected landing payload, got %s", chatStore.appended[0].PayloadJSON)
	}
	if !strings.Contains(chatStore.appended[0].PayloadJSON, fmt.Sprintf(`"configuration_id":%q`, cfgID)) {
		t.Fatalf("emitted landing must stamp configuration_id=%s, got %s", cfgID, chatStore.appended[0].PayloadJSON)
	}
}

// TestNextTurn_ProgressionEmitsExactlyOnePromptPerStep walks the full happy
// path (binding1 → binding2 → overseer → policies → matrix → landing) and
// re-fires NextTurn after every step to confirm each emits exactly once.
func TestNextTurn_ProgressionEmitsExactlyOnePromptPerStep(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("fetch-news", "write-draft")
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	tpl.Id = testTemplateUUID
	withPolicyParams(tpl, "publish_approval_mode", "elicitation_timeout_behavior")
	cfgID := uuid.NewString()
	configs := &fakeConfigs{cur: &plansv1.PlanConfiguration{
		Id:             cfgID,
		OriginThreadId: testThreadUUID,
		PlanTemplateId: testTemplateUUID,
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
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
	tenant := uuid.New()

	// Step 1: BINDING_STEP/fetch-news. Re-fire to confirm idempotency.
	mustNextTurn(t, c, ctx, tenant)
	mustNextTurn(t, c, ctx, tenant)
	assertStateStep(t, chatStore.appended, 0, "BINDING_STEP", "fetch-news")
	if len(chatStore.appended) != 1 {
		t.Fatalf("step 1: expected 1 append, got %d", len(chatStore.appended))
	}

	// Step 2: BINDING_STEP/write-draft.
	configs.cur = &plansv1.PlanConfiguration{
		Id: cfgID, OriginThreadId: testThreadUUID, PlanTemplateId: testTemplateUUID,
		Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
		},
	}
	mustNextTurn(t, c, ctx, tenant)
	mustNextTurn(t, c, ctx, tenant)
	assertStateStep(t, chatStore.appended, 1, "BINDING_STEP", "write-draft")
	if len(chatStore.appended) != 2 {
		t.Fatalf("step 2: expected 2 appends, got %d", len(chatStore.appended))
	}

	// Step 3: OVERSEER_STEP/write-draft (write-draft is agent-backed).
	configs.cur = &plansv1.PlanConfiguration{
		Id: cfgID, OriginThreadId: testThreadUUID, PlanTemplateId: testTemplateUUID,
		Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
	}
	mustNextTurn(t, c, ctx, tenant)
	mustNextTurn(t, c, ctx, tenant)
	assertStateStep(t, chatStore.appended, 2, "OVERSEER_STEP", "write-draft")
	if len(chatStore.appended) != 3 {
		t.Fatalf("step 3: expected 3 appends, got %d", len(chatStore.appended))
	}

	// Step 4: POLICIES_STEP.
	configs.cur = &plansv1.PlanConfiguration{
		Id: cfgID, OriginThreadId: testThreadUUID, PlanTemplateId: testTemplateUUID,
		Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
	}
	mustNextTurn(t, c, ctx, tenant)
	mustNextTurn(t, c, ctx, tenant)
	if !strings.Contains(chatStore.appended[3].PayloadJSON, `"state":"POLICIES_STEP"`) {
		t.Fatalf("step 4: expected POLICIES_STEP, got %s", chatStore.appended[3].PayloadJSON)
	}
	if len(chatStore.appended) != 4 {
		t.Fatalf("step 4: expected 4 appends, got %d", len(chatStore.appended))
	}

	// Step 5: BINDING_MATRIX.
	configs.cur = &plansv1.PlanConfiguration{
		Id: cfgID, OriginThreadId: testThreadUUID, PlanTemplateId: testTemplateUUID,
		Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
		BehaviorPolicies: validPolicies(),
	}
	mustNextTurn(t, c, ctx, tenant)
	mustNextTurn(t, c, ctx, tenant)
	if !strings.Contains(chatStore.appended[4].PayloadJSON, `"state":"BINDING_MATRIX"`) {
		t.Fatalf("step 5: expected BINDING_MATRIX, got %s", chatStore.appended[4].PayloadJSON)
	}
	if len(chatStore.appended) != 5 {
		t.Fatalf("step 5: expected 5 appends, got %d", len(chatStore.appended))
	}

	// Step 6: landing on promotion to RUNNABLE. Re-fire to confirm idempotency.
	configs.cur = &plansv1.PlanConfiguration{
		Id: cfgID, OriginThreadId: testThreadUUID, PlanTemplateId: testTemplateUUID,
		Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "rss-tech"},
			{StepKey: "write-draft", ExecutorInstallationId: "writer"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
		BehaviorPolicies: validPolicies(),
	}
	mustNextTurn(t, c, ctx, tenant)
	mustNextTurn(t, c, ctx, tenant)
	if !strings.Contains(chatStore.appended[5].PayloadJSON, `"state":"landing"`) {
		t.Fatalf("step 6: expected landing, got %s", chatStore.appended[5].PayloadJSON)
	}
	if got := len(chatStore.appended); got != 6 {
		t.Fatalf("step 6: expected exactly 6 appends (one per step), got %d", got)
	}
}

func mustNextTurn(t *testing.T, c *planassistant.Controller, ctx context.Context, tenant uuid.UUID) {
	t.Helper()
	if err := c.NextTurn(ctx, tenant, uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func assertStateStep(t *testing.T, appended []chat.AppendInput, idx int, state, stepKey string) {
	t.Helper()
	if idx >= len(appended) {
		t.Fatalf("assertStateStep: index %d out of range (len=%d)", idx, len(appended))
	}
	got := appended[idx].PayloadJSON
	if !strings.Contains(got, fmt.Sprintf(`"state":%q`, state)) {
		t.Fatalf("appended[%d]: expected state %s, got %s", idx, state, got)
	}
	if !strings.Contains(got, fmt.Sprintf(`"step_key":%q`, stepKey)) {
		t.Fatalf("appended[%d]: expected step_key %s, got %s", idx, stepKey, got)
	}
}
