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

	w.RegisterWorkflow(TaskOrchestration)
	w.RegisterActivity(DecomposeTaskActivity)
	w.RegisterActivity(ExecuteSubtaskActivity)

	w.RegisterWorkflow(PlanScheduledExecution)
	if planActivities != nil {
		w.RegisterActivity(planActivities.CreateScheduledPlanExecutionActivity)
	}

	slog.Info("starting temporal worker", "task_queue", taskQueue)

	if err := w.Run(worker.InterruptCh()); err != nil {
		return fmt.Errorf("temporal worker: %w", err)
	}

	return nil
}
