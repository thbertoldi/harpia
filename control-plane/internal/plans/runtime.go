package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/workflow"
)

type RuntimeRepository struct {
	plans     *Repository
	executors ExecutorLookup
}

func NewRuntimeRepository(planRepo *Repository, executors ExecutorLookup) *RuntimeRepository {
	return &RuntimeRepository{
		plans:     planRepo,
		executors: executors,
	}
}

func (r *RuntimeRepository) CreateScheduledExecution(ctx context.Context, tenantID, configID, executionID uuid.UUID) (workflow.PlanWorkflowInput, error) {
	if r == nil || r.plans == nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("plan runtime repository is not configured")
	}

	config, err := r.plans.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("get plan configuration: %w", err)
	}
	if config.Status != ConfigurationStatusScheduled {
		return workflow.PlanWorkflowInput{}, nil
	}

	template, err := r.plans.GetTemplateByID(ctx, config.PlanTemplateID)
	if err != nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("get plan template: %w", err)
	}
	if err := NewBindingValidator(r.executors).ValidateConfigurationForExecution(ctx, tenantID, template, config); err != nil {
		return workflow.PlanWorkflowInput{}, err
	}
	if executionID == uuid.Nil {
		executionID = uuid.New()
	}

	snapshot, err := buildPlanExecutionSnapshot(ctx, tenantID, config, template, r.executors, time.Now().UTC())
	if err != nil {
		return workflow.PlanWorkflowInput{}, err
	}
	rawSnapshot, err := marshalPlanExecutionSnapshot(snapshot)
	if err != nil {
		return workflow.PlanWorkflowInput{}, err
	}

	now := time.Now().UTC()
	execution, err := r.plans.CreateExecution(ctx, &PlanExecution{
		ID:                        executionID,
		TenantID:                  tenantID,
		PlanConfigurationID:       configID,
		PlanConfigurationSnapshot: rawSnapshot,
		Status:                    ExecutionStatusPending,
		TriggeredAt:               &now,
	})
	if err != nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("create scheduled plan execution: %w", err)
	}

	return workflow.PlanWorkflowInput{
		TenantID:        tenantID.String(),
		PlanExecutionID: execution.ID.String(),
	}, nil
}

func (r *RuntimeRepository) LoadPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) (workflow.LoadedPlanExecution, error) {
	if r == nil || r.plans == nil {
		return workflow.LoadedPlanExecution{}, fmt.Errorf("plan runtime repository is not configured")
	}

	execution, err := r.plans.GetExecution(ctx, tenantID, executionID)
	if err != nil {
		return workflow.LoadedPlanExecution{}, err
	}
	snapshot, err := unmarshalPlanExecutionSnapshot(execution.PlanConfigurationSnapshot)
	if err != nil {
		return workflow.LoadedPlanExecution{}, err
	}
	return workflow.LoadedPlanExecution{
		PlanExecutionID: execution.ID.String(),
		Status:          execution.Status,
		Snapshot:        snapshot,
	}, nil
}

func (r *RuntimeRepository) StartPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	return r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusRunning, nil)
}

func (r *RuntimeRepository) CreateStepExecution(ctx context.Context, input workflow.CreateStepExecutionInput) (workflow.StepExecutionRecord, error) {
	tenantID, executionID, err := parseRuntimeTenantExecution(input.TenantID, input.PlanExecutionID)
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}
	rawInstallation, err := json.Marshal(input.ExecutorInstallationSnapshot)
	if err != nil {
		return workflow.StepExecutionRecord{}, fmt.Errorf("marshal executor installation snapshot: %w", err)
	}

	step, err := r.plans.CreateStepExecution(ctx, &StepExecution{
		TenantID:                     tenantID,
		PlanExecutionID:              executionID,
		PlanStepKey:                  input.PlanStepKey,
		Status:                       StepStatusRunning,
		InputArtifactID:              input.InputArtifactID,
		ExecutorInstallationSnapshot: rawInstallation,
	})
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}

	return workflow.StepExecutionRecord{
		ID:              step.ID.String(),
		PlanStepKey:     step.PlanStepKey,
		Attempt:         step.Attempt,
		InputArtifactID: step.InputArtifactID,
	}, nil
}

func (r *RuntimeRepository) ResumeStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusRunning, "", "", "")
}

func (r *RuntimeRepository) CompleteStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusCompleted, input.OutputArtifactID, "", "")
}

func (r *RuntimeRepository) FailStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusFailed, "", "", "")
}

func (r *RuntimeRepository) AwaitElicitationStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusAwaitingElicitation, "", input.ElicitationThreadID, "")
}

func (r *RuntimeRepository) CreateApprovalRequest(ctx context.Context, input workflow.CreateApprovalRequestInput) error {
	tenantID, executionID, err := parseRuntimeTenantExecution(input.TenantID, input.PlanExecutionID)
	if err != nil {
		return err
	}
	stepID, err := uuid.Parse(input.StepExecutionID)
	if err != nil {
		return fmt.Errorf("parse step execution id: %w", err)
	}
	if err := r.plans.CreatePlanApprovalRequest(ctx, &PlanApprovalRequest{
		ID:              input.ApprovalRequestID,
		TenantID:        tenantID,
		PlanExecutionID: executionID,
		StepExecutionID: stepID,
		PlanStepKey:     input.PlanStepKey,
		InputArtifactID: input.InputArtifactID,
		Status:          ApprovalRequestStatusPending,
	}); err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusAwaitingApproval, "", "", input.ApprovalRequestID)
}

func (r *RuntimeRepository) ResolveApprovalRequest(ctx context.Context, input workflow.ResolveApprovalRequestInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.ResolvePlanApprovalRequest(ctx, tenantID, input.ApprovalRequestID, stepID, input.Approved, input.Reason)
}

func (r *RuntimeRepository) CompletePlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	now := time.Now().UTC()
	return r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusCompleted, &now)
}

func (r *RuntimeRepository) FailPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	now := time.Now().UTC()
	return r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusFailed, &now)
}

func parseRuntimeTenantExecution(tenantIDRaw, executionIDRaw string) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(tenantIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse tenant id: %w", err)
	}
	executionID, err := uuid.Parse(executionIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse plan execution id: %w", err)
	}
	return tenantID, executionID, nil
}

func parseRuntimeTenantStep(tenantIDRaw, stepIDRaw string) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(tenantIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse tenant id: %w", err)
	}
	stepID, err := uuid.Parse(stepIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse step execution id: %w", err)
	}
	return tenantID, stepID, nil
}
