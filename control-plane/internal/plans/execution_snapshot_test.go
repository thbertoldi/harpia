package plans

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
)

func TestBuildPlanExecutionSnapshotLeavesExplicitDateRangeSeed(t *testing.T) {
	tenantID := uuid.New()
	templateID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	config := &PlanConfiguration{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		PlanTemplateID:      templateID,
		PlanTemplateVersion: 1,
		Status:              ConfigurationStatusRunnable,
		SeedArtifacts: mustMarshalSeeds(t, []*plansv1.SeedArtifactBinding{
			{StepKey: "fetch-news", InputName: "date_range", LiteralJson: `{"startDate":"2026-01-01","endDate":"2026-01-31"}`},
		}),
		SlotBindings: mustMarshalBindings(t, []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: installationID.String()},
		}),
	}
	template := &PlanTemplate{
		ID:      templateID,
		Key:     "news-to-social-post",
		Version: 1,
		Steps: []PlanStep{{
			Key:                   "fetch-news",
			InputArtifactTypeID:   "harpia.artifacts.v1.DateRange",
			OutputArtifactTypeID:  "harpia.artifacts.v1.NewsList",
			DefaultExecutorSKUKey: "rss-news-feed",
		}},
	}
	lookup := &mockExecutorLookup{
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
		skus:         map[uuid.UUID]*executors.ExecutorSKU{skuID: {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration}},
		entitledSKUs: map[uuid.UUID]bool{skuID: true},
	}

	snapshot, err := buildPlanExecutionSnapshot(
		context.Background(),
		tenantID,
		config,
		template,
		lookup,
		time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildPlanExecutionSnapshot() error = %v", err)
	}

	seed := findSeed(snapshot.Configuration.GetSeedArtifacts(), "fetch-news", "date_range")
	if seed == nil {
		t.Fatal("missing date_range seed in snapshot")
	}
	assertJSONEqual(t, seed.GetLiteralJson(), `{"startDate":"2026-01-01","endDate":"2026-01-31"}`)
}

func TestBuildPlanExecutionSnapshotResolvesScheduleWindowPerRun(t *testing.T) {
	tenantID := uuid.New()
	templateID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	config := &PlanConfiguration{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		PlanTemplateID:      templateID,
		PlanTemplateVersion: 1,
		Status:              ConfigurationStatusScheduled,
		Schedule:            json.RawMessage(`{"cronExpression":"0 9 * * MON","timezone":"UTC"}`),
		SeedArtifacts: mustMarshalSeeds(t, []*plansv1.SeedArtifactBinding{
			{StepKey: "fetch-news", InputName: "date_range", LiteralJson: `{"preset":"schedule_window"}`},
		}),
		SlotBindings: mustMarshalBindings(t, []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: installationID.String()},
		}),
	}
	template := &PlanTemplate{
		ID:      templateID,
		Key:     "news-to-social-post",
		Version: 1,
		Steps: []PlanStep{{
			Key:                  "fetch-news",
			InputArtifactTypeID:  "harpia.artifacts.v1.DateRange",
			OutputArtifactTypeID: "harpia.artifacts.v1.NewsList",
		}},
	}
	lookup := &mockExecutorLookup{
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
		skus:         map[uuid.UUID]*executors.ExecutorSKU{skuID: {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration}},
		entitledSKUs: map[uuid.UUID]bool{skuID: true},
	}

	first, err := buildPlanExecutionSnapshot(context.Background(), tenantID, config, template, lookup, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first snapshot error = %v", err)
	}
	second, err := buildPlanExecutionSnapshot(context.Background(), tenantID, config, template, lookup, time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second snapshot error = %v", err)
	}

	firstSeed := findSeed(first.Configuration.GetSeedArtifacts(), "fetch-news", "date_range")
	secondSeed := findSeed(second.Configuration.GetSeedArtifacts(), "fetch-news", "date_range")
	assertJSONEqual(t, firstSeed.GetLiteralJson(), `{"startDate":"2026-06-24","endDate":"2026-06-30"}`)
	assertJSONEqual(t, secondSeed.GetLiteralJson(), `{"startDate":"2026-07-01","endDate":"2026-07-07"}`)
}

func mustMarshalSeeds(t *testing.T, seeds []*plansv1.SeedArtifactBinding) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(seeds)
	if err != nil {
		t.Fatalf("marshal seeds: %v", err)
	}
	return raw
}

