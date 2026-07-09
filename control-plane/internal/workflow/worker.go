package workflow

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func StartWorker(ctx context.Context, temporalClient client.Client, taskQueue string, planActivities *PlanActivities) error {
	w := worker.New(temporalClient, taskQueue, worker.Options{})

	w.RegisterWorkflow(PlanWorkflow)
	w.RegisterWorkflow(PlanScheduledExecution)
	if planActivities != nil {
		w.RegisterActivity(planActivities.CreateScheduledPlanExecutionActivity)
		w.RegisterActivity(planActivities.LoadPlanExecutionActivity)
		w.RegisterActivity(planActivities.StartPlanExecutionActivity)
		w.RegisterActivity(planActivities.CreateStepExecutionActivity)
		w.RegisterActivity(planActivities.CreateSkippedStepExecutionActivity)
		w.RegisterActivity(planActivities.RunIntegrationActivity)
		// RunAgentActivity is owned by the Python agent-runtime worker. If the
		// Go worker registers it too, Temporal may dispatch agent work to the
		// old Go implementation and fail real plan executions.
		w.RegisterActivity(planActivities.ResumeStepExecutionActivity)
		w.RegisterActivity(planActivities.CompleteStepExecutionActivity)
		w.RegisterActivity(planActivities.FailStepExecutionActivity)
		w.RegisterActivity(planActivities.AwaitElicitationStepExecutionActivity)
		w.RegisterActivity(planActivities.TimeoutElicitationStepExecutionActivity)
		w.RegisterActivity(planActivities.CreateApprovalRequestActivity)
		w.RegisterActivity(planActivities.ResolveApprovalRequestActivity)
		w.RegisterActivity(planActivities.CompletePlanExecutionActivity)
		w.RegisterActivity(planActivities.FailPlanExecutionActivity)
	}

	slog.Info("starting temporal worker", "task_queue", taskQueue)

	if err := w.Run(worker.InterruptCh()); err != nil {
		return fmt.Errorf("temporal worker: %w", err)
	}

	return nil
}
