package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/database"
)

type PlanTemplate struct {
	ID              uuid.UUID
	Key             string
	Name            string
	Description     string
	Vertical        string
	Version         int32
	InputParameters json.RawMessage
	Steps           []PlanStep
	Edges           []PlanStepDependency
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PlanStep struct {
	ID                    uuid.UUID
	Key                   string
	Title                 string
	Description           string
	InputArtifactTypeID   string
	OutputArtifactTypeID  string
	ExecutorRequirement   json.RawMessage
	DefaultExecutorSKUKey string
	Position              int32
}

type PlanStepDependency struct {
	FromStepKey string
	ToStepKey   string
}

type PlanConfiguration struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	WorkspaceID         uuid.NullUUID
	PlanTemplateID      uuid.UUID
	PlanTemplateVersion int32
	Status              string
	SeedArtifacts       json.RawMessage
	SlotBindings        json.RawMessage
	OverseerBindings    json.RawMessage
	BehaviorPolicies    json.RawMessage
	Schedule            json.RawMessage
	ParameterValues     json.RawMessage
	ThreadID            uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PlanExecution struct {
	ID                        uuid.UUID
	TenantID                  uuid.UUID
	PlanConfigurationID       uuid.UUID
	PlanConfigurationSnapshot json.RawMessage
	Status                    string
	TriggeredAt               *time.Time
	CompletedAt               *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	StepExecutions            []StepExecution
}

type StepExecution struct {
	ID                           uuid.UUID
	TenantID                     uuid.UUID
	PlanExecutionID              uuid.UUID
	PlanStepKey                  string
	Status                       string
	InputArtifactID              string
	OutputArtifactID             string
	ExecutorInstallationSnapshot json.RawMessage
	Attempt                      int32
	ElicitationThreadID          string
	ApprovalRequestID            string
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

type PlanApprovalRequest struct {
	ID                  string
	TenantID            uuid.UUID
	PlanExecutionID     uuid.UUID
	PlanConfigurationID uuid.UUID
	StepExecutionID     uuid.UUID
	PlanStepKey         string
	InputArtifactID     string
	Status              string
	DecisionReason      string
	RequestedAt         time.Time
	DecidedAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetTemplateByID(ctx context.Context, templateID uuid.UUID) (*PlanTemplate, error) {
	var template PlanTemplate
	err := r.pool.QueryRow(ctx,
		`SELECT id, key, name, description, vertical, version, input_parameters, created_at, updated_at
		 FROM plan_templates WHERE id = $1`,
		templateID,
	).Scan(
		&template.ID, &template.Key, &template.Name, &template.Description,
		&template.Vertical, &template.Version, &template.InputParameters, &template.CreatedAt, &template.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plan template: %w", err)
	}

	if err := r.loadTemplateGraph(ctx, &template); err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *Repository) GetTemplateByKey(ctx context.Context, key string) (*PlanTemplate, error) {
	var template PlanTemplate
	err := r.pool.QueryRow(ctx,
		`SELECT id, key, name, description, vertical, version, input_parameters, created_at, updated_at
		 FROM plan_templates WHERE key = $1`,
		key,
	).Scan(
		&template.ID, &template.Key, &template.Name, &template.Description,
		&template.Vertical, &template.Version, &template.InputParameters, &template.CreatedAt, &template.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plan template by key: %w", err)
	}

	if err := r.loadTemplateGraph(ctx, &template); err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *Repository) ListTemplates(ctx context.Context, vertical string, limit, offset int) ([]PlanTemplate, error) {
	var rows pgx.Rows
	var err error
	if vertical != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, key, name, description, vertical, version, input_parameters, created_at, updated_at
			 FROM plan_templates
			 WHERE vertical = $1
			 ORDER BY name ASC
			 LIMIT $2 OFFSET $3`,
			vertical, limit, offset,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, key, name, description, vertical, version, input_parameters, created_at, updated_at
			 FROM plan_templates
			 ORDER BY name ASC
			 LIMIT $1 OFFSET $2`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list plan templates: %w", err)
	}
	defer rows.Close()

	templates := make([]PlanTemplate, 0)
	for rows.Next() {
		var template PlanTemplate
		if err := rows.Scan(
			&template.ID, &template.Key, &template.Name, &template.Description,
			&template.Vertical, &template.Version, &template.InputParameters, &template.CreatedAt, &template.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plan template: %w", err)
		}
		if err := r.loadTemplateGraph(ctx, &template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan templates: %w", err)
	}
	return templates, nil
}

func (r *Repository) loadTemplateGraph(ctx context.Context, template *PlanTemplate) error {
	stepRows, err := r.pool.Query(ctx,
		`SELECT id, key, title, description, input_artifact_type_id, output_artifact_type_id,
		        executor_requirement, COALESCE(default_executor_sku_key, ''), position
		 FROM plan_template_steps
		 WHERE plan_template_id = $1
		 ORDER BY position ASC, key ASC`,
		template.ID,
	)
	if err != nil {
		return fmt.Errorf("load plan template steps: %w", err)
	}
	defer stepRows.Close()

	template.Steps = make([]PlanStep, 0)
	for stepRows.Next() {
		var step PlanStep
		if err := stepRows.Scan(
			&step.ID, &step.Key, &step.Title, &step.Description,
			&step.InputArtifactTypeID, &step.OutputArtifactTypeID,
			&step.ExecutorRequirement, &step.DefaultExecutorSKUKey, &step.Position,
		); err != nil {
			return fmt.Errorf("scan plan template step: %w", err)
		}
		template.Steps = append(template.Steps, step)
	}
	if err := stepRows.Err(); err != nil {
		return fmt.Errorf("iterate plan template steps: %w", err)
	}

	edgeRows, err := r.pool.Query(ctx,
		`SELECT from_step_key, to_step_key
		 FROM plan_template_step_dependencies
		 WHERE plan_template_id = $1
		 ORDER BY from_step_key ASC, to_step_key ASC`,
		template.ID,
	)
	if err != nil {
		return fmt.Errorf("load plan template dependencies: %w", err)
	}
	defer edgeRows.Close()

	template.Edges = make([]PlanStepDependency, 0)
	for edgeRows.Next() {
		var edge PlanStepDependency
		if err := edgeRows.Scan(&edge.FromStepKey, &edge.ToStepKey); err != nil {
			return fmt.Errorf("scan plan template dependency: %w", err)
		}
		template.Edges = append(template.Edges, edge)
	}
	return edgeRows.Err()
}

func (r *Repository) CreateConfiguration(ctx context.Context, config *PlanConfiguration) (*PlanConfiguration, error) {
	var created PlanConfiguration
	err := database.WithTenant(ctx, r.pool, config.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO plan_configurations (
				tenant_id, workspace_id, plan_template_id, plan_template_version, status,
				seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
				parameter_values, thread_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
			          seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
			          parameter_values, thread_id, created_at, updated_at`,
			config.TenantID, config.WorkspaceID, config.PlanTemplateID, config.PlanTemplateVersion,
			config.Status, config.SeedArtifacts, config.SlotBindings, config.OverseerBindings,
			config.BehaviorPolicies, config.Schedule, config.ParameterValues, config.ThreadID,
		)
		if err := scanConfiguration(row, &created); err != nil {
			return err
		}
		if created.ThreadID != uuid.Nil {
			if _, err := q.Exec(ctx, `
				UPDATE threads
				SET active_plan_configuration_id = $1, updated_at = now()
				WHERE tenant_id = $2 AND id = $3
			`, created.ID, created.TenantID, created.ThreadID); err != nil {
				return fmt.Errorf("attach plan configuration to thread: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *Repository) GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*PlanConfiguration, error) {
	var config PlanConfiguration
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
			        seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
			        parameter_values, thread_id, created_at, updated_at
			 FROM plan_configurations
			 WHERE id = $1 AND tenant_id = $2`,
			configID, tenantID,
		)
		return scanConfiguration(row, &config)
	})
	if err != nil {
		return nil, fmt.Errorf("get plan configuration: %w", err)
	}
	return &config, nil
}

func (r *Repository) UpdateConfiguration(ctx context.Context, config *PlanConfiguration) (*PlanConfiguration, error) {
	var updated PlanConfiguration
	err := database.WithTenant(ctx, r.pool, config.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`UPDATE plan_configurations
			 SET status = $1,
			     seed_artifacts = $2,
			     slot_bindings = $3,
			     overseer_bindings = $4,
			     behavior_policies = $5,
			     schedule = $6,
			     parameter_values = $7,
			     updated_at = now()
			 WHERE id = $8 AND tenant_id = $9
			 RETURNING id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
			           seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
			           parameter_values, thread_id, created_at, updated_at`,
			config.Status, config.SeedArtifacts, config.SlotBindings, config.OverseerBindings,
			config.BehaviorPolicies, config.Schedule, config.ParameterValues, config.ID, config.TenantID,
		)
		return scanConfiguration(row, &updated)
	})
	if err != nil {
		return nil, fmt.Errorf("update plan configuration: %w", err)
	}
	return &updated, nil
}

