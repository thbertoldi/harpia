package plans

import (
	"context"
	"encoding/json"
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
