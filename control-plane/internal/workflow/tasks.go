package workflow

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

type TaskInput struct {
	TaskID      string `json:"task_id"`
	TenantID    string `json:"tenant_id"`
	Description string `json:"description"`
}

type TaskResult struct {
	Status string `json:"status"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

type Subtask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	AgentType   string `json:"agent_type"`
}

type SubtaskResult struct {
	SubtaskID string `json:"subtask_id"`
	Status    string `json:"status"`
	Output    string `json:"output"`
	Error     string `json:"error"`
}

type HumanFeedbackSignal struct {
	TaskID   string `json:"task_id"`
	Feedback string `json:"feedback"`
	Approved bool   `json:"approved"`
}

const (
	TaskQueueName = "harpia-task-queue"
	// AgentTaskQueueName is polled only by the Python agent-runtime worker,
	// which owns RunAgentActivity. The Go worker serves the plan workflow plus
	// integration + lifecycle activities on TaskQueueName. Agent steps are
	// routed here so cross-language activities don't land on the wrong worker.
	AgentTaskQueueName         = "harpia-agent-task-queue"
	DecomposeTaskActivityName  = "DecomposeTaskActivity"
	ExecuteSubtaskActivityName = "ExecuteSubtaskActivity"
	HumanFeedbackSignalName    = "human-feedback-signal"

	TaskResultStatusFailed    = "failed"
	TaskResultStatusCompleted = "completed"
	TaskResultStatusCancelled = "cancelled"
	SubtaskResultStatusFailed = "failed"
)

func TaskOrchestration(ctx workflow.Context, input TaskInput) (TaskResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting task orchestration", "task_id", input.TaskID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var subtasks []Subtask
	err := workflow.ExecuteActivity(ctx, DecomposeTaskActivity, input).Get(ctx, &subtasks)
	if err != nil {
		return TaskResult{Status: TaskResultStatusFailed, Error: fmt.Sprintf("decompose: %v", err)}, nil
	}

	var finalOutput string
	for _, subtask := range subtasks {
		var result SubtaskResult
		err := workflow.ExecuteActivity(ctx, ExecuteSubtaskActivity, subtask).Get(ctx, &result)
		if err != nil {
			return TaskResult{Status: TaskResultStatusFailed, Error: fmt.Sprintf("subtask %s: %v", subtask.ID, err)}, nil
		}

		if result.Status == "needs_approval" {
			signalChan := workflow.GetSignalChannel(ctx, HumanFeedbackSignalName)
			var signal HumanFeedbackSignal
			var received bool

			selector := workflow.NewSelector(ctx)
			selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
				c.Receive(ctx, &signal)
				received = true
			})
			selector.AddFuture(workflow.NewTimer(ctx, 24*time.Hour), func(f workflow.Future) {
			})
			selector.Select(ctx)

			if !received {
				return TaskResult{Status: TaskResultStatusCancelled, Error: "timed out waiting for human feedback"}, nil
			}
			if !signal.Approved {
				return TaskResult{Status: TaskResultStatusCancelled, Output: signal.Feedback}, nil
			}
		}

		if result.Status == SubtaskResultStatusFailed {
			return TaskResult{Status: TaskResultStatusFailed, Error: fmt.Sprintf("subtask %s: %s", subtask.ID, result.Error)}, nil
		}

		finalOutput += result.Output + "\n"
	}

	logger.Info("task orchestration completed", "task_id", input.TaskID)
	return TaskResult{Status: TaskResultStatusCompleted, Output: finalOutput}, nil
}

func DecomposeTaskActivity(ctx context.Context, input TaskInput) ([]Subtask, error) {
	// TODO: call agent-runtime via gRPC to decompose the task
	return nil, fmt.Errorf("decompose task activity not yet implemented")
}

func ExecuteSubtaskActivity(ctx context.Context, subtask Subtask) (SubtaskResult, error) {
	// TODO: dispatch to agent-runtime via gRPC to execute the subtask
	return SubtaskResult{}, fmt.Errorf("execute subtask activity not yet implemented")
}
