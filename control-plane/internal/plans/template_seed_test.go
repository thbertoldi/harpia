package plans

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/database"
)

var testArtifactTypes = map[string]struct{}{
	"harpia.artifacts.v1.DateRange":           {},
	"harpia.artifacts.v1.NewsList":            {},
	"harpia.artifacts.v1.TextDraft":           {},
	"harpia.artifacts.v1.LinkedInPostDraft":   {},
	"harpia.artifacts.v1.PublishConfirmation": {},
}

var testExecutorSKUs = map[string]struct{}{
	"rss-news-feed":            {},
	"newsletter-writer-senior": {},
	"linkedin-voice-senior":    {},
	"linkedin-publish":         {},
}

func TestLoadPlanTemplateCatalogLoadsValidYAML(t *testing.T) {
	catalog, err := loadPlanTemplateCatalog(map[string][]byte{
		"weekly.yaml": []byte(validTemplateYAML("weekly-newsletter-linkedin")),
	}, testArtifactTypes, testExecutorSKUs)
	if err != nil {
		t.Fatalf("load valid catalog: %v", err)
	}

	if len(catalog) != 1 {
		t.Fatalf("template count = %d, want 1", len(catalog))
	}
	got := catalog[0]
	if got.Key != "weekly-newsletter-linkedin" {
		t.Fatalf("key = %q, want weekly-newsletter-linkedin", got.Key)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("step count = %d, want 2", len(got.Steps))
	}
	if got.InputParameters[0].RuntimeMappings[0].StepKey != "fetch-news" {
		t.Fatalf("runtime mapping step = %q, want fetch-news", got.InputParameters[0].RuntimeMappings[0].StepKey)
	}
}

func TestLoadPlanTemplateCatalogRejectsDuplicateTemplateKeys(t *testing.T) {
	_, err := loadPlanTemplateCatalog(map[string][]byte{
		"one.yaml": []byte(validTemplateYAML("weekly-newsletter-linkedin")),
		"two.yaml": []byte(validTemplateYAML("weekly-newsletter-linkedin")),
	}, testArtifactTypes, testExecutorSKUs)
	if err == nil || !strings.Contains(err.Error(), "duplicate template key") {
		t.Fatalf("err = %v, want duplicate template key", err)
	}
}