// The runtime snapshot is the single place that populates
// PlanConfiguration.included_optional_capabilities from the persisted parameter
// values, so the engine can skip opt-out steps (ADR-018 D4).
func TestBuildPlanExecutionSnapshotPopulatesIncludedOptionalCapabilities(t *testing.T) {
	tenantID := uuid.New()
	templateID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	config := func(parameterValues string) *PlanConfiguration {
		return &PlanConfiguration{
			ID:                  uuid.New(),
			TenantID:            tenantID,
			PlanTemplateID:      templateID,
			PlanTemplateVersion: 1,
			Status:              ConfigurationStatusRunnable,
			ParameterValues:     json.RawMessage(parameterValues),
			SlotBindings: mustMarshalBindings(t, []*plansv1.SlotBinding{
				{StepKey: "fetch-news", ExecutorInstallationId: installationID.String()},
			}),
		}
	}
	template := &PlanTemplate{
		ID:      templateID,
		Key:     "linkedin-content-studio",
		Version: 1,
		Steps: []PlanStep{{
			Key:                   "fetch-news",
			InputArtifactTypeID:   "harpia.artifacts.v1.DateRange",
			OutputArtifactTypeID:  "harpia.artifacts.v1.NewsList",
			DefaultExecutorSKUKey: "rss-news-feed",
		}},
		// The opt-in SELECTs map to the INCLUDED_CAPABILITY target.
		InputParameters: json.RawMessage(`[{"key":"include_carousel","runtimeMappings":[{"target":"TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY","policyKey":"carousel-authoring"}]},{"key":"include_images","runtimeMappings":[{"target":"TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY","policyKey":"image-generation"}]}]`),
	}
	lookup := &mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
			},
		},
		skus:         map[uuid.UUID]*executors.ExecutorSKU{skuID: {ID: skuID, Key: "rss-news-feed", Kind: executors.KindIntegration}},
		entitledSKUs: map[uuid.UUID]bool{skuID: true},
	}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	t.Run("carousel included, image not", func(t *testing.T) {
		snapshot, err := buildPlanExecutionSnapshot(context.Background(), tenantID, config(`{"include_carousel":"yes","include_images":"no"}`), template, lookup, now)
		if err != nil {
			t.Fatalf("buildPlanExecutionSnapshot() error = %v", err)
		}
		included := snapshot.Configuration.GetIncludedOptionalCapabilities()
		if !containsString(included, "carousel-authoring") {
			t.Fatalf("included = %v, want carousel-authoring", included)
		}
		if containsString(included, "image-generation") {
			t.Fatalf("included = %v, want image-generation absent", included)
		}
	})

	t.Run("nothing included by default", func(t *testing.T) {
		snapshot, err := buildPlanExecutionSnapshot(context.Background(), tenantID, config(`{"include_carousel":"no","include_images":"no"}`), template, lookup, now)
		if err != nil {
			t.Fatalf("buildPlanExecutionSnapshot() error = %v", err)
		}
		if len(snapshot.Configuration.GetIncludedOptionalCapabilities()) != 0 {
			t.Fatalf("included = %v, want empty", snapshot.Configuration.GetIncludedOptionalCapabilities())
		}
	})
}

func TestBuildPlanExecutionSnapshotFreezesActiveStepKeys(t *testing.T) {
	tenantID := uuid.New()
	templateID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	template := &PlanTemplate{
		ID:      templateID,
		Key:     "linkedin-content-studio",
		Version: 3,
		Steps: []PlanStep{
			{Key: "author-content", InputArtifactTypeID: "harpia.artifacts.v1.TextDraft", OutputArtifactTypeID: "harpia.artifacts.v1.LinkedInPost"},
			{Key: "generate-image", InputArtifactTypeID: "harpia.artifacts.v1.LinkedInPost", OutputArtifactTypeID: "harpia.artifacts.v1.LinkedInPost", ExecutorRequirement: json.RawMessage(`{"optional_capabilities":["image-generation"]}`)},
			{Key: "publish", InputArtifactTypeID: "harpia.artifacts.v1.LinkedInPost", OutputArtifactTypeID: "harpia.artifacts.v1.PublishConfirmation"},
		},
		InputParameters: json.RawMessage(`[{
			"key":"include_images",
			"runtimeMappings":[{"target":"TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY","policyKey":"image-generation"}]
		}]`),
	}
	config := &PlanConfiguration{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		PlanTemplateID:      templateID,
		PlanTemplateVersion: 3,
		Status:              ConfigurationStatusRunnable,
		ParameterValues:     json.RawMessage(`{"include_images":"no"}`),
		SlotBindings: mustMarshalBindings(t, []*plansv1.SlotBinding{
			{StepKey: "author-content", ExecutorInstallationId: installationID.String()},
			{StepKey: "publish", ExecutorInstallationId: installationID.String()},
		}),
	}
	lookup := &mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{installationID: {
			ID: installationID, TenantID: tenantID, ExecutorSKUID: skuID, Kind: executors.KindAgent, Enabled: true, ConnectionStatus: &connected,
		}},
		skus:         map[uuid.UUID]*executors.ExecutorSKU{skuID: {ID: skuID, Key: "linkedin-content-specialist", Kind: executors.KindAgent}},
		entitledSKUs: map[uuid.UUID]bool{skuID: true},
	}

	snapshot, err := buildPlanExecutionSnapshot(context.Background(), tenantID, config, template, lookup, time.Now().UTC())
	if err != nil {
		t.Fatalf("buildPlanExecutionSnapshot() error = %v", err)
	}
	if got, want := strings.Join(snapshot.ActiveStepKeys, ","), "author-content,publish"; got != want {
		t.Fatalf("active step keys = %q, want %q", got, want)
	}

	// A later edit is a configuration concern for a future execution. The stored
	// snapshot remains the complete, immutable active graph for this run.
	config.ParameterValues = json.RawMessage(`{"include_images":"yes"}`)
	if got, want := strings.Join(snapshot.ActiveStepKeys, ","), "author-content,publish"; got != want {
		t.Fatalf("frozen active step keys = %q, want %q", got, want)
	}
}
