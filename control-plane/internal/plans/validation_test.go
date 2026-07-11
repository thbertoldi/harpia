package plans

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
)

type mockExecutorLookup struct {
	installations map[uuid.UUID]*executors.ExecutorInstallation
	skus          map[uuid.UUID]*executors.ExecutorSKU
	entitledSKUs  map[uuid.UUID]bool
}

func (m *mockExecutorLookup) GetInstallationByID(_ context.Context, tenantID, id uuid.UUID) (*executors.ExecutorInstallation, error) {
	installation, ok := m.installations[id]
	if !ok || installation.TenantID != tenantID {
		return nil, pgx.ErrNoRows
	}
	return installation, nil
}

func (m *mockExecutorLookup) GetSKUByID(_ context.Context, id uuid.UUID) (*executors.ExecutorSKU, error) {
	sku, ok := m.skus[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return sku, nil
}

func (m *mockExecutorLookup) ListEntitlements(_ context.Context, tenantID uuid.UUID, skuID *uuid.UUID, limit, offset int) ([]executors.ExecutorEntitlement, error) {
	if skuID == nil || !m.entitledSKUs[*skuID] {
		return nil, nil
	}
	return []executors.ExecutorEntitlement{{
		ID:            uuid.New(),
		TenantID:      tenantID,
		ExecutorSKUID: *skuID,
	}}, nil
}

func (m *mockExecutorLookup) ListCompatibleInstallationsForStep(_ context.Context, tenantID uuid.UUID, step *plansv1.PlanStep) ([]executors.ExecutorInstallation, error) {
	if m == nil || step == nil {
		return nil, nil
	}
	defaultSKUKey := strings.TrimSpace(step.GetDefaultExecutorSkuKey())
	var out []executors.ExecutorInstallation
	for _, installation := range m.installations {
		if installation == nil || installation.TenantID != tenantID || !installation.Enabled {
			continue
		}
		sku := m.skus[installation.ExecutorSKUID]
		if sku == nil {
			continue
		}
		if defaultSKUKey != "" && sku.Key != defaultSKUKey {
			continue
		}
		switch step.GetExecutorRequirement().GetExecutorKind() {
		case plansv1.ExecutorKind_EXECUTOR_KIND_AGENT:
			if installation.Kind != executors.KindAgent {
				continue
			}
		case plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION:
			if installation.Kind != executors.KindIntegration {
				continue
			}
		}
		out = append(out, *installation)
	}
	return out, nil
}

func testTemplate() *PlanTemplate {
	return &PlanTemplate{
		Steps: []PlanStep{
			{Key: "fetch-news"},
			{Key: "adapt-for-linkedin"},
		},
	}
}

func overseerTestTemplate() *PlanTemplate {
	return &PlanTemplate{
		Steps: []PlanStep{
			{Key: "fetch-news", ExecutorRequirement: json.RawMessage(`{"executor_kind":2}`)},
			{Key: "adapt-for-linkedin", ExecutorRequirement: json.RawMessage(`{"executor_kind":1}`)},
		},
	}
}

func TestValidateSlotBindingsDraftAllowsMissingBindings(t *testing.T) {
	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateSlotBindings(context.Background(), uuid.New(), testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, nil, nil)
	if err != nil {
		t.Fatalf("expected draft with no bindings to succeed: %v", err)
	}
}

func TestValidateSlotBindingsDraftRejectsInvalidStepKey(t *testing.T) {
	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateSlotBindings(context.Background(), uuid.New(), testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, []*plansv1.SlotBinding{
		{StepKey: "missing-step"},
	}, nil)
	assertBindingError(t, err, connect.CodeInvalidArgument, "step_key does not exist")
}

func TestValidateSlotBindingsDraftRejectsUnentitledInstallation(t *testing.T) {
	tenantID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"

	validator := NewBindingValidator(&mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
				ConfigJSON:       json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
			},
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			skuID: {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration},
		},
		entitledSKUs: map[uuid.UUID]bool{},
	})

	err := validator.ValidateSlotBindings(context.Background(), tenantID, testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, []*plansv1.SlotBinding{
		{
			StepKey:                "fetch-news",
			ExecutorSkuId:          skuID.String(),
			ExecutorInstallationId: installationID.String(),
		},
	}, nil)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "not entitled")
}

