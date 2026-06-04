package workflow

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
)

type TemporalClient struct {
	client client.Client
}

func NewTemporalClient(hostPort string) (*TemporalClient, error) {
	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		return nil, fmt.Errorf("temporal client: %w", err)
	}
	return &TemporalClient{client: c}, nil
}

func (tc *TemporalClient) StartTaskWorkflow(ctx context.Context, input TaskInput) (client.WorkflowRun, error) {
	opts := client.StartWorkflowOptions{
		ID:        input.TaskID,
		TaskQueue: TaskQueueName,
	}
	return tc.client.ExecuteWorkflow(ctx, opts, TaskOrchestration, input)
}

func (tc *TemporalClient) SignalFeedback(ctx context.Context, workflowID string, runID string, signal HumanFeedbackSignal) error {
	return tc.client.SignalWorkflow(ctx, workflowID, runID, HumanFeedbackSignalName, signal)
}

func (tc *TemporalClient) Close() {
	tc.client.Close()
}
