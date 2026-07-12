// Package acceptance exercises release-critical behavior through production
// adapters. It is intentionally separate from unit packages so `mise run
// acceptance` cannot silently pass without a real app-role database.
package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/audit"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/integrations/image"
	"github.com/harpia/control-plane/internal/plans"
)

// Acceptance needs a non-superuser NOBYPASSRLS role. Run migrations first as
// the database owner, then provision the app role, for example:
//
//	CREATE ROLE harpia_app LOGIN PASSWORD 'harpia-app-password' NOBYPASSRLS;
//	GRANT CONNECT ON DATABASE harpia TO harpia_app;
//	GRANT USAGE ON SCHEMA public TO harpia_app;
//	GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO harpia_app;
//	GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO harpia_app;
//	ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO harpia_app;
//
// HARPIA_TEST_DATABASE_URL must use that role. A privileged test connection
// bypasses forced RLS and would make the tenant-isolation assertion meaningless.
func requireAcceptanceDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("acceptance requires HARPIA_TEST_DATABASE_URL set to a NON-SUPERUSER NOBYPASSRLS app-role DSN (see acceptance_test.go setup SQL)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect acceptance database: %v", err)
	}
	t.Cleanup(pool.Close)

	var isSuperuser string
	if err := pool.QueryRow(ctx, "SHOW is_superuser").Scan(&isSuperuser); err != nil {
		t.Fatalf("check app role superuser status: %v", err)
	}
	if strings.EqualFold(strings.TrimSpace(isSuperuser), "on") {
		t.Fatal("HARPIA_TEST_DATABASE_URL must not use a superuser; forced RLS acceptance needs a non-superuser app role")
	}

	var bypassRLS bool
	if err := pool.QueryRow(ctx, "SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(&bypassRLS); err != nil {
		t.Fatalf("check app role bypassrls status: %v", err)
	}
	if bypassRLS {
		t.Fatal("HARPIA_TEST_DATABASE_URL role has BYPASSRLS; acceptance requires a NOBYPASSRLS app role")
	}
	return pool
}

func TestAcceptanceLinkedInOptionalCapabilities(t *testing.T) {
	pool := requireAcceptanceDB(t)
	ctx := context.Background()
	template, templateProto := seedAndLoadLinkedInTemplate(t, ctx, pool)
	tenantID := createTenant(t, ctx, pool, "acceptance-linkedin")
	executorRepo := executors.NewRepository(pool)

	bindings := createRequiredLinkedInBindings(t, ctx, executorRepo, tenantID)
	_, _, _, excluded, err := plans.MaterializePlanConfiguration(templateProto, map[string]any{
		"include_images":   "no",
		"include_carousel": "no",
	})
	if err != nil {
		t.Fatalf("materialize real linkedin-content-studio opt-outs: %v", err)
	}
	if contains(excluded, "image-generation") || contains(excluded, "carousel-authoring") {
		t.Fatalf("included optional capabilities = %v, want image-generation and carousel-authoring excluded", excluded)
	}
	assertNoBinding(t, bindings, "generate-image")
	assertNoBinding(t, bindings, "draft-carousel")

	validator := plans.NewBindingValidator(executorRepo)
	if err := validator.ValidateSlotBindings(ctx, tenantID, template, plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, bindings, excluded); err != nil {
		t.Fatalf("RUNNABLE validation with opted-out optional steps: %v", err)
	}

	_, _, _, imageIncluded, err := plans.MaterializePlanConfiguration(templateProto, map[string]any{
		"include_images":   "yes",
		"include_carousel": "no",
	})
	if err != nil {
		t.Fatalf("materialize real linkedin-content-studio image opt-in: %v", err)
	}
	if !contains(imageIncluded, "image-generation") {
		t.Fatalf("included optional capabilities = %v, want image-generation", imageIncluded)
	}
	if err := validator.ValidateSlotBindings(ctx, tenantID, template, plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, bindings, imageIncluded); err == nil {
		t.Fatal("RUNNABLE validation must block image opt-in without a compatible image installation")
	} else if !strings.Contains(err.Error(), "generate-image") {
		t.Fatalf("image opt-in validation error = %v, want generate-image blocker", err)
	}

	imageBinding := createInstallationBinding(t, ctx, executorRepo, tenantID, executors.SKUImageAssetGenerator, "Acceptance noop image", executors.KindIntegration, json.RawMessage(`{"provider":"noop"}`), nil)
	bindings = append(bindings, imageBinding)
	if err := validator.ValidateSlotBindings(ctx, tenantID, template, plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, bindings, imageIncluded); err != nil {
		t.Fatalf("RUNNABLE validation with compatible noop image installation: %v", err)
	}
}