func TestValidateSlotBindingsRunnableRequiresAllSteps(t *testing.T) {
	tenantID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"

	validator := NewBindingValidator(&mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
				ConfigJSON:       json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
			},
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			skuID: {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration},
		},
		entitledSKUs: map[uuid.UUID]bool{skuID: true},
	})

	err := validator.ValidateSlotBindings(context.Background(), tenantID, testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, []*plansv1.SlotBinding{
		{
			StepKey:                "fetch-news",
			ExecutorSkuId:          skuID.String(),
			ExecutorInstallationId: installationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
		},
	}, nil)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "slot binding is required")
}

func TestValidateSlotBindingsRunnableRejectsDisconnectedIntegration(t *testing.T) {
	tenantID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	agentInstallationID := uuid.New()
	agentSKUID := uuid.New()
	disconnected := "disconnected"
	manifestID := "linkedin-voice-senior"
	manifestVersion := "1.0.0"

	validator := NewBindingValidator(&mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &disconnected,
				ConfigJSON:       json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
			},
			agentInstallationID: {
				ID:              agentInstallationID,
				TenantID:        tenantID,
				ExecutorSKUID:   agentSKUID,
				Kind:            executors.KindAgent,
				Enabled:         true,
				ManifestID:      &manifestID,
				ManifestVersion: &manifestVersion,
			},
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			skuID:      {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration},
			agentSKUID: {ID: agentSKUID, Key: "linkedin-voice-senior", Kind: executors.KindAgent},
		},
		entitledSKUs: map[uuid.UUID]bool{skuID: true, agentSKUID: true},
	})

	bindings := []*plansv1.SlotBinding{
		{
			StepKey:                "fetch-news",
			ExecutorSkuId:          skuID.String(),
			ExecutorInstallationId: installationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
		},
		{
			StepKey:                "adapt-for-linkedin",
			ExecutorSkuId:          agentSKUID.String(),
			ExecutorInstallationId: agentInstallationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_AGENT,
		},
	}
	err := validator.ValidateSlotBindings(context.Background(), tenantID, testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, bindings, nil)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "not connected")
}

func TestValidateSlotBindingsRunnableAcceptsReadyBindings(t *testing.T) {
	tenantID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	manifestID := "linkedin-voice-senior"
	manifestVersion := "1.0.0"
	agentInstallationID := uuid.New()
	agentSKUID := uuid.New()

	validator := NewBindingValidator(&mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
				ConfigJSON:       json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
			},
			agentInstallationID: {
				ID:              agentInstallationID,
				TenantID:        tenantID,
				ExecutorSKUID:   agentSKUID,
				Kind:            executors.KindAgent,
				Enabled:         true,
				ManifestID:      &manifestID,
				ManifestVersion: &manifestVersion,
			},
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			skuID:      {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration},
			agentSKUID: {ID: agentSKUID, Key: "linkedin-voice-senior", Kind: executors.KindAgent},
		},
		entitledSKUs: map[uuid.UUID]bool{skuID: true, agentSKUID: true},
	})

	bindings := []*plansv1.SlotBinding{
		{
			StepKey:                "fetch-news",
			ExecutorSkuId:          skuID.String(),
			ExecutorInstallationId: installationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
		},
		{
			StepKey:                "adapt-for-linkedin",
			ExecutorSkuId:          agentSKUID.String(),
			ExecutorInstallationId: agentInstallationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_AGENT,
		},
	}

	err := validator.ValidateSlotBindings(context.Background(), tenantID, testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, bindings, nil)
	if err != nil {
		t.Fatalf("expected runnable bindings to pass: %v", err)
	}
}

