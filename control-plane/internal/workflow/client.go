package workflow

import (
	"context"
	"fmt"

	"go.temporal.io/api/enums/v1"
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

func (tc *TemporalClient) StartPlanWorkflow(ctx context.Context, input PlanWorkflowInput) (client.WorkflowRun, error) {
	opts := client.StartWorkflowOptions{
		ID:                    PlanWorkflowID(input.PlanExecutionID),
		TaskQueue:             TaskQueueName,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}
	return tc.client.ExecuteWorkflow(ctx, opts, PlanWorkflow, input)
}

func (tc *TemporalClient) SignalPlanElicitationResponse(ctx context.Context, workflowID string, runID string, signal ElicitationResponseSignal) error {
	return tc.client.SignalWorkflow(ctx, workflowID, runID, PlanElicitationResponseSignalName, signal)
}

func (tc *TemporalClient) SignalPlanApprovalDecision(ctx context.Context, workflowID string, runID string, signal ApprovalDecisionSignal) error {
	return tc.client.SignalWorkflow(ctx, workflowID, runID, PlanApprovalDecisionSignalName, signal)
}

func (tc *TemporalClient) RawClient() client.Client {
	return tc.client
}

func (tc *TemporalClient) Close() {
	tc.client.Close()
}

func PlanWorkflowID(planExecutionID string) string {
	return fmt.Sprintf("plan-execution-%s", planExecutionID)
}
