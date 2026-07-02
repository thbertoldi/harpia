package plans

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
)

func TestMaterializePlanConfigurationFromParameterValues(t *testing.T) {
	template := weeklyNewsletterMaterializeTemplate()
	sourceGroupID := uuid.New().String()

	seeds, slots, policies, err := MaterializePlanConfiguration(template, map[string]any{
		"theme":           "AI operations",
		"language":        "pt-BR",
		"tone":            "analytical",
		"audience":        "operations leaders",
		"topics_to_avoid": "crypto hype",
		"date_range":      map[string]any{"preset": "last_7_days"},
		"source_group":    sourceGroupID,
		"approval_mode":   "require_approval",
		"unknown":         "ignored",
	})
	if err != nil {
		t.Fatalf("MaterializePlanConfiguration() error = %v", err)
	}

	if len(seeds) != 2 {
		t.Fatalf("seed count = %d, want 2: %#v", len(seeds), seeds)
	}
	contentSeed := findSeed(seeds, "write-draft", "harpia.internal.ContentPreferences")
	if contentSeed == nil {
		t.Fatal("missing write-draft ContentPreferences seed")
	}
	assertJSONEqual(t, contentSeed.LiteralJson, `{
		"topic":"AI operations",
		"language":"pt-BR",
		"tone":"analytical",
		"audience":"operations leaders",
		"topics_to_avoid":"crypto hype"
	}`)

	dateSeed := findSeed(seeds, "fetch-news", "date_range")
	if dateSeed == nil {
		t.Fatal("missing fetch-news date_range seed")
	}
	assertJSONEqual(t, dateSeed.LiteralJson, `{"preset":"last_7_days"}`)

	if len(slots) != 1 {
		t.Fatalf("slot binding count = %d, want 1: %#v", len(slots), slots)
	}
	if slots[0].StepKey != "fetch-news" || slots[0].ExecutorInstallationId != sourceGroupID {
		t.Fatalf("slot binding = %#v, want fetch-news installation %q", slots[0], sourceGroupID)
	}

	if got := policies.GetPublishApprovalMode(); got != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL {
		t.Fatalf("publish approval mode = %v, want REQUIRE_APPROVAL", got)
	}
}

func TestMaterializePlanConfigurationSkipsAbsentParameters(t *testing.T) {
	seeds, slots, policies, err := MaterializePlanConfiguration(weeklyNewsletterMaterializeTemplate(), map[string]any{
		"theme": "AI operations",
	})
	if err != nil {
		t.Fatalf("MaterializePlanConfiguration() error = %v", err)
	}

	contentSeed := findSeed(seeds, "write-draft", "harpia.internal.ContentPreferences")
	if contentSeed == nil {
		t.Fatal("missing write-draft ContentPreferences seed")
	}
	assertJSONEqual(t, contentSeed.LiteralJson, `{"topic":"AI operations"}`)

	if len(slots) != 0 {
		t.Fatalf("slot binding count = %d, want 0", len(slots))
	}
	if policies.GetPublishApprovalMode() != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED {
		t.Fatalf("publish approval mode = %v, want UNSPECIFIED", policies.GetPublishApprovalMode())
	}
}

func TestResolveDefaultSlotBindingsSkipsParameterBindingsAndBindsDefaults(t *testing.T) {
	tenantID := uuid.New()
	sourceGroupID := uuid.New()
	writerSKUID := uuid.New()
	writerInstallationID := uuid.New()
	publisherSKUID := uuid.New()
	publisherInstallationID := uuid.New()
	manifestID := "newsletter-writer-senior"
	manifestVersion := "1.0.0"

	lookup := &mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			writerInstallationID: {
				ID:              writerInstallationID,
				TenantID:        tenantID,
				ExecutorSKUID:   writerSKUID,
				Kind:            executors.KindAgent,
				Enabled:         true,
				ManifestID:      &manifestID,
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
			publisherSKUID: {ID: publisherSKUID, Key: "linkedin-publish", Kind: executors.KindIntegration},
		},
		entitledSKUs: map[uuid.UUID]bool{writerSKUID: true, publisherSKUID: true},
	}

	defaults, err := ResolveDefaultSlotBindings(
		context.Background(),
		tenantID,
		defaultSlotBindingTemplate(),
		[]*plansv1.SlotBinding{{StepKey: "fetch-news", ExecutorInstallationId: sourceGroupID.String()}},
		lookup,
	)
	if err != nil {
		t.Fatalf("ResolveDefaultSlotBindings() error = %v", err)
	}

	if len(defaults) != 2 {
		t.Fatalf("default binding count = %d, want 2: %#v", len(defaults), defaults)
	}
	writer := findSlot(defaults, "write-draft")
	if writer == nil {
		t.Fatal("missing write-draft default binding")
	}
	if writer.ExecutorInstallationId != writerInstallationID.String() ||
		writer.ExecutorSkuId != writerSKUID.String() ||
		writer.ExecutorKind != plansv1.ExecutorKind_EXECUTOR_KIND_AGENT {
		t.Fatalf("writer binding = %#v", writer)
	}
	publisher := findSlot(defaults, "publish-linkedin")
	if publisher == nil {
		t.Fatal("missing publish-linkedin default binding")
	}
	if publisher.ExecutorInstallationId != publisherInstallationID.String() ||
		publisher.ExecutorSkuId != publisherSKUID.String() ||
		publisher.ExecutorKind != plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION {
		t.Fatalf("publisher binding = %#v", publisher)
	}
}

