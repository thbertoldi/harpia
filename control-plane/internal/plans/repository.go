package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

type PlanTemplate struct {
	ID          uuid.UUID
	Key         string
	Name        string
	Description string
	Vertical    string
	Version     int32
	Steps       []PlanStep
	Edges       []PlanStepDependency
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlanStep struct {
	ID                     uuid.UUID
	Key                    string
	Title                  string
	Description            string
	InputArtifactTypeID    string
	OutputArtifactTypeID   string
	ExecutorRequirement    json.RawMessage
	DefaultExecutorSKUKey  string
	Position               int32
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

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetTemplateByID(ctx context.Context, templateID uuid.UUID) (*PlanTemplate, error) {
	var template PlanTemplate
	err := r.pool.QueryRow(ctx,
		`SELECT id, key, name, description, vertical, version, created_at, updated_at
		 FROM plan_templates WHERE id = $1`,
		templateID,
	).Scan(
		&template.ID, &template.Key, &template.Name, &template.Description,
		&template.Vertical, &template.Version, &template.CreatedAt, &template.UpdatedAt,
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
		`SELECT id, key, name, description, vertical, version, created_at, updated_at
		 FROM plan_templates WHERE key = $1`,
		key,
	).Scan(
		&template.ID, &template.Key, &template.Name, &template.Description,
		&template.Vertical, &template.Version, &template.CreatedAt, &template.UpdatedAt,
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
			`SELECT id, key, name, description, vertical, version, created_at, updated_at
			 FROM plan_templates
			 WHERE vertical = $1
			 ORDER BY name ASC
			 LIMIT $2 OFFSET $3`,
			vertical, limit, offset,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, key, name, description, vertical, version, created_at, updated_at
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
			&template.Vertical, &template.Version, &template.CreatedAt, &template.UpdatedAt,
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
				seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
			          seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
			          created_at, updated_at`,
			config.TenantID, config.WorkspaceID, config.PlanTemplateID, config.PlanTemplateVersion,
			config.Status, config.SeedArtifacts, config.SlotBindings, config.OverseerBindings,
			config.BehaviorPolicies, config.Schedule,
		)
		return scanConfiguration(row, &created)
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
			        created_at, updated_at
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
			     updated_at = now()
			 WHERE id = $7 AND tenant_id = $8
			 RETURNING id, tenant_id, workspace_id, plan_template_id, plan_template_version, status,
			           seed_artifacts, slot_bindings, overseer_bindings, behavior_policies, schedule,
			           created_at, updated_at`,
			config.Status, config.SeedArtifacts, config.SlotBindings, config.OverseerBindings,
			config.BehaviorPolicies, config.Schedule, config.ID, config.TenantID,
		)
		return scanConfiguration(row, &updated)
	})
	if err != nil {
		return nil, fmt.Errorf("update plan configuration: %w", err)
	}
	return &updated, nil
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
				        created_at, updated_at
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
				        created_at, updated_at
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
				        created_at, updated_at
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
				        created_at, updated_at
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
				&config.Schedule, &config.CreatedAt, &config.UpdatedAt,
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
		&config.Schedule, &config.CreatedAt, &config.UpdatedAt,
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
