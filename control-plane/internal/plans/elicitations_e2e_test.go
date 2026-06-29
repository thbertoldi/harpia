package plans

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

// fakeElicitationStore is an in-memory ElicitationStore for handler tests.
type fakeElicitationStore struct {
	elicitation *Elicitation
	answered    bool
}

func (f *fakeElicitationStore) GetElicitation(_ context.Context, _ uuid.UUID, id uuid.UUID) (*Elicitation, error) {
	if f.elicitation == nil || f.elicitation.ID != id {
		return nil, connect.NewError(connect.CodeNotFound, errFakeNotFound)
	}
	clone := *f.elicitation
	return &clone, nil
}

func (f *fakeElicitationStore) ListElicitations(_ context.Context, _ ElicitationFilter) ([]Elicitation, error) {
	if f.elicitation == nil {
		return nil, nil
	}
	return []Elicitation{*f.elicitation}, nil
}

func (f *fakeElicitationStore) MarkElicitationAnswered(_ context.Context, _ uuid.UUID, id uuid.UUID, payloadJSON json.RawMessage, responseText string, respondedBy uuid.NullUUID) (*Elicitation, error) {
	f.answered = true
	now := time.Now().UTC()
	updated := *f.elicitation
	updated.Status = ElicitationStatusAnswered
	updated.ResponseJSON = payloadJSON
	updated.ResponseText = responseText
	updated.RespondedBy = respondedBy
	updated.RespondedAt = &now
	f.elicitation = &updated
	return &updated, nil
}

var errFakeNotFound = connectError("elicitation not found")

type connectError string

func (e connectError) Error() string { return string(e) }

// fakeSignaler forwards the elicitation response into the running test workflow.
type fakeSignaler struct {
	deliver func(signal workflow.ElicitationResponseSignal)
	called  bool
}

func (f *fakeSignaler) SignalPlanElicitationResponse(_ context.Context, _ string, _ string, signal workflow.ElicitationResponseSignal) error {
	f.called = true
	f.deliver(signal)
	return nil
}