// optionalStepTestTemplate is a two-step template whose second step declares an
// opt-in capability (ADR-018 D4). When the run does not include
// "image-generation", generate-image is skipped and needs no binding.
func optionalStepTestTemplate() *PlanTemplate {
	return &PlanTemplate{
		Steps: []PlanStep{
			{Key: "fetch-news"},
			{
				Key:                 "generate-image",
				ExecutorRequirement: json.RawMessage(`{"optional_capabilities":["image-generation"]}`),
			},
		},
	}
}

// optionalStepValidator builds a validator with a ready integration binding for
// fetch-news, leaving generate-image unbound so tests can assert whether it is
// required depending on the opt-in set.
func optionalStepValidator(t *testing.T, tenantID uuid.UUID) (*BindingValidator, *plansv1.SlotBinding) {
	t.Helper()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	validator := NewBindingValidator(&mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
				ConfigJSON:       json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
			},
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			skuID: {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration},
		},
		entitledSKUs: map[uuid.UUID]bool{skuID: true},
	})
	fetchBinding := &plansv1.SlotBinding{
		StepKey:                "fetch-news",
		ExecutorSkuId:          skuID.String(),
		ExecutorInstallationId: installationID.String(),
		ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
	}
	return validator, fetchBinding
}

func TestValidateSlotBindingsRunnableSkipsOptedOutStep(t *testing.T) {
	tenantID := uuid.New()
	validator, fetchBinding := optionalStepValidator(t, tenantID)
	err := validator.ValidateSlotBindings(
		context.Background(), tenantID, optionalStepTestTemplate(),
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		[]*plansv1.SlotBinding{fetchBinding},
		nil, // image-generation opted out: no binding, no error
	)
	if err != nil {
		t.Fatalf("expected runnable validation to skip opted-out optional step: %v", err)
	}
}

func TestValidateSlotBindingsRunnableRequiresOptedInStep(t *testing.T) {
	tenantID := uuid.New()
	validator, fetchBinding := optionalStepValidator(t, tenantID)
	err := validator.ValidateSlotBindings(
		context.Background(), tenantID, optionalStepTestTemplate(),
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		[]*plansv1.SlotBinding{fetchBinding},
		[]string{"image-generation"}, // opted in: binding still required
	)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "slot binding is required")
}

func TestValidateConfigurationForExecutionUsesRunnableRules(t *testing.T) {
	tenantID := uuid.New()
	config := &PlanConfiguration{
		TenantID: tenantID,
		SlotBindings: mustMarshalBindings(t, []*plansv1.SlotBinding{
			{StepKey: "fetch-news"},
		}),
	}

	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateConfigurationForExecution(context.Background(), tenantID, testTemplate(), config)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "executor_installation_id is required")
}

func TestValidateConfigurationForExecutionRejectsMissingRootSeed(t *testing.T) {
	tenantID := uuid.New()
	validator, bindings := readySeedValidationValidator(t, tenantID)
	config := &PlanConfiguration{
		TenantID:     tenantID,
		SlotBindings: mustMarshalBindings(t, bindings),
		SeedArtifacts: mustMarshalSeeds(t, []*plansv1.SeedArtifactBinding{
			{StepKey: "write-draft", InputName: "harpia.internal.ContentPreferences", LiteralJson: `{"tone":"analytical"}`},
		}),
	}

	err := validator.ValidateConfigurationForExecution(context.Background(), tenantID, seedValidationTemplate(), config)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "fetch-news")
	if !strings.Contains(err.Error(), "harpia.artifacts.v1.DateRange") {
		t.Fatalf("error = %v, want artifact type", err)
	}
}

