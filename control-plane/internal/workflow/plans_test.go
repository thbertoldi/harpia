package workflow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func TestPlanWorkflowRunsStepsInTopologicalOrder(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := testPlanSnapshot()

	var createdSteps []string
	var ranSteps []string
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, stepInput CreateStepExecutionInput) (StepExecutionRecord, error) {
			createdSteps = append(createdSteps, stepInput.PlanStepKey)
			return StepExecutionRecord{
				ID:              "step-" + stepInput.PlanStepKey,
				PlanStepKey:     stepInput.PlanStepKey,
				Attempt:         1,
				InputArtifactID: stepInput.InputArtifactID,
			}, nil
		},
	)
	env.OnActivity(RunIntegrationActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, executorInput ExecutorActivityInput) (ExecutorActivityResult, error) {
			ranSteps = append(ranSteps, executorInput.PlanStepKey)
			if executorInput.PlanStepKey == "fetch-news" {
				assertArtifactIDs(t, executorInput.InputArtifacts, []string{"seed-date-range"})
			}
			return ExecutorActivityResult{
				Status:           ExecutorResultStatusCompleted,
				OutputArtifactID: "out-" + executorInput.PlanStepKey,
			}, nil
		},
	)
	env.OnActivity(RunAgentActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, executorInput ExecutorActivityInput) (ExecutorActivityResult, error) {
			ranSteps = append(ranSteps, executorInput.PlanStepKey)
			switch executorInput.PlanStepKey {
			case "write-draft":
				assertArtifactIDs(t, executorInput.InputArtifacts, []string{"out-fetch-news"})
			case "adapt-for-linkedin":
				assertArtifactIDs(t, executorInput.InputArtifacts, []string{"out-write-draft"})
			default:
				t.Fatalf("unexpected agent step %q", executorInput.PlanStepKey)
			}
			return ExecutorActivityResult{
				Status:           ExecutorResultStatusCompleted,
				OutputArtifactID: "out-" + executorInput.PlanStepKey,
			}, nil
		},
	)
	env.OnActivity(CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(CompletePlanExecutionActivityName, mock.Anything, input).Return(nil)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var result PlanWorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("result status = %q, want completed", result.Status)
	}

	expected := []string{"fetch-news", "write-draft", "adapt-for-linkedin"}
	if !reflect.DeepEqual(createdSteps, expected) {
		t.Fatalf("created steps = %#v, want %#v", createdSteps, expected)
	}
	if !reflect.DeepEqual(ranSteps, expected) {
		t.Fatalf("ran steps = %#v, want %#v", ranSteps, expected)
	}
}

func TestPlanWorkflowRetrySkipsUpstreamSteps(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:         "22222222-2222-2222-2222-222222222222",
		PlanExecutionID:  "retry-11111111-1111-1111-1111-111111111111",
		RetryFromStepKey: "write-draft",
		RetryStepArtifactsByKey: map[string]ArtifactRef{
			"fetch-news": {
				Source:          "step_output",
				StepKey:         "fetch-news",
				ArtifactID:      "out-fetch-news-original",
				ArtifactTypeKey: "harpia.artifacts.v1.NewsList",
			},
		},
	}
	snapshot := testPlanSnapshot()

	createdSteps := make([]string, 0, 2)
	ranSteps := make([]string, 0, 2)
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, stepInput CreateStepExecutionInput) (StepExecutionRecord, error) {
			createdSteps = append(createdSteps, stepInput.PlanStepKey)
			return StepExecutionRecord{
				ID:              "step-" + stepInput.PlanStepKey,
				PlanStepKey:     stepInput.PlanStepKey,
				Attempt:         2,
				InputArtifactID: stepInput.InputArtifactID,
			}, nil
		},
	)
	env.OnActivity(RunAgentActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, executorInput ExecutorActivityInput) (ExecutorActivityResult, error) {
			ranSteps = append(ranSteps, executorInput.PlanStepKey)
			switch executorInput.PlanStepKey {
			case "write-draft":
				assertArtifactIDs(t, executorInput.InputArtifacts, []string{"out-fetch-news-original"})
			case "adapt-for-linkedin":
				assertArtifactIDs(t, executorInput.InputArtifacts, []string{"out-write-draft"})
			default:
				t.Fatalf("unexpected retried step %q", executorInput.PlanStepKey)
			}
			return ExecutorActivityResult{
				Status:           ExecutorResultStatusCompleted,
				OutputArtifactID: "out-" + executorInput.PlanStepKey,
			}, nil
		},
	)
	env.OnActivity(CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(CompletePlanExecutionActivityName, mock.Anything, input).Return(nil)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if !reflect.DeepEqual(createdSteps, []string{"write-draft", "adapt-for-linkedin"}) {
		t.Fatalf("created steps = %#v", createdSteps)
	}
	if !reflect.DeepEqual(ranSteps, []string{"write-draft", "adapt-for-linkedin"}) {
		t.Fatalf("ran steps = %#v", ranSteps)
	}
}

