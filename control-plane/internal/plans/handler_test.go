package plans

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/planassistant"
)

// TestCreatePlanConfiguration_RequiresThreadID guards the Path B contract: a
// plan configuration must be created against an owning thread. thread_id is
// validated before any repository access, so this exercises the handler without
// a database.
func TestCreatePlanConfiguration_RequiresThreadID(t *testing.T) {
	tenantID := uuid.New()
	h := &PlanHandler{}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})

	_, err := h.CreatePlanConfiguration(ctx, connect.NewRequest(&plansv1.CreatePlanConfigurationRequest{
		TenantId:       tenantID.String(),
		PlanTemplateId: uuid.New().String(),
		ThreadId:       "",
	}))
	if err == nil {
		t.Fatal("expected error for missing thread_id, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", got)
	}
}

func TestNextTurn_InvalidConfigID(t *testing.T) {
	tenantID := uuid.New()
	h := &PlanHandler{}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})

	_, err := h.NextTurn(ctx, connect.NewRequest(&plansv1.NextTurnRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: "not-a-uuid",
	}))
	if err == nil {
		t.Fatal("expected error for invalid config id, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", got)
	}
}

// TestNextTurn_NilAssistantNoOps guards the dev path where no planassistant is
// wired: a valid request returns success without panicking. The actual
// prompt-emission logic is covered by internal/planassistant/controller_test.go.
func TestNextTurn_NilAssistantNoOps(t *testing.T) {
	tenantID := uuid.New()
	h := &PlanHandler{}
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID,
		Roles:    []string{"Overseer"},
	})

	resp, err := h.NextTurn(ctx, connect.NewRequest(&plansv1.NextTurnRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: uuid.New().String(),
	}))
	if err != nil {
		t.Fatalf("NextTurn with nil assistant: %v", err)
	}
	if resp == nil || resp.Msg == nil {
		t.Fatal("expected non-nil NextTurn response")
	}
}

func TestConfigurationThreadIDPrefersOriginThread(t *testing.T) {
	originThreadID := uuid.New()

	got := configurationThreadID(&PlanConfiguration{
		OriginThreadID: originThreadID,
	})
	if got != originThreadID {
		t.Fatalf("configurationThreadID() = %s, want origin thread %s", got, originThreadID)
	}
}

func TestConfigurationThreadIDReturnsNilWithoutOriginThread(t *testing.T) {
	got := configurationThreadID(&PlanConfiguration{})
	if got != uuid.Nil {
		t.Fatalf("configurationThreadID() = %s, want nil", got)
	}
}

func TestConfigurationFromProtoPreservesOriginThread(t *testing.T) {
	originThreadID := uuid.New()
	existing := &PlanConfiguration{
		ID:                  uuid.New(),
		TenantID:            uuid.New(),
		PlanTemplateID:      uuid.New(),
		PlanTemplateVersion: 1,
		Kind:                ConfigurationKindRecurring,
		OriginThreadID:      originThreadID,
	}

	got, err := configurationFromProto(&plansv1.PlanConfiguration{
		Status:              plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		Kind:                plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_ONE_SHOT,
		OriginThreadId:      uuid.New().String(),
		ParameterValuesJson: `{}`,
	}, existing)
	if err != nil {
		t.Fatalf("configurationFromProto() error = %v", err)
	}
	if got.Kind != existing.Kind {
		t.Fatalf("Kind = %q, want %q", got.Kind, existing.Kind)
	}
	if got.OriginThreadID != existing.OriginThreadID {
		t.Fatalf("OriginThreadID = %s, want %s", got.OriginThreadID, existing.OriginThreadID)
	}
}