func TestValidateConfigurationForExecutionAcceptsSeededRootStep(t *testing.T) {
	tenantID := uuid.New()
	validator, bindings := readySeedValidationValidator(t, tenantID)
	config := &PlanConfiguration{
		TenantID:     tenantID,
		SlotBindings: mustMarshalBindings(t, bindings),
		SeedArtifacts: mustMarshalSeeds(t, []*plansv1.SeedArtifactBinding{
			{StepKey: "fetch-news", InputName: "date_range", LiteralJson: `{"startDate":"2026-06-24","endDate":"2026-06-30"}`},
			{StepKey: "write-draft", InputName: "harpia.internal.ContentPreferences", LiteralJson: `{"tone":"analytical"}`},
		}),
	}

	err := validator.ValidateConfigurationForExecution(context.Background(), tenantID, seedValidationTemplate(), config)
	if err != nil {
		t.Fatalf("ValidateConfigurationForExecution() error = %v", err)
	}
}

// optionalRootStepTemplate has a required root step (fetch-news, seeded and
// bound) plus an optional root step (generate-image) that declares an opt-in
// capability. generate-image has no upstream edges and a non-empty
// InputArtifactTypeID, so without the opt-out skip validateRootSeedReadiness
// would demand a seed artifact for it. The InputParameters SELECT wires the
// capability to the INCLUDED_CAPABILITY target so materialization can derive
// the opt-in set from the configuration's parameter values.
func optionalRootStepTemplate() *PlanTemplate {
	return &PlanTemplate{
		Steps: []PlanStep{
			{
				Key:                  "fetch-news",
				InputArtifactTypeID:  "harpia.artifacts.v1.DateRange",
				OutputArtifactTypeID: "harpia.artifacts.v1.NewsList",
			},
			{
				Key:                  "generate-image",
				InputArtifactTypeID:  "harpia.artifacts.v1.ImageSpec",
				OutputArtifactTypeID: "harpia.artifacts.v1.Image",
				ExecutorRequirement:  json.RawMessage(`{"optional_capabilities":["image-generation"]}`),
			},
		},
		// No edges: both steps are root steps.
		InputParameters: json.RawMessage(`[{"key":"include_images","runtimeMappings":[{"target":"TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY","policyKey":"image-generation"}]}]`),
	}
}

func TestValidateConfigurationForExecutionSkipsOptedOutRootStep(t *testing.T) {
	tenantID := uuid.New()
	validator, bindings := readySeedValidationValidator(t, tenantID)
	// Only the fetch-news binding is needed: generate-image is opted out.
	var fetchBinding *plansv1.SlotBinding
	for _, b := range bindings {
		if b.GetStepKey() == "fetch-news" {
			fetchBinding = b
		}
	}
	if fetchBinding == nil {
		t.Fatal("missing fetch-news binding from readySeedValidationValidator")
	}
	// include_images="no" → image-generation opted out (ADR-018 D4). The
	// generate-image root step must not require a seed artifact or a binding.
	config := &PlanConfiguration{
		TenantID:        tenantID,
		ParameterValues: json.RawMessage(`{"include_images":"no"}`),
		SlotBindings:    mustMarshalBindings(t, []*plansv1.SlotBinding{fetchBinding}),
		SeedArtifacts: mustMarshalSeeds(t, []*plansv1.SeedArtifactBinding{
			{StepKey: "fetch-news", InputName: "date_range", LiteralJson: `{"startDate":"2026-07-01","endDate":"2026-07-11"}`},
		}),
	}

	err := validator.ValidateConfigurationForExecution(context.Background(), tenantID, optionalRootStepTemplate(), config)
	if err != nil {
		t.Fatalf("expected validation to skip opted-out optional root step, got: %v", err)
	}
}

func TestValidateOverseerBindingsDraftAllowsMissingOverseers(t *testing.T) {
	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateOverseerBindings(
		overseerTestTemplate(),
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("expected draft with no overseers to succeed: %v", err)
	}
}

func TestValidateOverseerBindingsRunnableRejectsMissingAgentOverseer(t *testing.T) {
	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateOverseerBindings(
		overseerTestTemplate(),
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		nil,
		nil,
	)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "overseer binding is required")
}