func TestPlanWorkflowFailsMissingExecutorBinding(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := testPlanSnapshot()
	delete(snapshot.ExecutorInstallations, "write-draft")

	failStepCalled := false
	failPlanCalled := false
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, stepInput CreateStepExecutionInput) (StepExecutionRecord, error) {
			switch stepInput.PlanStepKey {
			case "fetch-news":
				return StepExecutionRecord{
					ID:          "step-fetch-news",
					PlanStepKey: "fetch-news",
					Attempt:     1,
				}, nil
			case "write-draft":
				return StepExecutionRecord{
					ID:          "step-write-draft",
					PlanStepKey: "write-draft",
					Attempt:     1,
				}, nil
			default:
				t.Fatalf("unexpected step = %q", stepInput.PlanStepKey)
			}
			return StepExecutionRecord{}, nil
		},
	)
	env.OnActivity(RunIntegrationActivityName, mock.Anything, mock.Anything).Return(ExecutorActivityResult{
		Status:           ExecutorResultStatusCompleted,
		OutputArtifactID: "out-fetch-news",
	}, nil)
	env.OnActivity(CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(FailStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, statusInput StepStatusUpdateInput) error {
			failStepCalled = statusInput.StepExecutionID == "step-write-draft"
			return nil
		},
	)
	env.OnActivity(FailPlanExecutionActivityName, mock.Anything, input).Return(
		func(ctx context.Context, workflowInput PlanWorkflowInput) error {
			failPlanCalled = true
			return nil
		},
	)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected workflow error")
	}
	if !failStepCalled {
		t.Fatal("expected FailStepExecutionActivity")
	}
	if !failPlanCalled {
		t.Fatal("expected FailPlanExecutionActivity")
	}
}

func TestPlanWorkflowMarksStepAndPlanFailedWhenExecutorFails(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := testPlanSnapshot()
	snapshot.Template.Steps = snapshot.Template.Steps[:1]
	snapshot.Template.Edges = nil
	snapshot.ExecutorInstallations = map[string]ExecutorInstallationSnapshot{
		"fetch-news": snapshot.ExecutorInstallations["fetch-news"],
	}

	failStepCalled := false
	failPlanCalled := false
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, stepInput CreateStepExecutionInput) (StepExecutionRecord, error) {
			return StepExecutionRecord{
				ID:          "step-fetch-news",
				PlanStepKey: "fetch-news",
				Attempt:     1,
			}, nil
		},
	)
	env.OnActivity(RunIntegrationActivityName, mock.Anything, mock.Anything).Return(ExecutorActivityResult{
		Status: ExecutorResultStatusFailed,
		Error:  "rss executor unavailable",
	}, nil)
	env.OnActivity(FailStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, statusInput StepStatusUpdateInput) error {
			failStepCalled = statusInput.StepExecutionID == "step-fetch-news"
			return nil
		},
	)
	env.OnActivity(FailPlanExecutionActivityName, mock.Anything, input).Return(
		func(ctx context.Context, workflowInput PlanWorkflowInput) error {
			failPlanCalled = true
			return nil
		},
	)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected workflow error")
	}
	if !failStepCalled {
		t.Fatal("expected FailStepExecutionActivity")
	}
	if !failPlanCalled {
		t.Fatal("expected FailPlanExecutionActivity")
	}
}

