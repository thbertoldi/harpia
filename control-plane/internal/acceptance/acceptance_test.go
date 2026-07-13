// Package acceptance exercises release-critical behavior through production
// adapters. It is intentionally separate from unit packages so `mise run
// acceptance` cannot silently pass without a real app-role database.
package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/audit"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/integrations/image"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/planassistant"
	"github.com/harpia/control-plane/internal/plans"
	"github.com/harpia/control-plane/internal/threads"
	"github.com/harpia/control-plane/internal/workflow"
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

func TestAcceptanceLiveLLMExecution(t *testing.T) {
	// TODO(enforce-acceptance-before-archive): this release proof uses a
	// contract-faithful agent activity in workflow/plans_test.go. A live provider
	// scenario needs deterministic, budgeted test credentials and remains
	// intentionally visible rather than silently passing as a mock.
	t.Skip("TODO(enforce-acceptance-before-archive): live LLM execution requires deterministic provider credentials")
}

func TestAcceptanceLiveLinkedInExecution(t *testing.T) {
	// TODO(enforce-acceptance-before-archive): do not publish to a live LinkedIn
	// account from acceptance. The real request-routing and pinned-artifact
	// boundary is covered below with production repositories.
	t.Skip("TODO(enforce-acceptance-before-archive): live LinkedIn publishing requires an isolated non-production account")
}