func TestPlanHandlerMaterializesConfigurationFromParameterValuesOnly(t *testing.T) {
	tenantID := uuid.New()
	templateID := uuid.New()
	sourceGroupID := uuid.New()
	writerSKUID := uuid.New()
	writerInstallationID := uuid.New()
	voiceSKUID := uuid.New()
	voiceInstallationID := uuid.New()
	publisherSKUID := uuid.New()
	publisherInstallationID := uuid.New()
	manifestID := "newsletter-writer-senior"
	manifestVersion := "1.0.0"

	h := &PlanHandler{
		executors: &mockExecutorLookup{
			installations: map[uuid.UUID]*executors.ExecutorInstallation{
				sourceGroupID: {
					ID:               sourceGroupID,
					TenantID:         tenantID,
					ExecutorSKUID:    uuid.New(),
					Kind:             executors.KindIntegration,
					Enabled:          true,
					ConnectionStatus: ptr("connected"),
					ConfigJSON:       json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
				},
				writerInstallationID: {
					ID:              writerInstallationID,
					TenantID:        tenantID,
					ExecutorSKUID:   writerSKUID,
					Kind:            executors.KindAgent,
					Enabled:         true,
					ManifestID:      &manifestID,
					ManifestVersion: &manifestVersion,
				},
				voiceInstallationID: {
					ID:              voiceInstallationID,
					TenantID:        tenantID,
					ExecutorSKUID:   voiceSKUID,
					Kind:            executors.KindAgent,
					Enabled:         true,
					ManifestID:      ptr("linkedin-voice-senior"),
					ManifestVersion: &manifestVersion,
				},
				publisherInstallationID: {
					ID:               publisherInstallationID,
					TenantID:         tenantID,
					ExecutorSKUID:    publisherSKUID,
					Kind:             executors.KindIntegration,
					Enabled:          true,
					ConnectionStatus: ptr("connected"),
					ConfigJSON:       json.RawMessage(`{"oauth_credential_id":"cred-1"}`),
				},
			},
			skus: map[uuid.UUID]*executors.ExecutorSKU{
				writerSKUID:    {ID: writerSKUID, Key: "newsletter-writer-senior", Kind: executors.KindAgent},
				voiceSKUID:     {ID: voiceSKUID, Key: "linkedin-voice-senior", Kind: executors.KindAgent},
				publisherSKUID: {ID: publisherSKUID, Key: "linkedin-publish", Kind: executors.KindIntegration},
			},
			entitledSKUs: map[uuid.UUID]bool{writerSKUID: true, voiceSKUID: true, publisherSKUID: true},
		},
	}
	template := handlerMaterializeTemplate(t, templateID)
	parameterValues := `{
		"theme":"AI operations",
		"language":"pt-BR",
		"tone":"analytical",
		"audience":"operators",
		"topics_to_avoid":"hype",
		"source_group":"` + sourceGroupID.String() + `",
		"date_range":{"preset":"schedule_window"},
		"approval_mode":"require_approval"
	}`

	seeds, slots, policies, err := h.materializeConfigurationProjection(context.Background(), tenantID, template, parameterValues)
	if err != nil {
		t.Fatalf("materializeConfigurationProjection() error = %v", err)
	}
	config, err := h.buildConfigurationFromRequest(
		tenantID,
		template,
		"",
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED,
		seeds,
		slots,
		nil,
		policies,
		nil,
		parameterValues,
	)
	if err != nil {
		t.Fatalf("buildConfigurationFromRequest() error = %v", err)
	}

	got := configurationToProto(config)
	if got.GetKind() != plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_ONE_SHOT {
		t.Fatalf("kind = %v, want ONE_SHOT", got.GetKind())
	}
	if findSeed(got.GetSeedArtifacts(), "fetch-news", "date_range") == nil {
		t.Fatal("missing fetch-news date_range seed")
	}
	if findSeed(got.GetSeedArtifacts(), "write-draft", "harpia.internal.ContentPreferences") == nil {
		t.Fatal("missing write-draft ContentPreferences seed")
	}
	fetchBinding := findSlot(got.GetSlotBindings(), "fetch-news")
	if fetchBinding == nil || fetchBinding.GetExecutorInstallationId() != sourceGroupID.String() {
		t.Fatalf("fetch-news binding = %#v, want source group installation", fetchBinding)
	}
	if findSlot(got.GetSlotBindings(), "write-draft") == nil ||
		findSlot(got.GetSlotBindings(), "adapt-for-linkedin") == nil ||
		findSlot(got.GetSlotBindings(), "publish-linkedin") == nil {
		t.Fatalf("slot bindings = %#v, want defaults for non-parameter steps", got.GetSlotBindings())
	}
	if got.GetBehaviorPolicies().GetPublishApprovalMode() != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL {
		t.Fatalf("publish approval mode = %v, want REQUIRE_APPROVAL", got.GetBehaviorPolicies().GetPublishApprovalMode())
	}
	assertJSONEqual(t, got.GetParameterValuesJson(), parameterValues)
}

