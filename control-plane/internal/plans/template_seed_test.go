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
	"harpia.artifacts.v1.LinkedInPost":        {},
	"harpia.artifacts.v1.CarouselDraft":       {},
	"harpia.artifacts.v1.ImageAsset":          {},
	"harpia.artifacts.v1.PublishConfirmation": {},
}

var testExecutorSKUs = map[string]struct{}{
	"rss-news-feed":               {},
	"newsletter-writer-senior":    {},
	"linkedin-voice-senior":       {},
	"linkedin-carousel-senior":    {},
	"linkedin-content-specialist": {},
	"image-asset-generator":       {},
	"linkedin-publish":            {},
}

func TestLoadPlanTemplateCatalogLoadsValidYAML(t *testing.T) {
	catalog, err := loadPlanTemplateCatalog(map[string][]byte{
		"weekly.yaml": []byte(validTemplateYAML("news-to-social-post")),
	}, testArtifactTypes, testExecutorSKUs)
	if err != nil {
		t.Fatalf("load valid catalog: %v", err)
	}

	if len(catalog) != 1 {
		t.Fatalf("template count = %d, want 1", len(catalog))
	}
	got := catalog[0]
	if got.Key != "news-to-social-post" {
		t.Fatalf("key = %q, want news-to-social-post", got.Key)
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
		"one.yaml": []byte(validTemplateYAML("news-to-social-post")),
		"two.yaml": []byte(validTemplateYAML("news-to-social-post")),
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
				// Make the back-edge type-consistent (write-draft output ==
				// fetch-news input) so the cycle detector fires rather than the
				// edge type-check, which now runs during edge iteration.
				seed.Steps[1].OutputArtifactTypeID = seed.Steps[0].InputArtifactTypeID
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
			name: "edge artifact mismatch",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps[1].InputArtifactTypeID = "harpia.artifacts.v1.DateRange"
			},
			wantError: "artifact mismatch",
		},
		{
			name: "optional middle step changes artifact type",
			mutate: func(seed *planTemplateSeed) {
				seed.Steps = append(seed.Steps, planTemplateStepSeed{
					Key:                  "optional-transform",
					Title:                "Optional transform",
					InputArtifactTypeID:  "harpia.artifacts.v1.NewsList",
					OutputArtifactTypeID: "harpia.artifacts.v1.TextDraft",
					ExecutorRequirement: map[string]any{
						"executor_kind":         1,
						"optional_capabilities": []any{"optional-transform"},
					},
					DefaultExecutorSKUKey: "newsletter-writer-senior",
				})
				seed.Edges = []planTemplateEdgeSeed{
					{FromStepKey: "fetch-news", ToStepKey: "optional-transform"},
					{FromStepKey: "optional-transform", ToStepKey: "write-draft"},
				}
				seed.Steps[1].InputArtifactTypeID = "harpia.artifacts.v1.TextDraft"
			},
			wantError: "must preserve its artifact type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := validTemplateSeed("news-to-social-post")
			tt.mutate(&seed)
			err := validatePlanTemplateCatalog([]planTemplateSeed{seed}, testArtifactTypes, testExecutorSKUs)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

// branchingTemplateSeed builds a DAG whose root fans out to two siblings that
// both consume the root's output type. Such a shape is rejected by a naive
// list-adjacency check (the second sibling's input would be compared against
// the first sibling's output) but must be accepted by the edge-based check.
func branchingTemplateSeed(key string) planTemplateSeed {
	return planTemplateSeed{
		Key:         key,
		Name:        "Branching Studio",
		Description: "Root fans out to two siblings sharing one upstream output.",
		Vertical:    "creator-economy",
		Version:     1,
		Steps: []planTemplateStepSeed{
			{
				Key:                   "root",
				Title:                 "Root",
				Description:           "Produces a NewsList.",
				InputArtifactTypeID:   "harpia.artifacts.v1.DateRange",
				OutputArtifactTypeID:  "harpia.artifacts.v1.NewsList",
				ExecutorRequirement:   map[string]any{"executor_kind": 2, "connection_type": "rss_feed"},
				DefaultExecutorSKUKey: "rss-news-feed",
			},
			{
				Key:                   "branch-text",
				Title:                 "Branch Text",
				Description:           "Consumes the NewsList into a TextDraft.",
				InputArtifactTypeID:   "harpia.artifacts.v1.NewsList",
				OutputArtifactTypeID:  "harpia.artifacts.v1.TextDraft",
				ExecutorRequirement:   map[string]any{"executor_kind": 1},
				DefaultExecutorSKUKey: "newsletter-writer-senior",
			},
			{
				Key:                   "branch-post",
				Title:                 "Branch Post",
				Description:           "Consumes the same NewsList into a LinkedInPostDraft.",
				InputArtifactTypeID:   "harpia.artifacts.v1.NewsList",
				OutputArtifactTypeID:  "harpia.artifacts.v1.LinkedInPostDraft",
				ExecutorRequirement:   map[string]any{"executor_kind": 1},
				DefaultExecutorSKUKey: "linkedin-voice-senior",
			},
		},
		Edges: []planTemplateEdgeSeed{
			{FromStepKey: "root", ToStepKey: "branch-text"},
			{FromStepKey: "root", ToStepKey: "branch-post"},
		},
		InputParameters: []templateInputParameterSeed{
			{
				Key:              "date_range",
				Label:            "Date range",
				Description:      "Article publication window.",
				Type:             "TEMPLATE_INPUT_PARAMETER_TYPE_DATE_RANGE",
				Required:         true,
				DefaultValueJSON: `{"preset":"schedule_window"}`,
				RuntimeMappings: []templateInputRuntimeMappingSeed{
					{
						Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT",
						StepKey:   "root",
						InputName: "date_range",
					},
				},
			},
		},
	}
}

func TestValidatePlanTemplateBranchingDAG(t *testing.T) {
	t.Run("branching DAG validates", func(t *testing.T) {
		seed := branchingTemplateSeed("branching-studio")
		if err := validatePlanTemplateCatalog([]planTemplateSeed{seed}, testArtifactTypes, testExecutorSKUs); err != nil {
			t.Fatalf("branching DAG should validate, got: %v", err)
		}
	})

	t.Run("edge artifact mismatch fails", func(t *testing.T) {
		seed := branchingTemplateSeed("branching-studio")
		// root outputs NewsList, but force branch-post to expect LinkedInPostDraft,
		// so the edge root -> branch-post no longer type-matches.
		seed.Steps[2].InputArtifactTypeID = "harpia.artifacts.v1.LinkedInPostDraft"
		err := validatePlanTemplateCatalog([]planTemplateSeed{seed}, testArtifactTypes, testExecutorSKUs)
		if err == nil || !strings.Contains(err.Error(), "artifact mismatch") {
			t.Fatalf("err = %v, want containing %q", err, "artifact mismatch")
		}
	})

	t.Run("linear template still validates", func(t *testing.T) {
		seed := validTemplateSeed("news-to-social-post")
		if err := validatePlanTemplateCatalog([]planTemplateSeed{seed}, testArtifactTypes, testExecutorSKUs); err != nil {
			t.Fatalf("linear template should validate, got: %v", err)
		}
	})
}

// TestEmbeddedPlanTemplatesLoadLinkedInContentStudio exercises the real
// version-3 catalog entry. Its single path preserves one LinkedInPost identity
// through the optional enrichments before the approval-governed publish step.
func TestEmbeddedPlanTemplatesLoadLinkedInContentStudio(t *testing.T) {
	files, err := embeddedPlanTemplateFiles()
	if err != nil {
		t.Fatalf("load embedded plan template files: %v", err)
	}
	catalog, err := loadPlanTemplateCatalog(files, testArtifactTypes, testExecutorSKUs)
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}

	var linkedin *planTemplateSeed
	for i := range catalog {
		if catalog[i].Key == "linkedin-content-studio" {
			linkedin = &catalog[i]
			break
		}
	}
	if linkedin == nil {
		t.Fatalf("embedded catalog does not contain linkedin-content-studio (keys: %v)", templateKeys(catalog))
	}

	if len(linkedin.Steps) != 6 {
		t.Fatalf("step count = %d, want 6", len(linkedin.Steps))
	}
	if len(linkedin.Edges) != 5 {
		t.Fatalf("edge count = %d, want 5", len(linkedin.Edges))
	}
	if linkedin.Version != 3 {
		t.Fatalf("version = %d, want 3", linkedin.Version)
	}

	// The two content-authoring steps are filled by the multi-capable
	// specialist SKU (not the retired narrow voice/carousel SKUs).
	stepByKey := make(map[string]planTemplateStepSeed, len(linkedin.Steps))
	for _, step := range linkedin.Steps {
		stepByKey[step.Key] = step
	}
	author, ok := stepByKey["author-content"]
	if !ok {
		t.Fatal("missing author-content step")
	}
	if author.DefaultExecutorSKUKey != "linkedin-content-specialist" {
		t.Fatalf("author-content sku = %q, want linkedin-content-specialist", author.DefaultExecutorSKUKey)
	}
	if !requiredCapability(author, "linkedin-content-adaptation") {
		t.Fatalf("author-content executor_requirement = %#v, want required_capabilities [linkedin-content-adaptation]", author.ExecutorRequirement)
	}
	assertRequiredContentInteractionPolicy(t, author)
	if author.OutputArtifactTypeID != "harpia.artifacts.v1.LinkedInPost" {
		t.Fatalf("author-content output = %q, want LinkedInPost", author.OutputArtifactTypeID)
	}
	carousel, ok := stepByKey["draft-carousel"]
	if !ok {
		t.Fatal("missing draft-carousel step")
	}
	if carousel.DefaultExecutorSKUKey != "linkedin-content-specialist" {
		t.Fatalf("draft-carousel sku = %q, want linkedin-content-specialist", carousel.DefaultExecutorSKUKey)
	}
	if !optionalCapability(carousel, "carousel-authoring") {
		t.Fatalf("draft-carousel executor_requirement = %#v, want optional_capabilities [carousel-authoring]", carousel.ExecutorRequirement)
	}
	assertRequiredContentInteractionPolicy(t, carousel)
	if carousel.InputArtifactTypeID != "harpia.artifacts.v1.LinkedInPost" || carousel.OutputArtifactTypeID != "harpia.artifacts.v1.LinkedInPost" {
		t.Fatalf("draft-carousel contract = %q -> %q, want LinkedInPost -> LinkedInPost", carousel.InputArtifactTypeID, carousel.OutputArtifactTypeID)
	}
	image, ok := stepByKey["generate-image"]
	if !ok {
		t.Fatal("missing generate-image step")
	}
	if !optionalCapability(image, "image-generation") {
		t.Fatalf("generate-image executor_requirement = %#v, want optional_capabilities [image-generation]", image.ExecutorRequirement)
	}
	assertRequiredContentInteractionPolicy(t, image)
	if image.InputArtifactTypeID != "harpia.artifacts.v1.LinkedInPost" || image.OutputArtifactTypeID != "harpia.artifacts.v1.LinkedInPost" {
		t.Fatalf("generate-image contract = %q -> %q, want LinkedInPost -> LinkedInPost", image.InputArtifactTypeID, image.OutputArtifactTypeID)
	}

	publish, ok := stepByKey["publish"]
	if !ok {
		t.Fatal("missing publish step")
	}
	if publish.InputArtifactTypeID != "harpia.artifacts.v1.LinkedInPost" || publish.OutputArtifactTypeID != "harpia.artifacts.v1.PublishConfirmation" {
		t.Fatalf("publish contract = %q -> %q, want LinkedInPost -> PublishConfirmation", publish.InputArtifactTypeID, publish.OutputArtifactTypeID)
	}
	if publish.ExecutorRequirement["executor_kind"] != 2 {
		t.Fatalf("publish executor_requirement = %#v, want integration executor", publish.ExecutorRequirement)
	}

	wantPath := []planTemplateEdgeSeed{
		{FromStepKey: "fetch-news", ToStepKey: "write-draft"},
		{FromStepKey: "write-draft", ToStepKey: "author-content"},
		{FromStepKey: "author-content", ToStepKey: "draft-carousel"},
		{FromStepKey: "draft-carousel", ToStepKey: "generate-image"},
		{FromStepKey: "generate-image", ToStepKey: "publish"},
	}
	for i, want := range wantPath {
		if linkedin.Edges[i] != want {
			t.Fatalf("edge[%d] = %#v, want %#v", i, linkedin.Edges[i], want)
		}
	}

	// The two opt-in SELECTs map onto the INCLUDED_CAPABILITY target.
	includeCarousel := findInputParameter(linkedin, "include_carousel")
	if includeCarousel == nil {
		t.Fatal("missing include_carousel input parameter")
	}
	if !hasIncludedCapabilityMapping(includeCarousel, "carousel-authoring") {
		t.Fatalf("include_carousel mappings = %#v, want INCLUDED_CAPABILITY carousel-authoring", includeCarousel.RuntimeMappings)
	}
	includeImages := findInputParameter(linkedin, "include_images")
	if includeImages == nil {
		t.Fatal("missing include_images input parameter")
	}
	if !hasIncludedCapabilityMapping(includeImages, "image-generation") {
		t.Fatalf("include_images mappings = %#v, want INCLUDED_CAPABILITY image-generation", includeImages.RuntimeMappings)
	}
}

