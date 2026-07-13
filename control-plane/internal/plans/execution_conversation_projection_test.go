package plans

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
	"github.com/harpia/control-plane/internal/threads"
)

func TestExecutionConversationProjectionReadsAreTenantAndExecutionScoped(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run execution conversation projection integration test")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tenantID, userID, err := database.EnsureDevData(ctx, pool)
	if err != nil {
		t.Fatalf("ensure dev data: %v", err)
	}
	cleanupExecutionProjectionTestRows(t, ctx, pool, tenantID)
	repository := NewRepository(pool)
	templateID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO plan_templates (id, key, name, description, vertical, version)
		VALUES ($1, $2, 'Execution projection test', '', '', 1)`, templateID, "execution-projection-test-"+uuid.NewString()); err != nil {
		t.Fatalf("create isolated template: %v", err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DELETE FROM plan_templates WHERE id = $1", templateID) }()
	thread, err := threads.NewRepository(pool).Create(ctx, threads.CreateInput{TenantID: tenantID, Title: "execution projection test " + uuid.NewString(), CreatedByUserID: uuid.NullUUID{UUID: userID, Valid: true}})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	configuration, err := repository.CreateConfiguration(ctx, &PlanConfiguration{
		TenantID: tenantID, PlanTemplateID: templateID, PlanTemplateVersion: 1,
		Status: ConfigurationStatusRunnable, Kind: ConfigurationKindOneShot, SeedArtifacts: []byte("[]"), SlotBindings: []byte("[]"), OverseerBindings: []byte("[]"), BehaviorPolicies: []byte("{}"), ParameterValues: []byte("{}"), OriginThreadID: thread.ID,
	})
	if err != nil {
		t.Fatalf("create configuration: %v", err)
	}
	defer cleanupExecutionProjectionTestRows(t, context.Background(), pool, tenantID)

	first, err := repository.CreateExecution(ctx, &PlanExecution{TenantID: tenantID, PlanConfigurationID: configuration.ID, PlanConfigurationSnapshot: []byte(`{"schema_version":1}`), Status: ExecutionStatusRunning})
	if err != nil {
		t.Fatalf("create first execution: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := repository.CreateExecution(ctx, &PlanExecution{TenantID: tenantID, PlanConfigurationID: configuration.ID, PlanConfigurationSnapshot: []byte(`{"schema_version":1}`), Status: ExecutionStatusRunning})
	if err != nil {
		t.Fatalf("create second execution: %v", err)
	}
	stepA, err := repository.CreateStepExecution(ctx, &StepExecution{TenantID: tenantID, PlanExecutionID: first.ID, PlanStepKey: "publish", Status: StepStatusAwaitingElicitation, ExecutorInstallationSnapshot: []byte("{}")})
	if err != nil {
		t.Fatalf("create execution A step: %v", err)
	}
	stepB, err := repository.CreateStepExecution(ctx, &StepExecution{TenantID: tenantID, PlanExecutionID: second.ID, PlanStepKey: "publish", Status: StepStatusAwaitingElicitation, ExecutorInstallationSnapshot: []byte("{}")})
	if err != nil {
		t.Fatalf("create execution B step: %v", err)
	}
	if _, err := repository.UpsertElicitation(ctx, &Elicitation{TenantID: tenantID, PlanExecutionID: first.ID, StepExecutionID: stepA.ID, PlanStepKey: "publish", ElicitationThreadID: "a", Status: ElicitationStatusPending, SchemaJSON: []byte("{}")}); err != nil {
		t.Fatalf("create execution A elicitation: %v", err)
	}
	if _, err := repository.UpsertElicitation(ctx, &Elicitation{TenantID: tenantID, PlanExecutionID: second.ID, StepExecutionID: stepB.ID, PlanStepKey: "publish", ElicitationThreadID: "b", Status: ElicitationStatusPending, SchemaJSON: []byte("{}")}); err != nil {
		t.Fatalf("create execution B elicitation: %v", err)
	}

	pendingA, err := repository.GetPendingExecutionInteractions(ctx, tenantID, first.ID)
	if err != nil {
		t.Fatalf("load pending execution A interactions: %v", err)
	}
	if len(pendingA) != 1 || pendingA[0].StepExecutionID != stepA.ID || pendingA[0].PlanExecutionID != first.ID {
		t.Fatalf("execution A projection crossed same-key concurrent run: %+v", pendingA)
	}
	latest, err := repository.GetLatestNonTerminalExecutionForConfiguration(ctx, tenantID, configuration.ID)
	if err != nil {
		t.Fatalf("load latest non-terminal execution: %v", err)
	}
	if latest == nil || latest.ID != second.ID {
		t.Fatalf("latest execution = %+v, want exact second execution %s", latest, second.ID)
	}

	otherTenant := uuid.New()
	if _, _, err := database.EnsureDevData(ctx, pool); err != nil {
		t.Fatalf("ensure tenant baseline: %v", err)
	}
	if _, err := repository.GetExecution(ctx, otherTenant, first.ID); err == nil {
		t.Fatal("cross-tenant execution lookup must be not found")
	}
}

func cleanupExecutionProjectionTestRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Helper()
	// This test creates a configuration, and CreateConfiguration records it as
	// the thread's selected configuration. Clear that reverse FK before deleting
	// the configuration so the fixture never leaves catalog-reconcile debris.
	err := database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		if _, err := q.Exec(ctx, `UPDATE threads SET active_plan_configuration_id = NULL
			WHERE tenant_id = $1 AND title LIKE 'execution projection test%'`, tenantID); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM plan_executions
			WHERE tenant_id = $1 AND plan_configuration_id IN (
				SELECT id FROM plan_configurations WHERE tenant_id = $1 AND origin_thread_id IN (
					SELECT id FROM threads WHERE tenant_id = $1 AND title LIKE 'execution projection test%'
				)
			)`, tenantID); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM plan_configurations
			WHERE tenant_id = $1 AND origin_thread_id IN (
				SELECT id FROM threads WHERE tenant_id = $1 AND title LIKE 'execution projection test%'
			)`, tenantID); err != nil {
			return err
		}
		_, err := q.Exec(ctx, "DELETE FROM threads WHERE tenant_id = $1 AND title LIKE 'execution projection test%'", tenantID)
		return err
	})
	if err != nil {
		t.Fatalf("clean execution projection fixture: %v", err)
	}
}