func TestValidatePlanTemplateRejectsFailures(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*planTemplateSeed)
		wantError string
	}{
		{
			name: "cycle",
			mutate: func(seed *planTemplateSeed) {
				seed.Edges = append(seed.Edges, planTemplateEdgeSeed{FromStepKey: "write-draft", ToStepKey: "fetch-news"})
			},
			wantError: "cycle",
		},
		{
			name: "dangling edge",
			mutate: func(seed *planTemplateSeed) {
				seed.Edges = []planTemplateEdgeSeed{{FromStepKey: "missing", ToStepKey: "write-draft"}}
			},
			wantError: "unknown step",
		},
		{
			name: "unreachable step",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps = append(seed.Steps, planTemplateStepSeed{
					Key:                   "orphan",
					Title:                 "Orphan",
					InputArtifactTypeID:   "harpia.artifacts.v1.TextDraft",
					OutputArtifactTypeID:  "harpia.artifacts.v1.LinkedInPostDraft",
					DefaultExecutorSKUKey: "linkedin-voice-senior",
				})
			},
			wantError: "unreachable",
		},
		{
			name: "duplicate step key",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps = append(seed.Steps, seed.Steps[0])
			},
			wantError: "duplicate step key",
		},
		{
			name: "unknown artifact type",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps[0].InputArtifactTypeID = "harpia.artifacts.v1.DoesNotExist"
			},
			wantError: "unknown artifact type",
		},
		{
			name: "unknown executor sku",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps[0].DefaultExecutorSKUKey = "missing-sku"
			},
			wantError: "unknown executor sku",
		},
		{
			name: "bad runtime mapping step",
			mutate: func(seed *planTemplateSeed) {
				seed.InputParameters[0].RuntimeMappings[0].StepKey = "missing-step"
			},
			wantError: "runtime mapping",
		},
		{
			name: "bad runtime mapping policy",
			mutate: func(seed *planTemplateSeed) {
				seed.InputParameters[0].RuntimeMappings = []templateInputRuntimeMappingSeed{{
					Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY",
					PolicyKey: "unknown_policy",
				}}
			},
			wantError: "unknown behavior policy",
		},
		{
			name: "adjacent artifact mismatch",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps[1].InputArtifactTypeID = "harpia.artifacts.v1.DateRange"
			},
			wantError: "adjacent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := validTemplateSeed("weekly-newsletter-linkedin")
			tt.mutate(&seed)
			err := validatePlanTemplateCatalog([]planTemplateSeed{seed}, testArtifactTypes, testExecutorSKUs)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestReconcilePlanTemplateCatalogUpsertsAndReconcilesChildren(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run plan template seeder integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	seedCatalogReferences(t, ctx, tx)

	seed := validTemplateSeed("test-template-" + strings.ReplaceAll(uuid.NewString(), "-", ""))
	removed := validTemplateSeed("removed-template-" + strings.ReplaceAll(uuid.NewString(), "-", ""))
	if err := reconcilePlanTemplateCatalog(ctx, tx, []planTemplateSeed{seed, removed}); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	seed.Name = "Updated Template Name"
	seed.Steps[1].Title = "Updated Draft Title"
	seed.InputParameters[0].DefaultValueJSON = `{"preset":"last_30_days"}`
	if _, err := tx.Exec(ctx, `
		INSERT INTO plan_template_step_dependencies (plan_template_id, from_step_key, to_step_key)
		SELECT id, 'write-draft', 'stale-step'
		FROM plan_templates
		WHERE key = $1
	`, seed.Key); err != nil {
		t.Fatalf("insert stale dependency: %v", err)
	}

	if err := reconcilePlanTemplateCatalog(ctx, tx, []planTemplateSeed{seed}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	var name string
	var inputParameters []byte
	if err := tx.QueryRow(ctx, `SELECT name, input_parameters FROM plan_templates WHERE key = $1`, seed.Key).Scan(&name, &inputParameters); err != nil {
		t.Fatalf("load reconciled template: %v", err)
	}
	if name != "Updated Template Name" {
		t.Fatalf("template name = %q, want updated", name)
	}
	var params []templateInputParameterSeed
	if err := json.Unmarshal(inputParameters, &params); err != nil {
		t.Fatalf("unmarshal input parameters: %v", err)
	}
	if params[0].DefaultValueJSON != `{"preset":"last_30_days"}` {
		t.Fatalf("default value json = %q, want last_30_days", params[0].DefaultValueJSON)
	}

	var stepCount, edgeCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM plan_template_steps s
		JOIN plan_templates t ON t.id = s.plan_template_id
		WHERE t.key = $1
	`, seed.Key).Scan(&stepCount); err != nil {
		t.Fatalf("count steps: %v", err)
	}
	if stepCount != len(seed.Steps) {
		t.Fatalf("step count = %d, want %d", stepCount, len(seed.Steps))
	}
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM plan_template_step_dependencies d
		JOIN plan_templates t ON t.id = d.plan_template_id
		WHERE t.key = $1
	`, seed.Key).Scan(&edgeCount); err != nil {
		t.Fatalf("count edges: %v", err)
	}
	if edgeCount != len(seed.Edges) {
		t.Fatalf("edge count = %d, want %d", edgeCount, len(seed.Edges))
	}

	var removedCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM plan_templates WHERE key = $1`, removed.Key).Scan(&removedCount); err != nil {
		t.Fatalf("count removed template: %v", err)
	}
	if removedCount != 0 {
		t.Fatalf("removed template count = %d, want 0", removedCount)
	}
}

func seedCatalogReferences(t *testing.T, ctx context.Context, q database.Querier) {
	t.Helper()
	for key := range testArtifactTypes {
		if _, err := q.Exec(ctx, `
			INSERT INTO artifact_types (key, schema_ref, version, description)
			VALUES ($1, $1, 1, 'test artifact type')
			ON CONFLICT (key) DO NOTHING
		`, key); err != nil {
			t.Fatalf("seed artifact type %q: %v", key, err)
		}
	}
	for key := range testExecutorSKUs {
		if _, err := q.Exec(ctx, `
			INSERT INTO executor_skus (key, display_name, description, kind, price_cents, currency, compatibility)
			VALUES ($1, $1, 'test sku', 'agent', 0, 'USD', '{}'::jsonb)
			ON CONFLICT (key) DO NOTHING
		`, key); err != nil {
			t.Fatalf("seed executor sku %q: %v", key, err)
		}
	}
}

func validTemplateYAML(key string) string {
	return `
key: ` + key + `
name: Weekly Newsletter
description: Fetch news and write a draft.
vertical: creator-economy
version: 1
steps:
  - key: fetch-news
    title: Fetch News
    description: Collect articles.
    input_artifact_type: harpia.artifacts.v1.DateRange
    output_artifact_type: harpia.artifacts.v1.NewsList
    executor_requirement:
      executor_kind: 2
      connection_type: rss_feed
    default_executor_sku_key: rss-news-feed
  - key: write-draft
    title: Write Draft
    description: Synthesize a draft.
    input_artifact_type: harpia.artifacts.v1.NewsList
    output_artifact_type: harpia.artifacts.v1.TextDraft
    executor_requirement:
      executor_kind: 1
    default_executor_sku_key: newsletter-writer-senior
edges:
  - from_step_key: fetch-news
    to_step_key: write-draft
input_parameters:
  - key: date_range
    label: Date range
    description: Article publication window.
    type: TEMPLATE_INPUT_PARAMETER_TYPE_DATE_RANGE
    required: true
    defaultValueJson: '{"preset":"last_7_days"}'
    runtimeMappings:
      - target: TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT
        stepKey: fetch-news
        inputName: date_range
`
}

func validTemplateSeed(key string) planTemplateSeed {
	return planTemplateSeed{
		Key:         key,
		Name:        "Weekly Newsletter",
		Description: "Fetch news and write a draft.",
		Vertical:    "creator-economy",
		Version:     1,
		Steps: []planTemplateStepSeed{
			{
				Key:                   "fetch-news",
				Title:                 "Fetch News",
				Description:           "Collect articles.",
				InputArtifactTypeID:   "harpia.artifacts.v1.DateRange",
				OutputArtifactTypeID:  "harpia.artifacts.v1.NewsList",
				ExecutorRequirement:   map[string]any{"executor_kind": 2, "connection_type": "rss_feed"},
				DefaultExecutorSKUKey: "rss-news-feed",
			},
			{
				Key:                   "write-draft",
				Title:                 "Write Draft",
				Description:           "Synthesize a draft.",
				InputArtifactTypeID:   "harpia.artifacts.v1.NewsList",
				OutputArtifactTypeID:  "harpia.artifacts.v1.TextDraft",
				ExecutorRequirement:   map[string]any{"executor_kind": 1},
				DefaultExecutorSKUKey: "newsletter-writer-senior",
			},
		},
		Edges: []planTemplateEdgeSeed{
			{FromStepKey: "fetch-news", ToStepKey: "write-draft"},
		},
		InputParameters: []templateInputParameterSeed{
			{
				Key:              "date_range",
				Label:            "Date range",
				Description:      "Article publication window.",
				Type:             "TEMPLATE_INPUT_PARAMETER_TYPE_DATE_RANGE",
				Required:         true,
				DefaultValueJSON: `{"preset":"last_7_days"}`,
				RuntimeMappings: []templateInputRuntimeMappingSeed{
					{
						Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT",
						StepKey:   "fetch-news",
						InputName: "date_range",
					},
				},
			},
		},
	}
}