func TestPlanWorkflowWaitsForElicitationResponseAndResumesAgent(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := testPlanSnapshot()
	snapshot.Template.Steps = snapshot.Template.Steps[:2]
	snapshot.Template.Edges = snapshot.Template.Edges[:1]
	snapshot.ExecutorInstallations = map[string]ExecutorInstallationSnapshot{
		"fetch-news":  snapshot.ExecutorInstallations["fetch-news"],
		"write-draft": snapshot.ExecutorInstallations["write-draft"],
	}
	snapshot.Configuration.BehaviorPolicies = &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
		ElicitationTimeoutHours:    48,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
	}

	var agentCalls int
	awaitElicitationCalled := false
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, stepInput CreateStepExecutionInput) (StepExecutionRecord, error) {
			return StepExecutionRecord{
				ID:              "step-" + stepInput.PlanStepKey,
				PlanStepKey:     stepInput.PlanStepKey,
				Attempt:         1,
				InputArtifactID: stepInput.InputArtifactID,
			}, nil
		},
	)
	env.OnActivity(RunIntegrationActivityName, mock.Anything, mock.Anything).Return(ExecutorActivityResult{
		Status:           ExecutorResultStatusCompleted,
		OutputArtifactID: "out-fetch-news",
	}, nil)
	env.OnActivity(RunAgentActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, executorInput ExecutorActivityInput) (ExecutorActivityResult, error) {
			agentCalls++
			if agentCalls == 1 {
				if executorInput.ElicitationResponse != nil {
					t.Fatal("first agent call should not include elicitation response")
				}
				return ExecutorActivityResult{
					Status:              ExecutorResultStatusElicitationRequested,
					ElicitationThreadID: "thread-write-draft",
				}, nil
			}
			if executorInput.ElicitationResponse == nil || executorInput.ElicitationResponse.ResponseText != "use this context" {
				t.Fatalf("second agent call response = %#v", executorInput.ElicitationResponse)
			}
			return ExecutorActivityResult{
				Status:           ExecutorResultStatusCompleted,
				OutputArtifactID: "out-write-draft",
			}, nil
		},
	)
	env.OnActivity(AwaitElicitationStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, statusInput StepStatusUpdateInput) error {
			awaitElicitationCalled = statusInput.StepExecutionID == "step-write-draft" &&
				statusInput.ElicitationThreadID == "thread-write-draft"
			return nil
		},
	)
	env.OnActivity(ResumeStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(CompletePlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(PlanElicitationResponseSignalName, ElicitationResponseSignal{
			StepExecutionID:     "step-other",
			ElicitationThreadID: "thread-other",
			ResponseText:        "wrong context",
		})
		env.SignalWorkflow(PlanElicitationResponseSignalName, ElicitationResponseSignal{
			StepExecutionID:     "step-write-draft",
			ElicitationThreadID: "thread-write-draft",
			ResponseText:        "use this context",
		})
	}, time.Hour)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if agentCalls != 2 {
		t.Fatalf("agent calls = %d, want 2", agentCalls)
	}
	if !awaitElicitationCalled {
		t.Fatal("expected AwaitElicitationStepExecutionActivity")
	}
}

