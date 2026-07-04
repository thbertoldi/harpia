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
	threadID := uuid.New()
	originThreadID := uuid.New()

	got := configurationThreadID(&PlanConfiguration{
		ThreadID:       threadID,
		OriginThreadID: originThreadID,
	})
	if got != originThreadID {
		t.Fatalf("configurationThreadID() = %s, want origin thread %s", got, originThreadID)
	}
}

func TestConfigurationThreadIDFallsBackToThread(t *testing.T) {
	threadID := uuid.New()

	got := configurationThreadID(&PlanConfiguration{ThreadID: threadID})
	if got != threadID {
		t.Fatalf("configurationThreadID() = %s, want thread %s", got, threadID)
	}
}

func TestConfigurationFromProtoPreservesADR017Fields(t *testing.T) {
	threadID := uuid.New()
	originThreadID := uuid.New()
	existing := &PlanConfiguration{
		ID:                  uuid.New(),
		TenantID:            uuid.New(),
		PlanTemplateID:      uuid.New(),
		PlanTemplateVersion: 1,
		Kind:                ConfigurationKindRecurring,
		ThreadID:            threadID,
		OriginThreadID:      originThreadID,
	}

	got, err := configurationFromProto(&plansv1.PlanConfiguration{
		Status:              plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		Kind:                plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_ONE_SHOT,
		ThreadId:            uuid.New().String(),
		OriginThreadId:      uuid.New().String(),
		ParameterValuesJson: `{}`,
	}, existing)
	if err != nil {
		t.Fatalf("configurationFromProto() error = %v", err)
	}
	if got.Kind != existing.Kind {
		t.Fatalf("Kind = %q, want %q", got.Kind, existing.Kind)
	}
	if got.ThreadID != existing.ThreadID {
		t.Fatalf("ThreadID = %s, want %s", got.ThreadID, existing.ThreadID)
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
		"date_range":{"preset":"last_7_days"},
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