func TestResolveDefaultSlotBindingsReturnsPreconditionForMissingRequiredDefault(t *testing.T) {
	tenantID := uuid.New()
	_, err := ResolveDefaultSlotBindings(
		context.Background(),
		tenantID,
		defaultSlotBindingTemplate(),
		nil,
		&mockExecutorLookup{},
	)
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("error code = %v, want FailedPrecondition (err=%v)", got, err)
	}
	if !strings.Contains(err.Error(), "fetch-news") {
		t.Fatalf("error = %v, want step name", err)
	}
}

func weeklyNewsletterMaterializeTemplate() *plansv1.PlanTemplate {
	return &plansv1.PlanTemplate{
		Key: "weekly-newsletter-linkedin",
		InputParameters: []*plansv1.TemplateInputParameter{
			seedParam("theme", "write-draft", "harpia.internal.ContentPreferences", "$.topic"),
			seedParam("language", "write-draft", "harpia.internal.ContentPreferences", "$.language"),
			seedParam("tone", "write-draft", "harpia.internal.ContentPreferences", "$.tone"),
			seedParam("audience", "write-draft", "harpia.internal.ContentPreferences", "$.audience"),
			seedParam("topics_to_avoid", "write-draft", "harpia.internal.ContentPreferences", "$.topics_to_avoid"),
			{
				Key: "source_group",
				RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
					Target:  plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_SLOT_BINDING,
					StepKey: "fetch-news",
				}},
			},
			seedParam("date_range", "fetch-news", "date_range", ""),
			{
				Key: "approval_mode",
				RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
					Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
					PolicyKey: "publish_approval_mode",
				}},
			},
		},
	}
}

func defaultSlotBindingTemplate() *plansv1.PlanTemplate {
	return &plansv1.PlanTemplate{
		Steps: []*plansv1.PlanStep{
			{
				Key:                   "fetch-news",
				DefaultExecutorSkuKey: "rss-news-feed",
				ExecutorRequirement:   &plansv1.ExecutorRequirement{ExecutorKind: plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION},
			},
			{
				Key:                   "write-draft",
				DefaultExecutorSkuKey: "newsletter-writer-senior",
				ExecutorRequirement:   &plansv1.ExecutorRequirement{ExecutorKind: plansv1.ExecutorKind_EXECUTOR_KIND_AGENT},
			},
			{
				Key:                   "publish-linkedin",
				DefaultExecutorSkuKey: "linkedin-publish",
				ExecutorRequirement:   &plansv1.ExecutorRequirement{ExecutorKind: plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION},
			},
		},
	}
}

func seedParam(key, stepKey, inputName, path string) *plansv1.TemplateInputParameter {
	return &plansv1.TemplateInputParameter{
		Key: key,
		RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
			Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT,
			StepKey:   stepKey,
			InputName: inputName,
			JsonPath:  path,
		}},
	}
}

func findSeed(seeds []*plansv1.SeedArtifactBinding, stepKey, inputName string) *plansv1.SeedArtifactBinding {
	for _, seed := range seeds {
		if seed.GetStepKey() == stepKey && seed.GetInputName() == inputName {
			return seed
		}
	}
	return nil
}

func findSlot(slots []*plansv1.SlotBinding, stepKey string) *plansv1.SlotBinding {
	for _, slot := range slots {
		if slot.GetStepKey() == stepKey {
			return slot
		}
	}
	return nil
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("got invalid JSON %q: %v", got, err)
	}
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("want invalid JSON %q: %v", want, err)
	}
	gotJSON, _ := json.Marshal(gotValue)
	wantJSON, _ := json.Marshal(wantValue)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON = %s, want %s", gotJSON, wantJSON)
	}
}

func ptr[T any](v T) *T {
	return &v
}