func TestValidateOverseerBindingsScheduledRejectsMissingAgentOverseer(t *testing.T) {
	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateOverseerBindings(
		overseerTestTemplate(),
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED,
		[]*plansv1.OverseerBinding{{StepKey: "fetch-news", OverseerUserId: "user-ana"}},
		nil,
	)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "adapt-for-linkedin")
}

func TestValidateOverseerBindingsRunnableAcceptsAgentOverseerAndSkipsIntegration(t *testing.T) {
	validator := NewBindingValidator(&mockExecutorLookup{})
	err := validator.ValidateOverseerBindings(
		overseerTestTemplate(),
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		[]*plansv1.OverseerBinding{{StepKey: "adapt-for-linkedin", OverseerUserId: "user-ana"}},
		nil,
	)
	if err != nil {
		t.Fatalf("expected runnable overseer bindings to pass: %v", err)
	}
}

func mustMarshalBindings(t *testing.T, bindings []*plansv1.SlotBinding) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(bindings)
	if err != nil {
		t.Fatalf("marshal bindings: %v", err)
	}
	return raw
}

func seedValidationTemplate() *PlanTemplate {
	return &PlanTemplate{
		Steps: []PlanStep{
			{
				Key:                  "fetch-news",
				InputArtifactTypeID:  "harpia.artifacts.v1.DateRange",
				OutputArtifactTypeID: "harpia.artifacts.v1.NewsList",
			},
			{
				Key:                  "write-draft",
				InputArtifactTypeID:  "harpia.artifacts.v1.NewsList",
				OutputArtifactTypeID: "harpia.artifacts.v1.TextDraft",
			},
		},
		Edges: []PlanStepDependency{{FromStepKey: "fetch-news", ToStepKey: "write-draft"}},
	}
}

func readySeedValidationValidator(t *testing.T, tenantID uuid.UUID) (*BindingValidator, []*plansv1.SlotBinding) {
	t.Helper()
	fetchSKUID := uuid.New()
	fetchInstallationID := uuid.New()
	writerSKUID := uuid.New()
	writerInstallationID := uuid.New()
	connected := "connected"
	manifestID := "newsletter-writer-senior"
	manifestVersion := "1.0.0"
	lookup := &mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			fetchInstallationID: {
				ID:               fetchInstallationID,
				TenantID:         tenantID,
				ExecutorSKUID:    fetchSKUID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
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
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			fetchSKUID:  {ID: fetchSKUID, Key: "rss-news-feed", Kind: executors.KindIntegration},
			writerSKUID: {ID: writerSKUID, Key: "newsletter-writer-senior", Kind: executors.KindAgent},
		},
		entitledSKUs: map[uuid.UUID]bool{fetchSKUID: true, writerSKUID: true},
	}
	return NewBindingValidator(lookup), []*plansv1.SlotBinding{
		{
			StepKey:                "fetch-news",
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
			ExecutorSkuId:          fetchSKUID.String(),
			ExecutorInstallationId: fetchInstallationID.String(),
		},
		{
			StepKey:                "write-draft",
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_AGENT,
			ExecutorSkuId:          writerSKUID.String(),
			ExecutorInstallationId: writerInstallationID.String(),
		},
	}
}

func assertBindingError(t *testing.T, err error, code connect.Code, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected binding validation error containing %q", contains)
	}

	var bindingErr *BindingValidationError
	if !errors.As(err, &bindingErr) {
		t.Fatalf("expected BindingValidationError, got %T: %v", err, err)
	}
	if bindingErr.Code != code {
		t.Fatalf("expected code %v, got %v (%v)", code, bindingErr.Code, bindingErr)
	}
	if contains != "" && !errors.Is(err, ErrBindingValidation) {
		t.Fatalf("expected wrapped ErrBindingValidation")
	}
	if contains != "" && !strings.Contains(bindingErr.Error(), contains) {
		t.Fatalf("expected error to contain %q, got %q", contains, bindingErr.Error())
	}
}