// TestAcceptanceExecutionDrivenConversationBoundaryAndSnapshots exercises the
// production plan, executor, chat, and repository adapters at the explicit
// configuration-to-execution boundary. Agent-provider behavior is deliberately
// not live here; workflow/plans_test.go supplies the contract-faithful Temporal
// activity coverage for that boundary.
func TestAcceptanceExecutionDrivenConversationBoundaryAndSnapshots(t *testing.T) {
	pool := requireAcceptanceDB(t)
	ctx := context.Background()
	fixture := newExecutionConversationFixture(t, ctx, pool, "acceptance-edc-boundary")

	ready, err := planassistant.DeriveConversationTurn(planassistant.TurnInput{Configuration: fixture.configuration})
	if err != nil {
		t.Fatalf("derive RUNNABLE ready turn: %v", err)
	}
	if ready.Kind != planassistant.ConversationTurnReadyToRun {
		t.Fatalf("RUNNABLE turn = %q, want READY_TO_RUN", ready.Kind)
	}

	firstResponse, err := fixture.handler.CreatePlanExecution(fixture.requestContext, connect.NewRequest(&plansv1.CreatePlanExecutionRequest{
		TenantId: fixture.tenantID.String(), PlanConfigurationId: fixture.configuration.GetId(),
	}))
	if err != nil {
		t.Fatalf("explicit Run: %v", err)
	}
	first := firstResponse.Msg.GetPlanExecution()
	if first.GetStatus() != plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_PENDING {
		t.Fatalf("new execution status = %q, want pending", first.GetStatus())
	}
	if first.PlanTemplateSnapshot == nil || first.PlanTemplateSnapshot.GetVersion() == 0 {
		t.Fatal("Run response omitted the frozen PlanTemplate snapshot")
	}
	if fixture.starter.inputs[0].PlanExecutionID != first.GetId() {
		t.Fatalf("Run started workflow for %q, want %q", fixture.starter.inputs[0].PlanExecutionID, first.GetId())
	}

	firstID := uuid.MustParse(first.GetId())
	if err := fixture.assistant.OnExecutionEvent(ctx, fixture.tenantID, firstID, "RUN_QUEUED"); err != nil {
		t.Fatalf("project queued execution conversation: %v", err)
	}
	if err := fixture.runtime.StartPlanExecution(ctx, fixture.tenantID, firstID); err != nil {
		t.Fatalf("start execution through production runtime: %v", err)
	}
	assertExecutionPromptStates(t, ctx, fixture.chat, fixture.tenantID, fixture.thread.ID, firstID,
		chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_QUEUED,
		chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_RUNNING,
	)

	firstStored, err := fixture.planRepo.GetExecution(ctx, fixture.tenantID, firstID)
	if err != nil {
		t.Fatalf("load first execution: %v", err)
	}
	frozenFirstSnapshot := append([]byte(nil), firstStored.PlanConfigurationSnapshot...)

	// This is the Reconfigure boundary: changing the current configuration is a
	// future-run concern only. The active execution remains RUNNING and retains
	// both its configuration and template graph byte-for-byte.
	updated := updateFixtureConfiguration(t, fixture, `{"theme":"current configuration theme","language":"pt-BR","tone":"friendly and clear","source_group":"`+fixture.bindings["fetch-news"].GetExecutorInstallationId()+`","date_range":{"preset":"schedule_window"},"include_carousel":"no","include_images":"no","approval_mode":"require_approval","elicitation_timeout_behavior":"pause_until_answered"}`)
	if updated.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE {
		t.Fatalf("Reconfigure changed RUNNABLE status to %q", updated.GetStatus())
	}
	firstAfterEdit, err := fixture.planRepo.GetExecution(ctx, fixture.tenantID, firstID)
	if err != nil {
		t.Fatalf("reload in-flight execution: %v", err)
	}
	if !bytes.Equal(firstAfterEdit.PlanConfigurationSnapshot, frozenFirstSnapshot) {
		t.Fatal("editing PlanConfiguration mutated the in-flight execution snapshot")
	}

	// Run-again uses the current configuration snapshot, not the failed run's
	// frozen one.
	runAgain, err := fixture.handler.CreatePlanExecution(fixture.requestContext, connect.NewRequest(&plansv1.CreatePlanExecutionRequest{
		TenantId: fixture.tenantID.String(), PlanConfigurationId: fixture.configuration.GetId(),
	}))
	if err != nil {
		t.Fatalf("Run-again from current configuration: %v", err)
	}
	runAgainStored, err := fixture.planRepo.GetExecution(ctx, fixture.tenantID, uuid.MustParse(runAgain.Msg.GetPlanExecution().GetId()))
	if err != nil {
		t.Fatalf("load run-again execution: %v", err)
	}
	if bytes.Equal(runAgainStored.PlanConfigurationSnapshot, frozenFirstSnapshot) || !bytes.Contains(runAgainStored.PlanConfigurationSnapshot, []byte("current configuration theme")) {
		t.Fatal("Run-again did not take a fresh snapshot of the current configuration")
	}

	// Build a failed execution with a completed upstream output. Retry must copy
	// exactly this old snapshot and hand that upstream ArtifactRef to Temporal.
	upstream, err := fixture.planRepo.CreateStepExecution(ctx, &plans.StepExecution{
		TenantID: fixture.tenantID, PlanExecutionID: firstID, PlanStepKey: "fetch-news", Status: "completed",
		OutputArtifactID: "upstream-output", OutputArtifactTypeKey: "harpia.artifacts.v1.NewsList", ExecutorInstallationSnapshot: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create completed upstream step: %v", err)
	}
	failed, err := fixture.planRepo.CreateStepExecution(ctx, &plans.StepExecution{
		TenantID: fixture.tenantID, PlanExecutionID: firstID, PlanStepKey: "write-draft", Status: "failed", ExecutorInstallationSnapshot: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create failed step: %v", err)
	}
	if upstream.OutputArtifactID != "upstream-output" {
		t.Fatalf("upstream artifact = %q", upstream.OutputArtifactID)
	}
	completedAt := time.Now().UTC()
	if err := fixture.planRepo.UpdateExecutionStatus(ctx, fixture.tenantID, firstID, "failed", &completedAt); err != nil {
		t.Fatalf("mark original execution failed: %v", err)
	}

	retry, err := fixture.handler.RetryPlanExecution(fixture.requestContext, connect.NewRequest(&plansv1.RetryPlanExecutionRequest{
		TenantId: fixture.tenantID.String(), PlanExecutionId: firstID.String(), StepExecutionId: failed.ID.String(),
	}))
	if err != nil {
		t.Fatalf("RetryPlanExecution: %v", err)
	}
	retryID := uuid.MustParse(retry.Msg.GetPlanExecution().GetId())
	retryStored, err := fixture.planRepo.GetExecution(ctx, fixture.tenantID, retryID)
	if err != nil {
		t.Fatalf("load retry execution: %v", err)
	}
	if !bytes.Equal(retryStored.PlanConfigurationSnapshot, frozenFirstSnapshot) {
		t.Fatal("Retry did not reuse the failed execution's frozen snapshot")
	}
	retryInput := fixture.starter.inputs[len(fixture.starter.inputs)-1]
	if got := retryInput.RetryStepArtifactsByKey["fetch-news"].ArtifactID; got != "upstream-output" {
		t.Fatalf("Retry upstream artifact = %q, want upstream-output", got)
	}
	if retryInput.RetryFromStepKey != "write-draft" {
		t.Fatalf("RetryFromStepKey = %q, want write-draft", retryInput.RetryFromStepKey)
	}

	// A subsequent edit still cannot rewrite either non-terminal Run-again or
	// retry snapshots, and it cannot demote or clone the configuration.
	runAgainSnapshot := append([]byte(nil), runAgainStored.PlanConfigurationSnapshot...)
	retrySnapshot := append([]byte(nil), retryStored.PlanConfigurationSnapshot...)
	updateFixtureConfiguration(t, fixture, `{"theme":"later edit","language":"pt-BR","tone":"executive and direct","source_group":"`+fixture.bindings["fetch-news"].GetExecutorInstallationId()+`","date_range":{"preset":"schedule_window"},"include_carousel":"no","include_images":"no","approval_mode":"require_approval","elicitation_timeout_behavior":"pause_until_answered"}`)
	assertExecutionSnapshotUnchanged(t, ctx, fixture.planRepo, fixture.tenantID, runAgainStored.ID, runAgainSnapshot)
	assertExecutionSnapshotUnchanged(t, ctx, fixture.planRepo, fixture.tenantID, retryID, retrySnapshot)
	configs, err := fixture.planRepo.ListConfigurations(ctx, fixture.tenantID, nil, &fixture.thread.ID, "", "", 10, 0)
	if err != nil {
		t.Fatalf("list configurations after reconfigure: %v", err)
	}
	if len(configs) != 1 || configs[0].ID != uuid.MustParse(fixture.configuration.GetId()) || configs[0].Status != "runnable" {
		t.Fatalf("Reconfigure cloned or demoted configuration: %#v", configs)
	}
}

// TestAcceptanceExecutionDrivenConversationConcurrentReviewIsolation is the
// primary 1:N:M safety proof. Two configurations share one Thread, two live
// executions share the same plan_step_key, and an action addressed by request
// ID is routed only to the owning workflow/StepExecution/version-pinned review.
func TestAcceptanceExecutionDrivenConversationConcurrentReviewIsolation(t *testing.T) {
	pool := requireAcceptanceDB(t)
	ctx := context.Background()
	fixture := newExecutionConversationFixture(t, ctx, pool, "acceptance-edc-concurrency")

	configurationA := uuid.MustParse(fixture.configuration.GetId())
	configA, err := fixture.planRepo.GetConfiguration(ctx, fixture.tenantID, configurationA)
	if err != nil {
		t.Fatalf("load configuration A: %v", err)
	}
	configurationB, err := fixture.planRepo.CreateConfiguration(ctx, &plans.PlanConfiguration{
		TenantID: fixture.tenantID, PlanTemplateID: configA.PlanTemplateID, PlanTemplateVersion: configA.PlanTemplateVersion,
		Status: "runnable", Kind: configA.Kind, SeedArtifacts: configA.SeedArtifacts, SlotBindings: configA.SlotBindings,
		OverseerBindings: configA.OverseerBindings, BehaviorPolicies: configA.BehaviorPolicies, Schedule: configA.Schedule,
		ParameterValues: configA.ParameterValues, OriginThreadID: fixture.thread.ID,
	})
	if err != nil {
		t.Fatalf("create configuration B in same thread: %v", err)
	}

	executionA := createAcceptanceExecution(t, fixture, configurationA)
	executionB := createAcceptanceExecution(t, fixture, configurationB.ID)
	stepA := createAcceptanceReviewStep(t, ctx, fixture, executionA.ID)
	stepB := createAcceptanceReviewStep(t, ctx, fixture, executionB.ID)
	artifactA := createAcceptanceReviewArtifact(t, ctx, pool, fixture, executionA.ID, stepA.ID, "review-a")
	artifactB := createAcceptanceReviewArtifact(t, ctx, pool, fixture, executionB.ID, stepB.ID, "review-b")
	reviewA, err := fixture.planRepo.CreatePlanReviewRequest(ctx, &plans.PlanReviewRequest{
		TenantID: fixture.tenantID, PlanExecutionID: executionA.ID, StepExecutionID: stepA.ID, PlanStepKey: "author-content",
		SubjectArtifactID: artifactA.ID, SubjectArtifactVersionID: artifactA.CurrentVersionID.UUID,
		SubjectArtifactTypeKey: artifacts.TypeKeyLinkedInPost, SubjectContentHash: artifactA.ContentHash,
	})
	if err != nil {
		t.Fatalf("create review A: %v", err)
	}
	reviewB, err := fixture.planRepo.CreatePlanReviewRequest(ctx, &plans.PlanReviewRequest{
		TenantID: fixture.tenantID, PlanExecutionID: executionB.ID, StepExecutionID: stepB.ID, PlanStepKey: "author-content",
		SubjectArtifactID: artifactB.ID, SubjectArtifactVersionID: artifactB.CurrentVersionID.UUID,
		SubjectArtifactTypeKey: artifacts.TypeKeyLinkedInPost, SubjectContentHash: artifactB.ContentHash,
	})
	if err != nil {
		t.Fatalf("create review B: %v", err)
	}

	fixture.starter.receivedByWorkflow = map[string][]workflow.ReviewDecisionSignal{}
	handler, err := plans.NewPlanHandler(fixture.planRepo, fixture.executorRepo, nil, fixture.chat, nil, fixture.starter)
	if err != nil {
		t.Fatalf("construct review handler: %v", err)
	}
	response, err := handler.RespondToReviewRequest(fixture.requestContext, connect.NewRequest(&plansv1.RespondToReviewRequestRequest{
		TenantId: fixture.tenantID.String(), ReviewRequestId: reviewA.ID.String(),
		Decision: &plansv1.ReviewDecision{Kind: plansv1.ReviewDecisionKind_REVIEW_DECISION_KIND_ACCEPT},
	}))
	if err != nil {
		t.Fatalf("accept review A: %v", err)
	}
	if response.Msg.GetReviewRequest().GetStatus() != plansv1.ReviewRequestStatus_REVIEW_REQUEST_STATUS_PENDING {
		t.Fatal("command handler must leave terminal review persistence to the exact Temporal workflow")
	}
	workflowA, workflowB := workflow.PlanWorkflowID(executionA.ID.String()), workflow.PlanWorkflowID(executionB.ID.String())
	if got := fixture.starter.receivedByWorkflow[workflowA]; len(got) != 1 || got[0].StepExecutionID != stepA.ID.String() || got[0].ReviewRequestID != reviewA.ID.String() {
		t.Fatalf("request A signal = %#v, want only execution A / step A / review A", got)
	}
	if got := fixture.starter.receivedByWorkflow[workflowB]; len(got) != 0 {
		t.Fatalf("request A crossed into concurrent execution B: %#v", got)
	}

	// This is the production persistence operation called by
	// ResolveReviewRequestActivity after its workflow accepts the exact signal.
	accepted, err := fixture.planRepo.MarkReviewDecided(ctx, fixture.tenantID, reviewA.ID, "accept", "", uuid.NullUUID{})
	if err != nil {
		t.Fatalf("resolve exact review A: %v", err)
	}
	if accepted.PlanExecutionID != executionA.ID || accepted.StepExecutionID != stepA.ID || accepted.SubjectArtifactVersionID != artifactA.CurrentVersionID.UUID || accepted.SubjectContentHash != artifactA.ContentHash {
		t.Fatalf("accepted review lost its exact execution/step/version: %#v", accepted)
	}
	pendingB, err := fixture.planRepo.GetPlanReviewRequest(ctx, fixture.tenantID, reviewB.ID)
	if err != nil {
		t.Fatalf("load untouched review B: %v", err)
	}
	if pendingB.Status != "pending" || pendingB.PlanExecutionID != executionB.ID || pendingB.StepExecutionID != stepB.ID || pendingB.SubjectArtifactVersionID != artifactB.CurrentVersionID.UUID {
		t.Fatalf("concurrent review B changed after request A decision: %#v", pendingB)
	}

	// Each execution response is built from its own persisted snapshot. The
	// template is frozen independently even though both executions share a step
	// key and originate in the same thread.
	storedA, err := fixture.planRepo.GetExecution(ctx, fixture.tenantID, executionA.ID)
	if err != nil {
		t.Fatalf("load execution A: %v", err)
	}
	storedB, err := fixture.planRepo.GetExecution(ctx, fixture.tenantID, executionB.ID)
	if err != nil {
		t.Fatalf("load execution B: %v", err)
	}
	if bytes.Equal(storedA.PlanConfigurationSnapshot, storedB.PlanConfigurationSnapshot) ||
		!bytes.Contains(storedA.PlanConfigurationSnapshot, []byte(configurationA.String())) ||
		!bytes.Contains(storedB.PlanConfigurationSnapshot, []byte(configurationB.ID.String())) {
		t.Fatal("concurrent execution cards do not retain their own frozen configuration/template snapshots")
	}
}

type executionConversationFixture struct {
	tenantID       uuid.UUID
	thread         *threads.Thread
	configuration  *plansv1.PlanConfiguration
	bindings       map[string]*plansv1.SlotBinding
	planRepo       *plans.Repository
	executorRepo   *executors.Repository
	chat           chat.Store
	handler        *plans.PlanHandler
	runtime        *plans.RuntimeRepository
	assistant      *planassistant.Controller
	starter        *acceptanceWorkflowStarter
	requestContext context.Context
}

func newExecutionConversationFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string) *executionConversationFixture {
	t.Helper()
	template, templateProto := seedAndLoadLinkedInTemplate(t, ctx, pool)
	tenantID := createTenant(t, ctx, pool, prefix)
	executorRepo := executors.NewRepository(pool)
	bindingList := createRequiredLinkedInBindings(t, ctx, executorRepo, tenantID)
	bindings := make(map[string]*plansv1.SlotBinding, len(bindingList))
	for _, binding := range bindingList {
		bindings[binding.GetStepKey()] = binding
	}
	parameters := `{"theme":"frozen original theme","language":"pt-BR","tone":"analytical, concise, and practical","source_group":"` + bindings["fetch-news"].GetExecutorInstallationId() + `","date_range":{"preset":"schedule_window"},"include_carousel":"no","include_images":"no","approval_mode":"require_approval","elicitation_timeout_behavior":"pause_until_answered"}`
	seeds, _, policies, _, err := plans.MaterializePlanConfiguration(templateProto, jsonRawObject(t, parameters))
	if err != nil {
		t.Fatalf("materialize acceptance configuration: %v", err)
	}
	thread, err := threads.NewRepository(pool).Create(ctx, threads.CreateInput{TenantID: tenantID, Title: prefix + " " + uuid.NewString()})
	if err != nil {
		t.Fatalf("create acceptance thread: %v", err)
	}
	overseers := []*plansv1.OverseerBinding{{StepKey: "write-draft", OverseerUserId: uuid.NewString()}, {StepKey: "author-content", OverseerUserId: uuid.NewString()}}
	seedJSON, _ := json.Marshal(seeds)
	slotJSON, _ := json.Marshal(bindingList)
	overseerJSON, _ := json.Marshal(overseers)
	policyJSON, _ := json.Marshal(policies)
	planRepo := plans.NewRepository(pool)
	config, err := planRepo.CreateConfiguration(ctx, &plans.PlanConfiguration{
		TenantID: tenantID, PlanTemplateID: template.ID, PlanTemplateVersion: template.Version, Status: "draft", Kind: "one_shot",
		SeedArtifacts: seedJSON, SlotBindings: slotJSON, OverseerBindings: overseerJSON, BehaviorPolicies: policyJSON,
		Schedule: []byte("null"), ParameterValues: []byte(parameters), OriginThreadID: thread.ID,
	})
	if err != nil {
		t.Fatalf("create DRAFT configuration: %v", err)
	}
	chatStore := chat.NewPostgresStore(pool)
	starter := &acceptanceWorkflowStarter{}
	handler, err := plans.NewPlanHandler(planRepo, executorRepo, nil, chatStore, nil, starter)
	if err != nil {
		t.Fatalf("construct plan handler: %v", err)
	}
	requestContext := identity.WithRequestContext(ctx, identity.RequestContext{TenantID: tenantID, UserID: uuid.NewString(), Roles: []string{"leader"}})
	promoted, err := handler.UpdatePlanConfiguration(requestContext, connect.NewRequest(&plansv1.UpdatePlanConfigurationRequest{
		TenantId: tenantID.String(), PlanConfigurationId: config.ID.String(), Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		OverseerBindings: overseers, ParameterValuesJson: parameters, AnnounceSaved: true,
	}))
	if err != nil {
		t.Fatalf("promote DRAFT configuration to RUNNABLE: %v", err)
	}
	assistant := &planassistant.Controller{
		Chat:       chatStore,
		Configs:    &plans.AssistantConfigurationStore{Repo: planRepo, Executors: executorRepo},
		Executions: &plans.AssistantExecutionStore{Repo: planRepo},
	}
	runtime := plans.NewRuntimeRepository(planRepo, executorRepo, chatStore).WithExecutionConversationSink(assistant)
	return &executionConversationFixture{tenantID: tenantID, thread: thread, configuration: promoted.Msg.GetPlanConfiguration(), bindings: bindings, planRepo: planRepo, executorRepo: executorRepo, chat: chatStore, handler: handler, runtime: runtime, assistant: assistant, starter: starter, requestContext: requestContext}
}

func updateFixtureConfiguration(t *testing.T, fixture *executionConversationFixture, parameters string) *plansv1.PlanConfiguration {
	t.Helper()
	updated, err := fixture.handler.UpdatePlanConfiguration(fixture.requestContext, connect.NewRequest(&plansv1.UpdatePlanConfigurationRequest{
		TenantId: fixture.tenantID.String(), PlanConfigurationId: fixture.configuration.GetId(), Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		OverseerBindings: fixture.configuration.GetOverseerBindings(), ParameterValuesJson: parameters, AnnounceSaved: true,
	}))
	if err != nil {
		t.Fatalf("update current configuration: %v", err)
	}
	fixture.configuration = updated.Msg.GetPlanConfiguration()
	return fixture.configuration
}

func createAcceptanceExecution(t *testing.T, fixture *executionConversationFixture, configurationID uuid.UUID) *plans.PlanExecution {
	t.Helper()
	response, err := fixture.handler.CreatePlanExecution(fixture.requestContext, connect.NewRequest(&plansv1.CreatePlanExecutionRequest{
		TenantId: fixture.tenantID.String(), PlanConfigurationId: configurationID.String(),
	}))
	if err != nil {
		t.Fatalf("create execution for %s: %v", configurationID, err)
	}
	execution, err := fixture.planRepo.GetExecution(context.Background(), fixture.tenantID, uuid.MustParse(response.Msg.GetPlanExecution().GetId()))
	if err != nil {
		t.Fatalf("load execution for %s: %v", configurationID, err)
	}
	return execution
}

func createAcceptanceReviewStep(t *testing.T, ctx context.Context, fixture *executionConversationFixture, executionID uuid.UUID) *plans.StepExecution {
	t.Helper()
	step, err := fixture.planRepo.CreateStepExecution(ctx, &plans.StepExecution{TenantID: fixture.tenantID, PlanExecutionID: executionID, PlanStepKey: "author-content", Status: "running", ExecutorInstallationSnapshot: []byte(`{}`)})
	if err != nil {
		t.Fatalf("create same-key review step for %s: %v", executionID, err)
	}
	return step
}

func createAcceptanceReviewArtifact(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture *executionConversationFixture, executionID, stepID uuid.UUID, suffix string) *artifacts.Artifact {
	t.Helper()
	typeRecord, err := artifacts.NewRepository(pool).GetTypeByKey(ctx, artifacts.TypeKeyLinkedInPost)
	if err != nil {
		t.Fatalf("load LinkedInPost artifact type: %v", err)
	}
	created, err := artifacts.NewRepository(pool).CreateArtifact(ctx, &artifacts.Artifact{
		TenantID: fixture.tenantID, ArtifactTypeID: typeRecord.ID, StorageURI: "acceptance/" + suffix,
		ContentHash:         "acceptance-" + suffix + "-" + uuid.NewString(),
		PlanConfigurationID: uuid.NullUUID{UUID: uuid.MustParse(fixture.configuration.GetId()), Valid: true},
		PlanExecutionID:     uuid.NullUUID{UUID: executionID, Valid: true}, StepExecutionID: uuid.NullUUID{UUID: stepID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create version-pinned artifact %s: %v", suffix, err)
	}
	if !created.CurrentVersionID.Valid {
		t.Fatalf("artifact %s has no initial version", suffix)
	}
	return created
}

func assertExecutionPromptStates(t *testing.T, ctx context.Context, store chat.Store, tenantID, threadID, executionID uuid.UUID, want ...chatv1.ExecutionPromptState) {
	t.Helper()
	messages, err := store.ListMessages(ctx, tenantID, threadID.String(), 0, 0)
	if err != nil {
		t.Fatalf("list execution prompts: %v", err)
	}
	seen := make(map[chatv1.ExecutionPromptState]bool)
	for _, message := range messages {
		if message.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_EXECUTION_PROMPT || message.GetExecutionId() != executionID.String() {
			continue
		}
		payload, ok := chat.ParseExecutionPromptPayload(message.GetPayloadJson())
		if !ok {
			t.Fatalf("invalid execution prompt payload: %q", message.GetPayloadJson())
		}
		seen[payload.GetState()] = true
	}
	for _, state := range want {
		if !seen[state] {
			t.Fatalf("execution %s prompts = %#v, missing %s", executionID, seen, state)
		}
	}
}

func assertExecutionSnapshotUnchanged(t *testing.T, ctx context.Context, repo *plans.Repository, tenantID, executionID uuid.UUID, want []byte) {
	t.Helper()
	execution, err := repo.GetExecution(ctx, tenantID, executionID)
	if err != nil {
		t.Fatalf("reload execution %s: %v", executionID, err)
	}
	if !bytes.Equal(execution.PlanConfigurationSnapshot, want) {
		t.Fatalf("execution %s snapshot changed after configuration edit", executionID)
	}
}

func jsonRawObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("parse parameter values: %v", err)
	}
	return values
}

type acceptanceWorkflowStarter struct {
	inputs             []workflow.PlanWorkflowInput
	receivedByWorkflow map[string][]workflow.ReviewDecisionSignal
}

func (s *acceptanceWorkflowStarter) StartPlanWorkflow(_ context.Context, input workflow.PlanWorkflowInput) (client.WorkflowRun, error) {
	s.inputs = append(s.inputs, input)
	return acceptanceWorkflowRun{}, nil
}

type acceptanceWorkflowRun struct{}

func (acceptanceWorkflowRun) Get(context.Context, interface{}) error { return nil }
func (acceptanceWorkflowRun) GetWithOptions(context.Context, interface{}, client.WorkflowRunGetOptions) error {
	return nil
}
func (acceptanceWorkflowRun) GetID() string    { return "acceptance-workflow" }
func (acceptanceWorkflowRun) GetRunID() string { return "acceptance-run" }

func (s *acceptanceWorkflowStarter) SignalPlanReviewDecision(_ context.Context, workflowID, _ string, signal workflow.ReviewDecisionSignal) error {
	if s.receivedByWorkflow == nil {
		s.receivedByWorkflow = make(map[string][]workflow.ReviewDecisionSignal)
	}
	s.receivedByWorkflow[workflowID] = append(s.receivedByWorkflow[workflowID], signal)
	return nil
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