func TestAcceptanceAuditRepositoryTenantSafetyDedupeAndRedaction(t *testing.T) {
	pool := requireAcceptanceDB(t)
	ctx := context.Background()
	tenantA := createTenant(t, ctx, pool, "acceptance-audit-a")
	tenantB := createTenant(t, ctx, pool, "acceptance-audit-b")
	repo := audit.NewRepository(pool)

	writer, queue := audit.NewWriter(audit.WriterOptions{
		Repo: repo,
		Config: audit.WriterConfig{
			QueueSize:     4,
			BatchSize:     1,
			FlushInterval: time.Hour,
		},
	})
	writer.Start()
	recorder := audit.NewRecorder(audit.RecorderOptions{Queue: queue})
	draft := audit.EventDraft{
		TenantID:       tenantA,
		DedupeKey:      "acceptance-dedupe-" + uuid.NewString(),
		EventType:      "plan_configuration.created",
		BoundedContext: "plan_management",
		Actor:          audit.Actor{Kind: "human", ID: "acceptance-user"},
		Diff:           []audit.DiffEntry{{Field: "api_key", After: "sk-acceptance-secret", HasAfter: true}},
		DiffAllowlist:  []string{"api_key"},
	}
	if err := recorder.Record(ctx, draft); err != nil {
		t.Fatalf("record first audit event: %v", err)
	}
	if err := recorder.Record(ctx, draft); err != nil {
		t.Fatalf("record duplicate audit event: %v", err)
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := writer.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("flush audit writer: %v", err)
	}

	eventsA, err := repo.List(ctx, tenantA, audit.Filters{}, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list tenant A events: %v", err)
	}
	if len(eventsA) != 1 {
		t.Fatalf("tenant A event count = %d, want 1 after duplicate no-op", len(eventsA))
	}
	if len(eventsA[0].Diff) != 1 || eventsA[0].Diff[0].After != "<redacted>" || strings.Contains(eventsA[0].Diff[0].After, "acceptance-secret") {
		t.Fatalf("stored audit diff = %#v, want redacted secret", eventsA[0].Diff)
	}

	eventsB, err := repo.List(ctx, tenantB, audit.Filters{}, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list tenant B events: %v", err)
	}
	if len(eventsB) != 0 {
		t.Fatalf("tenant B saw %d tenant-A audit events; forced RLS leaked data", len(eventsB))
	}
}

func TestAcceptanceNoopImageInstallationRequiresNoKey(t *testing.T) {
	pool := requireAcceptanceDB(t)
	ctx := context.Background()
	tenantID := createTenant(t, ctx, pool, "acceptance-image")
	executorRepo := executors.NewRepository(pool)
	installation := createInstallationBinding(t, ctx, executorRepo, tenantID, executors.SKUImageAssetGenerator, "Acceptance noop image", executors.KindIntegration, json.RawMessage(`{"provider":"noop"}`), nil)
	if installation.ExecutorInstallationId == "" {
		t.Fatal("real noop image installation did not receive an ID")
	}

	validator := image.NewConfigValidator()
	configJSON := json.RawMessage(`{"provider":"noop"}`)
	if err := validator.ValidateConfig(configJSON); err != nil {
		t.Fatalf("validate noop image installation config: %v", err)
	}
	config, err := image.ParseInstallationConfig(configJSON)
	if err != nil {
		t.Fatalf("parse noop image installation config: %v", err)
	}
	provider, err := image.NewProviderResolver(map[string]string{image.ProviderOpenAI: "HARPIA_UNUSED_OPENAI_KEY"})(config)
	if err != nil {
		t.Fatalf("resolve noop image provider without a key: %v", err)
	}
	if provider == nil {
		t.Fatal("noop provider resolver returned nil provider")
	}

}

func TestAcceptanceImageTemporalExecution(t *testing.T) {
	// TODO(enforce-acceptance-before-archive): add full Temporal execution once
	// the production worker harness can be started deterministically in CI.
	t.Skip("TODO(enforce-acceptance-before-archive): full image execution requires Temporal worker wiring outside this acceptance boundary")
}