func (r *Repository) UpdateConfigurationStatus(ctx context.Context, tenantID, configID uuid.UUID, status plansv1.PlanConfigurationStatus) error {
	statusStr, err := configurationStatusToString(status)
	if err != nil {
		return err
	}
	return database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		tag, execErr := q.Exec(ctx,
			`UPDATE plan_configurations SET status = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
			statusStr, configID, tenantID,
		)
		if execErr != nil {
			return fmt.Errorf("update plan configuration status: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("plan configuration not found")
		}
		return nil
	})
}

func (r *Repository) ListConfigurations(ctx context.Context, tenantID uuid.UUID, workspaceID *uuid.UUID, status string, limit, offset int) ([]PlanConfiguration, error) {
	configs := make([]PlanConfiguration, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		var rows pgx.Rows
		var err error
		switch {
		case workspaceID != nil && status != "":
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
				        seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
				        parameter_values, thread_id, created_at, updated_at
				 FROM plan_configurations
				 WHERE tenant_id = $1 AND workspace_id = $2 AND status = $3
				 ORDER BY created_at DESC
				 LIMIT $4 OFFSET $5`,
				tenantID, *workspaceID, status, limit, offset,
			)
		case workspaceID != nil:
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
				        seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
				        parameter_values, thread_id, created_at, updated_at
				 FROM plan_configurations
				 WHERE tenant_id = $1 AND workspace_id = $2
				 ORDER BY created_at DESC
				 LIMIT $3 OFFSET $4`,
				tenantID, *workspaceID, limit, offset,
			)
		case status != "":
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
				        seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
				        parameter_values, thread_id, created_at, updated_at
				 FROM plan_configurations
				 WHERE tenant_id = $1 AND status = $2
				 ORDER BY created_at DESC
				 LIMIT $3 OFFSET $4`,
				tenantID, status, limit, offset,
			)
		default:
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
				        seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
				        parameter_values, thread_id, created_at, updated_at
				 FROM plan_configurations
				 WHERE tenant_id = $1
				 ORDER BY created_at DESC
				 LIMIT $2 OFFSET $3`,
				tenantID, limit, offset,
			)
		}
		if err != nil {
			return fmt.Errorf("list plan configurations: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var config PlanConfiguration
			if err := rows.Scan(
				&config.ID, &config.TenantID, &config.WorkspaceID, &config.PlanTemplateID,
				&config.PlanTemplateVersion, &config.Status, &config.SeedArtifacts,
				&config.SlotBindings, &config.OverseerBindings, &config.BehaviorPolicies,
				&config.Schedule, &config.ParameterValues, &config.ThreadID, &config.CreatedAt, &config.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan plan configuration: %w", err)
			}
			configs = append(configs, config)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *Repository) CreateExecution(ctx context.Context, execution *PlanExecution) (*PlanExecution, error) {
	var created PlanExecution
	err := database.WithTenant(ctx, r.pool, execution.TenantID, func(q database.Querier) error {
		if execution.ID != uuid.Nil {
			row := q.QueryRow(ctx,
				`INSERT INTO plan_executions (
					id, tenant_id, plan_configuration_id, plan_configuration_snapshot, status, triggered_at
				) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO UPDATE SET id = plan_executions.id
				RETURNING id, tenant_id, plan_configuration_id, plan_configuration_snapshot, status,
				          triggered_at, completed_at, created_at, updated_at`,
				execution.ID, execution.TenantID, execution.PlanConfigurationID, execution.PlanConfigurationSnapshot,
				execution.Status, execution.TriggeredAt,
			)
			return scanExecution(row, &created)
		}

		row := q.QueryRow(ctx,
			`INSERT INTO plan_executions (
				tenant_id, plan_configuration_id, plan_configuration_snapshot, status, triggered_at
			) VALUES ($1, $2, $3, $4, $5)
			RETURNING id, tenant_id, plan_configuration_id, plan_configuration_snapshot, status,
			          triggered_at, completed_at, created_at, updated_at`,
			execution.TenantID, execution.PlanConfigurationID, execution.PlanConfigurationSnapshot,
			execution.Status, execution.TriggeredAt,
		)
		return scanExecution(row, &created)
	})
	if err != nil {
		return nil, fmt.Errorf("create plan execution: %w", err)
	}
	return &created, nil
}

func (r *Repository) UpdateExecutionStatus(ctx context.Context, tenantID, executionID uuid.UUID, status string, completedAt *time.Time) error {
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		var currentStatus string
		err := q.QueryRow(ctx,
			`SELECT status
			   FROM plan_executions
			  WHERE id = $1 AND tenant_id = $2
			  FOR UPDATE`,
			executionID, tenantID,
		).Scan(&currentStatus)
		if err != nil {
			return fmt.Errorf("load plan execution status: %w", err)
		}

		switch status {
		case ExecutionStatusRunning:
			if currentStatus == ExecutionStatusRunning {
				return nil
			}
			if currentStatus != ExecutionStatusPending {
				return fmt.Errorf("cannot move plan execution from %q to %q", currentStatus, status)
			}
		case ExecutionStatusCompleted:
			if currentStatus == ExecutionStatusCompleted {
				return nil
			}
			if currentStatus != ExecutionStatusRunning {
				return fmt.Errorf("cannot move plan execution from %q to %q", currentStatus, status)
			}
		case ExecutionStatusFailed:
			if currentStatus == ExecutionStatusFailed {
				return nil
			}
			if currentStatus == ExecutionStatusCompleted || currentStatus == ExecutionStatusCancelled {
				return fmt.Errorf("cannot move plan execution from %q to %q", currentStatus, status)
			}
		default:
			return fmt.Errorf("unsupported plan execution status %q", status)
		}

		tag, err := q.Exec(ctx,
			`UPDATE plan_executions
			 SET status = $1,
			     completed_at = $2,
			     updated_at = now()
			 WHERE id = $3 AND tenant_id = $4`,
			status, completedAt, executionID, tenantID,
		)
		if err != nil {
			return fmt.Errorf("update plan execution status: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update plan execution status: %w", err)
	}
	return nil
}

func (r *Repository) GetExecution(ctx context.Context, tenantID, executionID uuid.UUID) (*PlanExecution, error) {
	var execution PlanExecution
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, plan_configuration_id, plan_configuration_snapshot, status,
			        triggered_at, completed_at, created_at, updated_at
			 FROM plan_executions
			 WHERE id = $1 AND tenant_id = $2`,
			executionID, tenantID,
		)
		if err := scanExecution(row, &execution); err != nil {
			return err
		}
		steps, err := r.listStepExecutions(ctx, q, tenantID, executionID, 1000, 0)
		if err != nil {
			return err
		}
		execution.StepExecutions = steps
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get plan execution: %w", err)
	}
	return &execution, nil
}