func TestPlanWorkflowFailsTimedOutElicitationPerPolicy(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := singleStepSnapshot("write-draft", ExecutorKindAgent)
	snapshot.Configuration.BehaviorPolicies = &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_STEP,
		ElicitationTimeoutHours:    1,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
	}

	failStepCalled := false
	failPlanCalled := false
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(StepExecutionRecord{
		ID:          "step-write-draft",
		PlanStepKey: "write-draft",
		Attempt:     1,
	}, nil)
	env.OnActivity(RunAgentActivityName, mock.Anything, mock.Anything).Return(ExecutorActivityResult{
		Status:              ExecutorResultStatusElicitationRequested,
		ElicitationThreadID: "thread-write-draft",
	}, nil)
	env.OnActivity(AwaitElicitationStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(TimeoutElicitationStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(FailStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, statusInput StepStatusUpdateInput) error {
			failStepCalled = statusInput.StepExecutionID == "step-write-draft"
			return nil
		},
	)
	env.OnActivity(FailPlanExecutionActivityName, mock.Anything, input).Return(
		func(ctx context.Context, workflowInput PlanWorkflowInput) error {
			failPlanCalled = true
			return nil
		},
	)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected workflow error")
	} else if !strings.Contains(err.Error(), "policy failed the step") {
		t.Fatalf("workflow error = %v, want FAIL_STEP policy context", err)
	}
	if !failStepCalled {
		t.Fatal("expected FailStepExecutionActivity")
	}
	if !failPlanCalled {
		t.Fatal("expected FailPlanExecutionActivity")
	}
}

func TestPlanWorkflowFailsPlanOnTimedOutElicitationPolicy(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := singleStepSnapshot("write-draft", ExecutorKindAgent)
	snapshot.Configuration.BehaviorPolicies = &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_PLAN,
		ElicitationTimeoutHours:    1,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
	}

	failStepCalled := false
	failPlanCalled := false
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(StepExecutionRecord{
		ID:          "step-write-draft",
		PlanStepKey: "write-draft",
		Attempt:     1,
	}, nil)
	env.OnActivity(RunAgentActivityName, mock.Anything, mock.Anything).Return(ExecutorActivityResult{
		Status:              ExecutorResultStatusElicitationRequested,
		ElicitationThreadID: "thread-write-draft",
	}, nil)
	env.OnActivity(AwaitElicitationStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(TimeoutElicitationStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(FailStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, statusInput StepStatusUpdateInput) error {
			failStepCalled = statusInput.StepExecutionID == "step-write-draft"
			return nil
		},
	)
	env.OnActivity(FailPlanExecutionActivityName, mock.Anything, input).Return(
		func(ctx context.Context, workflowInput PlanWorkflowInput) error {
			failPlanCalled = true
			return nil
		},
	)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected workflow error")
	} else if !strings.Contains(err.Error(), "policy failed the plan") {
		t.Fatalf("workflow error = %v, want FAIL_PLAN policy context", err)
	}
	if !failStepCalled {
		t.Fatal("expected FailStepExecutionActivity")
	}
	if !failPlanCalled {
		t.Fatal("expected FailPlanExecutionActivity")
	}
}