func handlerMaterializeTemplate(t *testing.T, id uuid.UUID) *PlanTemplate {
	t.Helper()
	protoTemplate := weeklyNewsletterMaterializeTemplate()
	protoTemplate.Steps = defaultSlotBindingTemplate().GetSteps()
	return &PlanTemplate{
		ID:              id,
		Key:             protoTemplate.GetKey(),
		Name:            "Weekly Newsletter (LinkedIn)",
		Version:         1,
		InputParameters: mustMarshalTemplateInputs(t, protoTemplate.GetInputParameters()),
		Steps: []PlanStep{
			{Key: "fetch-news", DefaultExecutorSKUKey: "rss-news-feed"},
			{Key: "write-draft", DefaultExecutorSKUKey: "newsletter-writer-senior"},
			{Key: "adapt-for-linkedin", DefaultExecutorSKUKey: "linkedin-voice-senior"},
			{Key: "publish-linkedin", DefaultExecutorSKUKey: "linkedin-publish"},
		},
	}
}

func mustMarshalTemplateInputs(t *testing.T, inputs []*plansv1.TemplateInputParameter) json.RawMessage {
	t.Helper()
	parts := make([]string, 0, len(inputs))
	for _, input := range inputs {
		raw, err := protojson.Marshal(input)
		if err != nil {
			t.Fatalf("marshal template input: %v", err)
		}
		parts = append(parts, string(raw))
	}
	return json.RawMessage("[" + strings.Join(parts, ",") + "]")
}

// autoBindTestTemplate has one integration step and two agent-backed steps.
// executor_kind uses the plans-proto numeric form (AGENT=1, INTEGRATION=2) so
// both planStepExecutorKind and stepToProto agree on the kind.
func autoBindTestTemplate() *PlanTemplate {
	return &PlanTemplate{
		ID:  uuid.New(),
		Key: "auto-bind-overseer-test",
		Steps: []PlanStep{
			{
				Key:                 "fetch-news",
				ExecutorRequirement: json.RawMessage(`{"executor_kind":2}`), // INTEGRATION
			},
			{
				Key:                 "write-draft",
				ExecutorRequirement: json.RawMessage(`{"executor_kind":1}`), // AGENT
			},
			{
				Key:                 "adapt-for-linkedin",
				ExecutorRequirement: json.RawMessage(`{"executor_kind":1}`), // AGENT
			},
		},
	}
}

func findOverseer(bindings []*plansv1.OverseerBinding, stepKey string) *plansv1.OverseerBinding {
	for _, ob := range bindings {
		if ob.GetStepKey() == stepKey {
			return ob
		}
	}
	return nil
}

// TestAutoBindOverseers_AssignsCurrentUserToAgentSteps proves the helper fills
// overseer bindings for agent-backed steps with the authenticated user id,
// leaves integration steps alone, and respects caller-supplied bindings.
func TestAutoBindOverseers_AssignsCurrentUserToAgentSteps(t *testing.T) {
	currentUser := uuid.New().String()
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   currentUser,
		TenantID: uuid.New(),
	})
	template := autoBindTestTemplate()

	// Caller pre-binds write-draft to someone else; that must be preserved.
	callerBindings := []*plansv1.OverseerBinding{
		{StepKey: "write-draft", OverseerUserId: "user-ana"},
	}

	got := autoBindOverseers(ctx, template, callerBindings)

	if got := findOverseer(got, "fetch-news"); got != nil {
		t.Fatalf("integration step fetch-news must not get an overseer binding, got %#v", got)
	}
	wd := findOverseer(got, "write-draft")
	if wd == nil {
		t.Fatal("missing overseer binding for write-draft")
	}
	if wd.OverseerUserId != "user-ana" {
		t.Fatalf("write-draft overseer = %q, caller-supplied value must be preserved", wd.OverseerUserId)
	}
	adapt := findOverseer(got, "adapt-for-linkedin")
	if adapt == nil {
		t.Fatal("missing overseer binding for unbound agent step adapt-for-linkedin")
	}
	if adapt.OverseerUserId != currentUser {
		t.Fatalf("adapt-for-linkedin overseer = %q, want current user %q", adapt.OverseerUserId, currentUser)
	}
	if len(got) != 2 {
		t.Fatalf("len(bindings) = %d, want 2 (caller write-draft + auto adapt-for-linkedin)", len(got))
	}
}