// GetThreadIDForConfiguration resolves a plan_configuration_id to its owning
// thread_id. Used by the Path B compatibility bridge so legacy plan-thread RPCs
// persist to threads.id.
func (r *Repository) GetThreadIDForConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (uuid.UUID, error) {
	var threadID uuid.UUID
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		return q.QueryRow(ctx, `
			SELECT thread_id
			FROM plan_configurations
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, configID).Scan(&threadID)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("get thread id for plan configuration: %w", err)
	}
	return threadID, nil
}

// GetPlanConfigurationIDForExecution returns the plan_configuration_id for a given plan_execution_id.
func (r *Repository) GetPlanConfigurationIDForExecution(ctx context.Context, tenantID, executionID uuid.UUID) (uuid.UUID, error) {
	var configID uuid.UUID
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		return q.QueryRow(ctx, `
			SELECT plan_configuration_id FROM plan_executions
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, executionID).Scan(&configID)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("plans: GetPlanConfigurationIDForExecution: %w", err)
	}
	return configID, nil
}

func (r *Repository) ListExecutions(ctx context.Context, tenantID uuid.UUID, configID *uuid.UUID, limit, offset int) ([]PlanExecution, error) {
	executions := make([]PlanExecution, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		var rows pgx.Rows
		var err error
		if configID != nil {
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, plan_configuration_id, plan_configuration_snapshot, status,
				        triggered_at, completed_at, created_at, updated_at
				 FROM plan_executions
				 WHERE tenant_id = $1 AND plan_configuration_id = $2
				 ORDER BY created_at DESC
				 LIMIT $3 OFFSET $4`,
				tenantID, *configID, limit, offset,
			)
		} else {
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, plan_configuration_id, plan_configuration_snapshot, status,
				        triggered_at, completed_at, created_at, updated_at
				 FROM plan_executions
				 WHERE tenant_id = $1
				 ORDER BY created_at DESC
				 LIMIT $2 OFFSET $3`,
				tenantID, limit, offset,
			)
		}
		if err != nil {
			return fmt.Errorf("list plan executions: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var execution PlanExecution
			if err := rows.Scan(
				&execution.ID, &execution.TenantID, &execution.PlanConfigurationID,
				&execution.PlanConfigurationSnapshot, &execution.Status,
				&execution.TriggeredAt, &execution.CompletedAt,
				&execution.CreatedAt, &execution.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan plan execution: %w", err)
			}
			executions = append(executions, execution)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return executions, nil
}

func (r *Repository) CreateStepExecution(ctx context.Context, step *StepExecution) (*StepExecution, error) {
	var created StepExecution
	err := database.WithTenant(ctx, r.pool, step.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`WITH existing AS (
				SELECT id, tenant_id, plan_execution_id, plan_step_key, status,
				       COALESCE(input_artifact_id, '') AS input_artifact_id,
				       COALESCE(output_artifact_id, '') AS output_artifact_id,
				       executor_installation_snapshot, attempt,
				       COALESCE(elicitation_thread_id, '') AS elicitation_thread_id,
				       COALESCE(approval_request_id, '') AS approval_request_id,
				       created_at, updated_at
				  FROM step_executions
				 WHERE tenant_id = $1
				   AND plan_execution_id = $2
				   AND plan_step_key = $3
				   AND status IN ('running', 'awaiting_elicitation', 'awaiting_approval')
				 ORDER BY attempt DESC
				 LIMIT 1
			), inserted AS (
				INSERT INTO step_executions (
					tenant_id, plan_execution_id, plan_step_key, status, input_artifact_id,
					executor_installation_snapshot, attempt
				)
				SELECT $1, $2, $3, $4, NULLIF($5, ''), $6,
				       COALESCE((
					       SELECT MAX(prior.attempt) + 1
					       FROM step_executions prior
					       WHERE prior.tenant_id = $1
					         AND prior.plan_execution_id = $2
					         AND prior.plan_step_key = $3
				       ), 1)
				WHERE NOT EXISTS (SELECT 1 FROM existing)
				RETURNING id, tenant_id, plan_execution_id, plan_step_key, status,
				          COALESCE(input_artifact_id, ''), COALESCE(output_artifact_id, ''),
				          executor_installation_snapshot, attempt,
				          COALESCE(elicitation_thread_id, ''), COALESCE(approval_request_id, ''),
				          created_at, updated_at
			)
			SELECT * FROM inserted
			UNION ALL
			SELECT * FROM existing
			LIMIT 1`,
			step.TenantID, step.PlanExecutionID, step.PlanStepKey, step.Status,
			step.InputArtifactID, step.ExecutorInstallationSnapshot,
		)
		return scanStepExecution(row, &created)
	})
	if err != nil {
		return nil, fmt.Errorf("create step execution: %w", err)
	}
	return &created, nil
}