func TestPlanWorkflowRequiresApprovalBeforePublishStep(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := singleStepSnapshot("publish-linkedin", ExecutorKindIntegration)
	snapshot.Template.Steps[0].OutputArtifactTypeId = "harpia.artifacts.v1.PublishConfirmation"
	snapshot.Configuration.BehaviorPolicies = &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
		ElicitationTimeoutHours:    48,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
	}

	approvalRequestID := planApprovalRequestID(input.PlanExecutionID, "step-publish-linkedin")
	events := make([]string, 0, 2)
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(StepExecutionRecord{
		ID:          "step-publish-linkedin",
		PlanStepKey: "publish-linkedin",
		Attempt:     1,
	}, nil)
	env.OnActivity(CreateApprovalRequestActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, approvalInput CreateApprovalRequestInput) error {
			events = append(events, "create-approval")
			if approvalInput.ApprovalRequestID != approvalRequestID {
				t.Fatalf("approval request id = %q, want %q", approvalInput.ApprovalRequestID, approvalRequestID)
			}
			if approvalInput.PlanExecutionID != input.PlanExecutionID {
				t.Fatalf("plan execution id = %q, want %q", approvalInput.PlanExecutionID, input.PlanExecutionID)
			}
			return nil
		},
	)
	env.OnActivity(ResolveApprovalRequestActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, decisionInput ResolveApprovalRequestInput) error {
			events = append(events, "resolve-approval")
			if !decisionInput.Approved {
				t.Fatal("expected approved decision")
			}
			return nil
		},
	)
	env.OnActivity(ResumeStepExecutionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, statusInput StepStatusUpdateInput) error {
			events = append(events, "resume-step")
			return nil
		},
	)
	env.OnActivity(RunIntegrationActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, executorInput ExecutorActivityInput) (ExecutorActivityResult, error) {
			events = append(events, "run-integration")
			return ExecutorActivityResult{
				Status:           ExecutorResultStatusCompleted,
				OutputArtifactID: "out-publish-linkedin",
			}, nil
		},
	)
	env.OnActivity(CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(CompletePlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(PlanApprovalDecisionSignalName, ApprovalDecisionSignal{
			StepExecutionID:   "step-other",
			ApprovalRequestID: "approval-other",
			Approved:          true,
		})
		env.SignalWorkflow(PlanApprovalDecisionSignalName, ApprovalDecisionSignal{
			StepExecutionID:   "step-publish-linkedin",
			ApprovalRequestID: approvalRequestID,
			Approved:          true,
		})
	}, time.Hour)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if !reflect.DeepEqual(events, []string{"create-approval", "resolve-approval", "resume-step", "run-integration"}) {
		t.Fatalf("events = %#v", events)
	}
}

func TestPlanWorkflowAutoPublishSkipsApprovalGate(t *testing.T) {
	env := newPlanWorkflowTestEnv(t)

	input := PlanWorkflowInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanExecutionID: "11111111-1111-1111-1111-111111111111",
	}
	snapshot := singleStepSnapshot("publish-linkedin", ExecutorKindIntegration)
	snapshot.Template.Steps[0].OutputArtifactTypeId = "harpia.artifacts.v1.PublishConfirmation"
	snapshot.Configuration.BehaviorPolicies = &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
		ElicitationTimeoutHours:    48,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
	}

	runIntegrationCalled := false
	env.OnActivity(LoadPlanExecutionActivityName, mock.Anything, input).Return(LoadedPlanExecution{
		PlanExecutionID: input.PlanExecutionID,
		Status:          "pending",
		Snapshot:        snapshot,
	}, nil)
	env.OnActivity(StartPlanExecutionActivityName, mock.Anything, input).Return(nil)
	env.OnActivity(CreateStepExecutionActivityName, mock.Anything, mock.Anything).Return(StepExecutionRecord{
		ID:          "step-publish-linkedin",
		PlanStepKey: "publish-linkedin",
		Attempt:     1,
	}, nil)
	env.OnActivity(RunIntegrationActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, executorInput ExecutorActivityInput) (ExecutorActivityResult, error) {
			runIntegrationCalled = true
			return ExecutorActivityResult{
				Status:           ExecutorResultStatusCompleted,
				OutputArtifactID: "out-publish-linkedin",
			}, nil
		},
	)
	env.OnActivity(CompleteStepExecutionActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(CompletePlanExecutionActivityName, mock.Anything, input).Return(nil)

	env.ExecuteWorkflow(PlanWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if !runIntegrationCalled {
		t.Fatal("expected RunIntegrationActivity")
	}
}

func TestTopologicalPlanStepsRejectsCycles(t *testing.T) {
	template := &plansv1.PlanTemplate{
		Steps: []*plansv1.PlanStep{
			{Key: "a"},
			{Key: "b"},
		},
		Edges: []*plansv1.PlanStepDependency{
			{FromStepKey: "a", ToStepKey: "b"},
			{FromStepKey: "b", ToStepKey: "a"},
		},
	}

	if _, err := topologicalPlanSteps(template); err == nil {
		t.Fatal("expected cycle error")
	}
}

func newPlanWorkflowTestEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	t.Cleanup(func() {
		env.AssertExpectations(t)
	})
	env.RegisterActivity(LoadPlanExecutionActivity)
	env.RegisterActivity(StartPlanExecutionActivity)
	env.RegisterActivity(CreateStepExecutionActivity)
	env.RegisterActivity(RunIntegrationActivity)
	env.RegisterActivity(RunAgentActivity)
	env.RegisterActivity(ResumeStepExecutionActivity)
	env.RegisterActivity(CompleteStepExecutionActivity)
	env.RegisterActivity(FailStepExecutionActivity)
	env.RegisterActivity(AwaitElicitationStepExecutionActivity)
	env.RegisterActivity(TimeoutElicitationStepExecutionActivity)
	env.RegisterActivity(CreateApprovalRequestActivity)
	env.RegisterActivity(ResolveApprovalRequestActivity)
	env.RegisterActivity(CompletePlanExecutionActivity)
	env.RegisterActivity(FailPlanExecutionActivity)
	return env
}