func assertRequiredContentInteractionPolicy(t *testing.T, step planTemplateStepSeed) {
	t.Helper()
	if step.HumanInteractionPolicy["elicitation_mode"] != "ELICITATION_MODE_WHEN_REQUIRED" {
		t.Fatalf("%s elicitation policy = %#v, want WHEN_REQUIRED", step.Key, step.HumanInteractionPolicy)
	}
	if step.HumanInteractionPolicy["review_mode"] != "REVIEW_MODE_REQUIRED" {
		t.Fatalf("%s review policy = %#v, want REQUIRED", step.Key, step.HumanInteractionPolicy)
	}
}

func requiredCapability(step planTemplateStepSeed, capability string) bool {
	caps, _ := step.ExecutorRequirement["required_capabilities"].([]any)
	for _, c := range caps {
		if s, ok := c.(string); ok && s == capability {
			return true
		}
	}
	return false
}

func optionalCapability(step planTemplateStepSeed, capability string) bool {
	caps, _ := step.ExecutorRequirement["optional_capabilities"].([]any)
	for _, c := range caps {
		if s, ok := c.(string); ok && s == capability {
			return true
		}
	}
	return false
}

func findInputParameter(template *planTemplateSeed, key string) *templateInputParameterSeed {
	for i := range template.InputParameters {
		if template.InputParameters[i].Key == key {
			return &template.InputParameters[i]
		}
	}
	return nil
}