func seedAndLoadLinkedInTemplate(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (*plans.PlanTemplate, *plansv1.PlanTemplate) {
	t.Helper()
	if err := artifacts.EnsureArtifactTypes(ctx, pool); err != nil {
		t.Fatalf("seed artifact types: %v", err)
	}
	if err := executors.EnsureCatalog(ctx, pool); err != nil {
		t.Fatalf("seed executor catalog: %v", err)
	}
	if err := plans.EnsurePlanTemplates(ctx, pool); err != nil {
		t.Fatalf("seed embedded plan templates: %v", err)
	}
	repo := plans.NewRepository(pool)
	template, err := repo.GetTemplateByKey(ctx, "linkedin-content-studio")
	if err != nil {
		t.Fatalf("load real linkedin-content-studio template: %v", err)
	}
	templateProto, err := (&plans.AssistantTemplates{Repo: repo}).GetTemplateByID(ctx, template.ID)
	if err != nil {
		t.Fatalf("convert real linkedin-content-studio template through production adapter: %v", err)
	}
	return template, templateProto
}

func createTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`, id, prefix+"-"+id.String()[:8], prefix+"-"+id.String()[:8]); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return id
}

func createRequiredLinkedInBindings(t *testing.T, ctx context.Context, repo *executors.Repository, tenantID uuid.UUID) []*plansv1.SlotBinding {
	t.Helper()
	return []*plansv1.SlotBinding{
		createInstallationBinding(t, ctx, repo, tenantID, executors.SKURSSNewsFeed, "Acceptance RSS", executors.KindIntegration, json.RawMessage(`{"feeds":["https://example.com/feed.xml"]}`), nil),
		createInstallationBinding(t, ctx, repo, tenantID, executors.SKUNewsletterWriterSenior, "Acceptance writer", executors.KindAgent, nil, &agentManifest{ID: "newsletter-writer-senior", Version: "0.1.0"}),
		createInstallationBinding(t, ctx, repo, tenantID, executors.SKULinkedinContentSpecialist, "Acceptance specialist", executors.KindAgent, nil, &agentManifest{ID: "linkedin-content-specialist", Version: "0.1.0"}),
		createInstallationBinding(t, ctx, repo, tenantID, executors.SKULinkedInPublish, "Acceptance LinkedIn", executors.KindIntegration, json.RawMessage(`{"mode":"approval_only"}`), nil),
	}
}

type agentManifest struct{ ID, Version string }

func createInstallationBinding(t *testing.T, ctx context.Context, repo *executors.Repository, tenantID uuid.UUID, skuKey, displayName, kind string, config json.RawMessage, manifest *agentManifest) *plansv1.SlotBinding {
	t.Helper()
	sku, err := repo.GetSKUByKey(ctx, skuKey)
	if err != nil {
		t.Fatalf("get %s sku: %v", skuKey, err)
	}
	if _, err := repo.CreateEntitlement(ctx, tenantID, sku.ID, nil); err != nil {
		t.Fatalf("create %s entitlement: %v", skuKey, err)
	}
	connected := "connected"
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	installation := &executors.ExecutorInstallation{
		TenantID:         tenantID,
		ExecutorSKUID:    sku.ID,
		Kind:             kind,
		DisplayName:      displayName,
		Enabled:          true,
		ConnectionStatus: &connected,
		ConfigJSON:       config,
	}
	if manifest != nil {
		installation.ManifestID = &manifest.ID
		installation.ManifestVersion = &manifest.Version
	}
	created, err := repo.CreateInstallation(ctx, installation)
	if err != nil {
		t.Fatalf("create %s installation: %v", skuKey, err)
	}
	kindProto := plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION
	if kind == executors.KindAgent {
		kindProto = plansv1.ExecutorKind_EXECUTOR_KIND_AGENT
	}
	stepKey := map[string]string{
		executors.SKURSSNewsFeed:               "fetch-news",
		executors.SKUNewsletterWriterSenior:    "write-draft",
		executors.SKULinkedinContentSpecialist: "author-content",
		executors.SKULinkedInPublish:           "publish",
		executors.SKUImageAssetGenerator:       "generate-image",
	}[skuKey]
	return &plansv1.SlotBinding{StepKey: stepKey, ExecutorSkuId: sku.ID.String(), ExecutorInstallationId: created.ID.String(), ExecutorKind: kindProto}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertNoBinding(t *testing.T, bindings []*plansv1.SlotBinding, stepKey string) {
	t.Helper()
	for _, binding := range bindings {
		if binding.GetStepKey() == stepKey {
			t.Fatalf("unexpected binding for opted-out step %q: %#v", stepKey, binding)
		}
	}
}