func LoadPlanExecutionActivity(context.Context, PlanWorkflowInput) (LoadedPlanExecution, error) {
	return LoadedPlanExecution{}, unexpectedActivityError("LoadPlanExecutionActivity")
}

func StartPlanExecutionActivity(context.Context, PlanWorkflowInput) error {
	return unexpectedActivityError("StartPlanExecutionActivity")
}

func CreateStepExecutionActivity(context.Context, CreateStepExecutionInput) (StepExecutionRecord, error) {
	return StepExecutionRecord{}, unexpectedActivityError("CreateStepExecutionActivity")
}

func RunIntegrationActivity(context.Context, ExecutorActivityInput) (ExecutorActivityResult, error) {
	return ExecutorActivityResult{}, unexpectedActivityError("RunIntegrationActivity")
}

func RunAgentActivity(context.Context, ExecutorActivityInput) (ExecutorActivityResult, error) {
	return ExecutorActivityResult{}, unexpectedActivityError("RunAgentActivity")
}

func ResumeStepExecutionActivity(context.Context, StepStatusUpdateInput) error {
	return unexpectedActivityError("ResumeStepExecutionActivity")
}

func CompleteStepExecutionActivity(context.Context, StepStatusUpdateInput) error {
	return unexpectedActivityError("CompleteStepExecutionActivity")
}

func FailStepExecutionActivity(context.Context, StepStatusUpdateInput) error {
	return unexpectedActivityError("FailStepExecutionActivity")
}

func AwaitElicitationStepExecutionActivity(context.Context, StepStatusUpdateInput) error {
	return unexpectedActivityError("AwaitElicitationStepExecutionActivity")
}

func TimeoutElicitationStepExecutionActivity(context.Context, StepStatusUpdateInput) error {
	return unexpectedActivityError("TimeoutElicitationStepExecutionActivity")
}

func CreateApprovalRequestActivity(context.Context, CreateApprovalRequestInput) error {
	return unexpectedActivityError("CreateApprovalRequestActivity")
}

func ResolveApprovalRequestActivity(context.Context, ResolveApprovalRequestInput) error {
	return unexpectedActivityError("ResolveApprovalRequestActivity")
}

func CompletePlanExecutionActivity(context.Context, PlanWorkflowInput) error {
	return unexpectedActivityError("CompletePlanExecutionActivity")
}

func FailPlanExecutionActivity(context.Context, PlanWorkflowInput) error {
	return unexpectedActivityError("FailPlanExecutionActivity")
}

func unexpectedActivityError(name string) error {
	return errors.New("unexpected unmocked activity: " + name)
}