// TestRespondToElicitationResumesWorkflow exercises the full E5.1 acceptance
// path: an agent step emits an elicitation, the overseer responds via the
// handler, and the plan-elicitation-response signal resumes the workflow so the
// step completes.
func TestRespondToElicitationResumesWorkflow(t *testing.T) {
	tenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	planExecutionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	stepExecutionID := uuid.New()
	elicitationID := uuid.New()
	overseerID := uuid.New()
	const threadID = "thread-write-draft"

	store := &fakeElicitationStore{
		elicitation: &Elicitation{
			ID:                  elicitationID,
			TenantID:            tenantID,
			PlanExecutionID:     planExecutionID,
			StepExecutionID:     stepExecutionID,
			PlanStepKey:         "write-draft",
			ElicitationThreadID: threadID,
			Status:              ElicitationStatusPending,
			Prompt:              "What tone should I use?",
			OverseerUserID:      uuid.NullUUID{UUID: overseerID, Valid: true},
		},
	}

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()

	signaler := &fakeSignaler{
		deliver: func(signal workflow.ElicitationResponseSignal) {
			env.SignalWorkflow(workflow.PlanElicitationResponseSignalName, signal)
		},
	}
	handler := &PlanHandler{elicitations: store, signaler: signaler}

	input := workflow.PlanWorkflowInput{
		TenantID:        tenantID.String(),
		PlanExecutionID: planExecutionID.String(),
	}
	snapshot := singleAgentStepSnapshot()

	registerStubActivities(env)
	env.OnActivity(workflow.LoadPlanExecutionActivityName, mock.Anything, input).Return(workflow.LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(workflow.StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(workflow.CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(workflow.StepExecutionRecord{
		ID:          stepExecutionID.String(),
		PlanStepKey: "write-draft",
		Attempt:     1,
	}, nil)

	agentCalls := 0
	env.OnActivity(workflow.RunAgentActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in workflow.ExecutorActivityInput) (workflow.ExecutorActivityResult, error) {
			agentCalls++
			if agentCalls == 1 {
				return workflow.ExecutorActivityResult{
					Status:              workflow.ExecutorResultStatusElicitationRequested,
					ElicitationThreadID: threadID,
					ElicitationPrompt:   "What tone should I use?",
				}, nil
			}
			if in.ElicitationResponse == nil || in.ElicitationResponse.ResponseText != "Use a confident tone" {
				t.Fatalf("resumed agent call missing response: %#v", in.ElicitationResponse)
			}
			return workflow.ExecutorActivityResult{
				Status:           workflow.ExecutorResultStatusCompleted,
				OutputArtifactID: "out-write-draft",
			}, nil
		},
	)
	env.OnActivity(workflow.AwaitElicitationStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(workflow.ResumeStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(workflow.CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(workflow.CompletePlanExecutionActivityName, mock.Anything, input).Return(nil)

	// Once the step is awaiting the elicitation, the overseer answers via the RPC
	// handler, which persists the response and signals the workflow.
	env.RegisterDelayedCallback(func() {
		ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
			TenantID: tenantID,
			UserID:   overseerID.String(),
			Roles:    []string{"Overseer"},
		})
		_, err := handler.RespondToElicitation(ctx, connect.NewRequest(&plansv1.RespondToElicitationRequest{
			TenantId:      tenantID.String(),
			ElicitationId: elicitationID.String(),
			ResponseText:  "Use a confident tone",
		}))
		if err != nil {
			t.Errorf("RespondToElicitation: %v", err)
		}
	}, time.Hour)

	env.ExecuteWorkflow(workflow.PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if agentCalls != 2 {
		t.Fatalf("agent calls = %d, want 2", agentCalls)
	}
	if !store.answered {
		t.Fatal("expected elicitation to be marked answered")
	}
	if !signaler.called {
		t.Fatal("expected elicitation signal to be sent")
	}
}

func singleAgentStepSnapshot() workflow.PlanExecutionSnapshot {
	return workflow.PlanExecutionSnapshot{
		SchemaVersion: 1,
		Configuration: &plansv1.PlanConfiguration{
			Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
			BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
				ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
				ElicitationTimeoutHours:    48,
				PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
			},
		},
		Template: &plansv1.PlanTemplate{
			Id:      "template-1",
			Key:     "single-agent",
			Version: 1,
			Steps: []*plansv1.PlanStep{
				{Key: "write-draft", OutputArtifactTypeId: "harpia.artifacts.v1.TextDraft"},
			},
		},
		ExecutorInstallations: map[string]workflow.ExecutorInstallationSnapshot{
			"write-draft": {ID: "installation-write-draft", Kind: workflow.ExecutorKindAgent},
		},
	}
}

func registerStubActivities(env *testsuite.TestWorkflowEnvironment) {
	register := func(name string, fn any) {
		env.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: name})
	}
	register(workflow.LoadPlanExecutionActivityName, func(context.Context, workflow.PlanWorkflowInput) (workflow.LoadedPlanExecution, error) {
		return workflow.LoadedPlanExecution{}, nil
	})
	register(workflow.StartPlanExecutionActivityName, func(context.Context, workflow.PlanWorkflowInput) error { return nil })
	register(workflow.CreateStepExecutionActivityName, func(context.Context, workflow.CreateStepExecutionInput) (workflow.StepExecutionRecord, error) {
		return workflow.StepExecutionRecord{}, nil
	})
	register(workflow.RunAgentActivityName, func(context.Context, workflow.ExecutorActivityInput) (workflow.ExecutorActivityResult, error) {
		return workflow.ExecutorActivityResult{}, nil
	})
	register(workflow.RunIntegrationActivityName, func(context.Context, workflow.ExecutorActivityInput) (workflow.ExecutorActivityResult, error) {
		return workflow.ExecutorActivityResult{}, nil
	})
	register(workflow.AwaitElicitationStepExecutionActivityName, func(context.Context, workflow.StepStatusUpdateInput) error { return nil })
	register(workflow.TimeoutElicitationStepExecutionActivityName, func(context.Context, workflow.StepStatusUpdateInput) error { return nil })
	register(workflow.ResumeStepExecutionActivityName, func(context.Context, workflow.StepStatusUpdateInput) error { return nil })
	register(workflow.CompleteStepExecutionActivityName, func(context.Context, workflow.StepStatusUpdateInput) error { return nil })
	register(workflow.FailStepExecutionActivityName, func(context.Context, workflow.StepStatusUpdateInput) error { return nil })
	register(workflow.CompletePlanExecutionActivityName, func(context.Context, workflow.PlanWorkflowInput) error { return nil })
	register(workflow.FailPlanExecutionActivityName, func(context.Context, workflow.PlanFailureInput) error { return nil })
}
