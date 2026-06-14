package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

const PlanScheduledExecutionWorkflowName = "PlanScheduledExecution"

type PlanExecutionInput struct {
	TenantID            string `json:"tenant_id"`
	PlanConfigurationID string `json:"plan_configuration_id"`
}

type ScheduledExecutionCreator interface {
	CreateScheduledExecution(ctx context.Context, tenantID, configID uuid.UUID) error
}

type PlanActivities struct {
	Creator ScheduledExecutionCreator
}

func (a *PlanActivities) CreateScheduledPlanExecutionActivity(ctx context.Context, input PlanExecutionInput) error {
	if a == nil || a.Creator == nil {
		return fmt.Errorf("scheduled plan execution creator is not configured")
	}

	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return fmt.Errorf("parse tenant id: %w", err)
	}
	configID, err := uuid.Parse(input.PlanConfigurationID)
	if err != nil {
		return fmt.Errorf("parse plan configuration id: %w", err)
	}

	return a.Creator.CreateScheduledExecution(ctx, tenantID, configID)
}

func PlanScheduledExecution(ctx workflow.Context, input PlanExecutionInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting scheduled plan execution",
		"tenant_id", input.TenantID,
		"plan_configuration_id", input.PlanConfigurationID,
	)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	return workflow.ExecuteActivity(ctx, CreateScheduledPlanExecutionActivityName, input).Get(ctx, nil)
}

const CreateScheduledPlanExecutionActivityName = "CreateScheduledPlanExecutionActivity"