func testPlanSnapshot() PlanExecutionSnapshot {
	return PlanExecutionSnapshot{
		SchemaVersion: 1,
		Configuration: &plansv1.PlanConfiguration{
			Id:                  "config-1",
			TenantId:            "22222222-2222-2222-2222-222222222222",
			PlanTemplateId:      "template-1",
			PlanTemplateVersion: 1,
			Status:              plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
			SeedArtifacts: []*plansv1.SeedArtifactBinding{
				{
					StepKey:    "fetch-news",
					InputName:  "date_range",
					ArtifactId: "seed-date-range",
				},
			},
		},
		Template: &plansv1.PlanTemplate{
			Id:      "template-1",
			Key:     "weekly-newsletter-linkedin",
			Version: 1,
			Steps: []*plansv1.PlanStep{
				{
					Key:                  "fetch-news",
					InputArtifactTypeId:  "harpia.artifacts.v1.DateRange",
					OutputArtifactTypeId: "harpia.artifacts.v1.NewsList",
				},
				{
					Key:                  "write-draft",
					InputArtifactTypeId:  "harpia.artifacts.v1.NewsList",
					OutputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
				},
				{
					Key:                  "adapt-for-linkedin",
					InputArtifactTypeId:  "harpia.artifacts.v1.TextDraft",
					OutputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
				},
			},
			Edges: []*plansv1.PlanStepDependency{
				{FromStepKey: "fetch-news", ToStepKey: "write-draft"},
				{FromStepKey: "write-draft", ToStepKey: "adapt-for-linkedin"},
			},
		},
		ExecutorInstallations: map[string]ExecutorInstallationSnapshot{
			"fetch-news": {
				ID:   "installation-fetch",
				Kind: ExecutorKindIntegration,
			},
			"write-draft": {
				ID:   "installation-write",
				Kind: ExecutorKindAgent,
			},
			"adapt-for-linkedin": {
				ID:   "installation-adapt",
				Kind: ExecutorKindAgent,
			},
		},
	}
}

func singleStepSnapshot(stepKey string, executorKind string) PlanExecutionSnapshot {
	snapshot := testPlanSnapshot()
	snapshot.Configuration.SeedArtifacts = nil
	snapshot.Configuration.BehaviorPolicies = &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
		ElicitationTimeoutHours:    48,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
	}
	snapshot.Template.Steps = []*plansv1.PlanStep{
		{
			Key:                  stepKey,
			InputArtifactTypeId:  "",
			OutputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
		},
	}
	snapshot.Template.Edges = nil
	snapshot.ExecutorInstallations = map[string]ExecutorInstallationSnapshot{
		stepKey: {
			ID:   "installation-" + stepKey,
			Kind: executorKind,
		},
	}
	return snapshot
}

func assertArtifactIDs(t *testing.T, artifacts []ArtifactRef, expected []string) {
	t.Helper()
	actual := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		actual = append(actual, artifact.ArtifactID)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("artifact ids = %v, want %v", actual, expected)
	}
}

func TestStepInputArtifactsAllowsInternalContentPreferencesSeed(t *testing.T) {
	step := &plansv1.PlanStep{
		Key:                 "write-draft",
		InputArtifactTypeId: "harpia.artifacts.v1.NewsList",
	}
	inputs, err := stepInputArtifacts(
		step,
		[]string{"fetch-news"},
		map[string][]ArtifactRef{
			"write-draft": {{
				Source:      "seed",
				StepKey:     "write-draft",
				InputName:   "harpia.internal.ContentPreferences",
				LiteralJSON: `{"tone":"analytical"}`,
			}},
		},
		map[string]ArtifactRef{
			"fetch-news": {
				Source:          "step_output",
				StepKey:         "fetch-news",
				ArtifactID:      "art-news",
				ArtifactTypeKey: "harpia.artifacts.v1.NewsList",
			},
		},
	)
	if err != nil {
		t.Fatalf("stepInputArtifacts returned error: %v", err)
	}
	if len(inputs) == 0 {
		t.Fatal("expected input artifacts")
	}
	if got := inputs[0].ArtifactTypeKey; got != "harpia.internal.ContentPreferences" {
		t.Fatalf("internal seed artifact type = %q", got)
	}
}

func TestPlanWorkflowID(t *testing.T) {
	id := PlanWorkflowID("11111111-1111-1111-1111-111111111111")
	if got, want := id, fmt.Sprintf("plan-execution-%s", "11111111-1111-1111-1111-111111111111"); got != want {
		t.Fatalf("PlanWorkflowID() = %q, want %q", got, want)
	}
}
