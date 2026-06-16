package plans

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

func TestRetryPlanExecutionStartsWorkflow(t *testing.T) {
	tenantID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	originalExecutionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	failedStepExecutionID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	retryExecutionID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	retryer := &stubRetryer{
		execution: &PlanExecution{
			ID:                  retryExecutionID,
			TenantID:            tenantID,
			PlanConfigurationID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
			Status:              ExecutionStatusPending,
		},
		workflowInput: workflow.PlanWorkflowInput{
			TenantID:                tenantID.String(),
			PlanExecutionID:         retryExecutionID.String(),
			RetryFromStepKey:        "write-draft",
			RetryStepArtifactsByKey: map[string]workflow.ArtifactRef{"fetch-news": {ArtifactID: "artifact-1"}},
		},
	}
	starter := &stubPlanWorkflowStarter{}
	handler := &PlanHandler{
		repo:            &Repository{},
		runtime:         retryer,
		workflowStarter: starter,
	}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{TenantID: tenantID})
	req := connect.NewRequest(&plansv1.RetryPlanExecutionRequest{
		TenantId:        tenantID.String(),
		PlanExecutionId: originalExecutionID.String(),
		StepExecutionId: failedStepExecutionID.String(),
	})

	resp, err := handler.RetryPlanExecution(ctx, req)
	if err != nil {
		t.Fatalf("RetryPlanExecution() error = %v", err)
	}
	if resp.Msg.PlanExecution == nil {
		t.Fatal("expected plan execution in response")
	}
	if resp.Msg.PlanExecution.Id != retryExecutionID.String() {
		t.Fatalf("retry execution id = %q", resp.Msg.PlanExecution.Id)
	}
	if starter.started.PlanExecutionID != retryExecutionID.String() {
		t.Fatalf("workflow started with plan_execution_id = %q", starter.started.PlanExecutionID)
	}
}

func TestRetryPlanExecutionPropagatesRetryValidationError(t *testing.T) {
	tenantID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	handler := &PlanHandler{
		runtime:         &stubRetryer{err: errors.New("execution is not failed")},
		workflowStarter: &stubPlanWorkflowStarter{},
	}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{TenantID: tenantID})
	req := connect.NewRequest(&plansv1.RetryPlanExecutionRequest{
		TenantId:        tenantID.String(),
		PlanExecutionId: uuid.NewString(),
		StepExecutionId: uuid.NewString(),
	})

	_, err := handler.RetryPlanExecution(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
	connectErr := new(connect.Error)
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect error, got %T", err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("error code = %s", connectErr.Code())
	}
}

type stubRetryer struct {
	execution     *PlanExecution
	workflowInput workflow.PlanWorkflowInput
	err           error
}

func (s *stubRetryer) PrepareRetryFromStep(_ context.Context, _, _, _ uuid.UUID) (*PlanExecution, workflow.PlanWorkflowInput, error) {
	if s.err != nil {
		return nil, workflow.PlanWorkflowInput{}, s.err
	}
	return s.execution, s.workflowInput, nil
}

type stubPlanWorkflowStarter struct {
	started workflow.PlanWorkflowInput
	err     error
}

func (s *stubPlanWorkflowStarter) StartPlanWorkflow(_ context.Context, input workflow.PlanWorkflowInput) (client.WorkflowRun, error) {
	s.started = input
	if s.err != nil {
		return nil, s.err
	}
	return stubWorkflowRun{}, nil
}

type stubWorkflowRun struct{}

func (stubWorkflowRun) Get(context.Context, interface{}) error { return nil }
func (stubWorkflowRun) GetWithOptions(context.Context, interface{}, client.WorkflowRunGetOptions) error {
	return nil
}
func (stubWorkflowRun) GetID() string    { return "workflow-id" }
func (stubWorkflowRun) GetRunID() string { return "run-id" }