func hasIncludedCapabilityMapping(param *templateInputParameterSeed, policyKey string) bool {
	for _, mapping := range param.RuntimeMappings {
		if mapping.Target == "TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY" &&
			mapping.PolicyKey == policyKey {
			return true
		}
	}
	return false
}

func templateKeys(catalog []planTemplateSeed) []string {
	keys := make([]string, 0, len(catalog))
	for _, t := range catalog {
		keys = append(keys, t.Key)
	}
	return keys
}

func TestValidateRuntimeMappingIncludedCapabilityWhitelist(t *testing.T) {
	stepKeys := map[string]int{"fetch-news": 0}

	// Any non-empty capability id is accepted.
	for _, capability := range []string{"carousel-authoring", "image-generation"} {
		mapping := &templateInputRuntimeMappingSeed{
			Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY",
			PolicyKey: capability,
		}
		if err := validateRuntimeMapping("include_carousel", mapping, stepKeys); err != nil {
			t.Fatalf("capability %q: unexpected error = %v", capability, err)
		}
	}

	// Empty capability id must be rejected (it is the opt-in key).
	empty := &templateInputRuntimeMappingSeed{
		Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY",
		PolicyKey: "",
	}
	if err := validateRuntimeMapping("include_carousel", empty, stepKeys); err == nil {
		t.Fatal("empty policyKey: expected error, got nil")
	}
}

func TestValidateRuntimeMappingBehaviorPolicyWhitelist(t *testing.T) {
	stepKeys := map[string]int{"fetch-news": 0}

	for _, policyKey := range []string{"publish_approval_mode", "elicitation_timeout_behavior", "content_output_format"} {
		mapping := &templateInputRuntimeMappingSeed{
			Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY",
			PolicyKey: policyKey,
		}
		if err := validateRuntimeMapping("date_range", mapping, stepKeys); err != nil {
			t.Fatalf("policyKey %q: unexpected error = %v", policyKey, err)
		}
	}

	mapping := &templateInputRuntimeMappingSeed{
		Target:    "TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY",
		PolicyKey: "unknown_policy",
	}
	if err := validateRuntimeMapping("date_range", mapping, stepKeys); err == nil {
		t.Fatal("unknown_policy: expected error, got nil")
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
    defaultValueJson: '{"preset":"schedule_window"}'
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
				DefaultValueJSON: `{"preset":"schedule_window"}`,
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