func (r *Repository) UpdateStepExecutionStatus(
	ctx context.Context,
	tenantID uuid.UUID,
	stepExecutionID uuid.UUID,
	status string,
	outputArtifactID string,
	elicitationThreadID string,
	approvalRequestID string,
) error {
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		tag, err := q.Exec(ctx,
			`UPDATE step_executions
			 SET status = $1,
			     output_artifact_id = COALESCE(NULLIF($2, ''), output_artifact_id),
			     elicitation_thread_id = COALESCE(NULLIF($3, ''), elicitation_thread_id),
			     approval_request_id = COALESCE(NULLIF($4, ''), approval_request_id),
			     updated_at = now()
			 WHERE id = $5 AND tenant_id = $6`,
			status, outputArtifactID, elicitationThreadID, approvalRequestID,
			stepExecutionID, tenantID,
		)
		if err != nil {
			return fmt.Errorf("update step execution status: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update step execution status: %w", err)
	}
	return nil
}

func (r *Repository) CreatePlanApprovalRequest(ctx context.Context, request *PlanApprovalRequest) error {
	if request == nil {
		return fmt.Errorf("plan approval request is required")
	}
	err := database.WithTenant(ctx, r.pool, request.TenantID, func(q database.Querier) error {
		_, err := q.Exec(ctx,
			`INSERT INTO plan_approval_requests (
				id, tenant_id, plan_execution_id, step_execution_id, plan_step_key,
				input_artifact_id, status
			)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
			ON CONFLICT (id) DO UPDATE
			SET updated_at = plan_approval_requests.updated_at`,
			request.ID, request.TenantID, request.PlanExecutionID,
			request.StepExecutionID, request.PlanStepKey, request.InputArtifactID,
			ApprovalRequestStatusPending,
		)
		if err != nil {
			return fmt.Errorf("create plan approval request: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("create plan approval request: %w", err)
	}
	return nil
}

func (r *Repository) ResolvePlanApprovalRequest(
	ctx context.Context,
	tenantID uuid.UUID,
	requestID string,
	stepExecutionID uuid.UUID,
	approved bool,
	reason string,
) error {
	status := ApprovalRequestStatusRejected
	if approved {
		status = ApprovalRequestStatusApproved
	}
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		tag, err := q.Exec(ctx,
			`UPDATE plan_approval_requests
			 SET status = $1,
			     decision_reason = $2,
			     decided_at = now(),
			     updated_at = now()
			 WHERE id = $3 AND tenant_id = $4 AND step_execution_id = $5`,
			status, reason, requestID, tenantID, stepExecutionID,
		)
		if err != nil {
			return fmt.Errorf("resolve plan approval request: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("resolve plan approval request: %w", err)
	}
	return nil
}

const planApprovalRequestColumns = `id, tenant_id, plan_execution_id, step_execution_id, plan_step_key,
	COALESCE(input_artifact_id, ''), status, COALESCE(decision_reason, ''),
	requested_at, decided_at, created_at, updated_at`

const planApprovalRequestJoinedColumns = `par.id, par.tenant_id, par.plan_execution_id, par.step_execution_id, par.plan_step_key,
	COALESCE(par.input_artifact_id, ''), par.status, COALESCE(par.decision_reason, ''),
	par.requested_at, par.decided_at, par.created_at, par.updated_at, pe.plan_configuration_id`

const planApprovalRequestFromJoin = ` FROM plan_approval_requests par
	JOIN plan_executions pe ON pe.id = par.plan_execution_id AND pe.tenant_id = par.tenant_id`

func scanPlanApprovalRequest(row pgx.Row, dest *PlanApprovalRequest) error {
	return row.Scan(
		&dest.ID, &dest.TenantID, &dest.PlanExecutionID, &dest.StepExecutionID,
		&dest.PlanStepKey, &dest.InputArtifactID, &dest.Status, &dest.DecisionReason,
		&dest.RequestedAt, &dest.DecidedAt, &dest.CreatedAt, &dest.UpdatedAt,
	)
}

func scanPlanApprovalRequestJoined(row pgx.Row, dest *PlanApprovalRequest) error {
	return row.Scan(
		&dest.ID, &dest.TenantID, &dest.PlanExecutionID, &dest.StepExecutionID,
		&dest.PlanStepKey, &dest.InputArtifactID, &dest.Status, &dest.DecisionReason,
		&dest.RequestedAt, &dest.DecidedAt, &dest.CreatedAt, &dest.UpdatedAt,
		&dest.PlanConfigurationID,
	)
}

func (r *Repository) GetPlanApprovalRequest(ctx context.Context, tenantID uuid.UUID, approvalID string) (*PlanApprovalRequest, error) {
	var request PlanApprovalRequest
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT `+planApprovalRequestJoinedColumns+planApprovalRequestFromJoin+`
			 WHERE par.id = $1 AND par.tenant_id = $2`,
			approvalID, tenantID,
		)
		return scanPlanApprovalRequestJoined(row, &request)
	})
	if err != nil {
		return nil, fmt.Errorf("get plan approval request: %w", err)
	}
	return &request, nil
}

func (r *Repository) ListPlanApprovalRequests(ctx context.Context, tenantID uuid.UUID, filters ApprovalFilters) ([]*PlanApprovalRequest, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	requests := make([]*PlanApprovalRequest, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		conditions := []string{"par.tenant_id = $1"}
		args := []any{tenantID}
		if filters.StepExecutionID != nil {
			args = append(args, *filters.StepExecutionID)
			conditions = append(conditions, fmt.Sprintf("par.step_execution_id = $%d::uuid", len(args)))
		}
		if filters.PlanExecutionID != nil {
			args = append(args, *filters.PlanExecutionID)
			conditions = append(conditions, fmt.Sprintf("par.plan_execution_id = $%d::uuid", len(args)))
		}
		if strings.TrimSpace(filters.Status) != "" {
			args = append(args, filters.Status)
			conditions = append(conditions, fmt.Sprintf("par.status = $%d", len(args)))
		}
		args = append(args, limit)
		limitPlaceholder := len(args)
		args = append(args, filters.Offset)
		offsetPlaceholder := len(args)

		query := `SELECT ` + planApprovalRequestJoinedColumns + planApprovalRequestFromJoin + `
			 WHERE ` + strings.Join(conditions, " AND ") + `
			 ORDER BY par.requested_at DESC
			 LIMIT $` + fmt.Sprint(limitPlaceholder) + ` OFFSET $` + fmt.Sprint(offsetPlaceholder)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list plan approval requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var request PlanApprovalRequest
			if err := scanPlanApprovalRequestJoined(rows, &request); err != nil {
				return err
			}
			row := request
			requests = append(requests, &row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list plan approval requests: %w", err)
	}
	return requests, nil
}

func (r *Repository) MarkApprovalDecided(
	ctx context.Context,
	tenantID uuid.UUID,
	approvalID string,
	approved bool,
	reason string,
) (*PlanApprovalRequest, error) {
	status := ApprovalRequestStatusRejected
	if approved {
		status = ApprovalRequestStatusApproved
	}
	var updated PlanApprovalRequest
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`UPDATE plan_approval_requests
			 SET status = $1,
			     decision_reason = $2,
			     decided_at = now(),
			     updated_at = now()
			 WHERE id = $3 AND tenant_id = $4 AND status = $5
			 RETURNING `+planApprovalRequestColumns,
			status, reason, approvalID, tenantID, ApprovalRequestStatusPending,
		)
		return scanPlanApprovalRequest(row, &updated)
	})
	if err != nil {
		return nil, fmt.Errorf("mark approval decided: %w", err)
	}
	return &updated, nil
}

func (r *Repository) GetStepExecution(ctx context.Context, tenantID, stepExecutionID uuid.UUID) (*StepExecution, error) {
	var step StepExecution
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, plan_execution_id, plan_step_key, status,
			        COALESCE(input_artifact_id, ''), COALESCE(output_artifact_id, ''),
			        executor_installation_snapshot, attempt,
			        COALESCE(elicitation_thread_id, ''), COALESCE(approval_request_id, ''),
			        created_at, updated_at
			 FROM step_executions
			 WHERE id = $1 AND tenant_id = $2`,
			stepExecutionID, tenantID,
		)
		return scanStepExecution(row, &step)
	})
	if err != nil {
		return nil, fmt.Errorf("get step execution: %w", err)
	}
	return &step, nil
}

func (r *Repository) ListStepExecutions(ctx context.Context, tenantID, executionID uuid.UUID, limit, offset int) ([]StepExecution, error) {
	steps := make([]StepExecution, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		var err error
		steps, err = r.listStepExecutions(ctx, q, tenantID, executionID, limit, offset)
		return err
	})
	if err != nil {
		return nil, err
	}
	return steps, nil
}