// TestAutoBindOverseers_NoUserReturnsCallerBindings guards the fallback: with
// no authenticated user in the context, auto-binding is skipped so the
// OVERSEER_STEP prompt remains the configuration fallback.
func TestAutoBindOverseers_NoUserReturnsCallerBindings(t *testing.T) {
	template := autoBindTestTemplate()
	callerBindings := []*plansv1.OverseerBinding{
		{StepKey: "write-draft", OverseerUserId: "user-ana"},
	}

	// Plain context with no request context attached.
	got := autoBindOverseers(context.Background(), template, callerBindings)
	if len(got) != 1 || got[0] != callerBindings[0] {
		t.Fatalf("bindings = %#v, want caller bindings unchanged when no user is present", got)
	}

	// Request context present but empty user id → same fallback.
	emptyCtx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   "   ",
		TenantID: uuid.New(),
	})
	got = autoBindOverseers(emptyCtx, template, callerBindings)
	if len(got) != 1 {
		t.Fatalf("bindings = %#v, want caller bindings unchanged for blank user id", got)
	}
}

// TestAutoBindOverseers_SkipsOverseerStepInDeriveState proves the end-to-end
// effect: a configuration built with auto-bound overseers causes the assistant
// state machine to skip OVERSEER_STEP, whereas the same configuration without
// auto-binding would land on OVERSEER_STEP.
func TestAutoBindOverseers_SkipsOverseerStepInDeriveState(t *testing.T) {
	currentUser := uuid.New().String()
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   currentUser,
		TenantID: uuid.New(),
	})
	template := autoBindTestTemplate()
	templateProto := templateToProto(template)
	tenantID := uuid.New()

	// Every step has a slot binding so DeriveState moves past BINDING_STEP.
	slotBindings := []*plansv1.SlotBinding{
		{StepKey: "fetch-news", ExecutorInstallationId: uuid.New().String()},
		{StepKey: "write-draft", ExecutorInstallationId: uuid.New().String()},
		{StepKey: "adapt-for-linkedin", ExecutorInstallationId: uuid.New().String()},
	}

	h := &PlanHandler{}

	// With auto-binding: every agent step is overseer-bound → DeriveState must
	// NOT return OVERSEER_STEP (it advances to POLICIES_STEP since policies are
	// unset).
	autoOverseers := autoBindOverseers(ctx, template, nil)
	autoCfg, err := h.buildConfigurationFromRequest(
		tenantID, template, "",
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED,
		nil, slotBindings, autoOverseers, nil, nil, "{}",
	)
	if err != nil {
		t.Fatalf("buildConfigurationFromRequest() error = %v", err)
	}
	autoState := planassistant.DeriveState(templateProto, configurationToProto(autoCfg), nil)
	if autoState.Kind == planassistant.StateOverseerStep {
		t.Fatalf("DeriveState = OVERSEER_STEP, want it skipped when overseers are auto-bound")
	}
	if autoState.Kind != planassistant.StatePoliciesStep {
		t.Fatalf("DeriveState = %v, want POLICIES_STEP (policies unset, overseers bound)", autoState.Kind)
	}

	// Contrast: with no caller bindings and no user in context, the overseer
	// step is unbound → DeriveState returns OVERSEER_STEP (the fallback path).
	plainOverseers := autoBindOverseers(context.Background(), template, nil)
	plainCfg, err := h.buildConfigurationFromRequest(
		tenantID, template, "",
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED,
		nil, slotBindings, plainOverseers, nil, nil, "{}",
	)
	if err != nil {
		t.Fatalf("buildConfigurationFromRequest() error = %v", err)
	}
	plainState := planassistant.DeriveState(templateProto, configurationToProto(plainCfg), nil)
	if plainState.Kind != planassistant.StateOverseerStep {
		t.Fatalf("DeriveState = %v, want OVERSEER_STEP when no user is in context", plainState.Kind)
	}
}