func (r *Repository) listStepExecutions(ctx context.Context, q database.Querier, tenantID, executionID uuid.UUID, limit, offset int) ([]StepExecution, error) {
	rows, err := q.Query(ctx,
		`SELECT id, tenant_id, plan_execution_id, plan_step_key, status,
		        COALESCE(input_artifact_id, ''), COALESCE(output_artifact_id, ''),
		        executor_installation_snapshot, attempt,
		        COALESCE(elicitation_thread_id, ''), COALESCE(approval_request_id, ''),
		        created_at, updated_at
		 FROM step_executions
		 WHERE tenant_id = $1 AND plan_execution_id = $2
		 ORDER BY created_at ASC
		 LIMIT $3 OFFSET $4`,
		tenantID, executionID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list step executions: %w", err)
	}
	defer rows.Close()

	steps := make([]StepExecution, 0)
	for rows.Next() {
		var step StepExecution
		if err := scanStepExecution(rows, &step); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func scanConfiguration(row pgx.Row, config *PlanConfiguration) error {
	return row.Scan(
		&config.ID, &config.TenantID, &config.WorkspaceID, &config.PlanTemplateID,
		&config.PlanTemplateVersion, &config.Status, &config.SeedArtifacts,
		&config.SlotBindings, &config.OverseerBindings, &config.BehaviorPolicies,
		&config.Schedule, &config.ParameterValues, &config.ThreadID, &config.CreatedAt, &config.UpdatedAt,
	)
}

func scanExecution(row pgx.Row, execution *PlanExecution) error {
	return row.Scan(
		&execution.ID, &execution.TenantID, &execution.PlanConfigurationID,
		&execution.PlanConfigurationSnapshot, &execution.Status,
		&execution.TriggeredAt, &execution.CompletedAt,
		&execution.CreatedAt, &execution.UpdatedAt,
	)
}

func scanStepExecution(row pgx.Row, step *StepExecution) error {
	return row.Scan(
		&step.ID, &step.TenantID, &step.PlanExecutionID, &step.PlanStepKey, &step.Status,
		&step.InputArtifactID, &step.OutputArtifactID, &step.ExecutorInstallationSnapshot,
		&step.Attempt, &step.ElicitationThreadID, &step.ApprovalRequestID,
		&step.CreatedAt, &step.UpdatedAt,
	)
}
