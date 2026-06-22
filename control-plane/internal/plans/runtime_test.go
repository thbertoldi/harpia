package plans

import (
	"testing"
	"time"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/workflow"
)

func TestExecutionFailureReasonUsesLatestFailedStep(t *testing.T) {
	now := time.Now().UTC()
	exec := &PlanExecution{
		StepExecutions: []StepExecution{
			{PlanStepKey: "step-1", Status: StepStatusCompleted, CreatedAt: now},
			{PlanStepKey: "step-2", Status: StepStatusFailed, CreatedAt: now.Add(time.Minute)},
		},
	}
	if got, want := executionFailureReason(exec), "step step-2 failed"; got != want {
		t.Fatalf("failure reason = %q, want %q", got, want)
	}
}

func TestExecutionFailureReasonEmptyWhenNoFailedStep(t *testing.T) {
	exec := &PlanExecution{
		StepExecutions: []StepExecution{
			{PlanStepKey: "step-1", Status: StepStatusCompleted},
		},
	}
	if got := executionFailureReason(exec); got != "" {
		t.Fatalf("failure reason = %q, want empty", got)
	}
}

func TestBuildRetryPlanWorkflowInputRequiresFailedExecution(t *testing.T) {
	executionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	failedStepID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	input, err := buildRetryPlanWorkflowInput(testRetrySnapshot(), &PlanExecution{
		ID:     executionID,
		Status: ExecutionStatusRunning,
		StepExecutions: []StepExecution{
			{
				ID:              failedStepID,
				PlanExecutionID: executionID,
				PlanStepKey:     "step-2",
				Status:          StepStatusFailed,
				Attempt:         1,
			},
		},
	}, failedStepID)
	if err == nil {
		t.Fatalf("expected error, got input %#v", input)
	}
}

func TestBuildRetryPlanWorkflowInputRejectsMissingUpstreamArtifact(t *testing.T) {
	executionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	failedStepID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	now := time.Now().UTC()
	_, err := buildRetryPlanWorkflowInput(testRetrySnapshot(), &PlanExecution{
		ID:     executionID,
		Status: ExecutionStatusFailed,
		StepExecutions: []StepExecution{
			{
				ID:               uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				PlanExecutionID:  executionID,
				PlanStepKey:      "step-1",
				Status:           StepStatusCompleted,
				OutputArtifactID: "",
				Attempt:          1,
				CreatedAt:        now,
			},
			{
				ID:              failedStepID,
				PlanExecutionID: executionID,
				PlanStepKey:     "step-2",
				Status:          StepStatusFailed,
				Attempt:         1,
				CreatedAt:       now.Add(time.Minute),
			},
		},
	}, failedStepID)
	if err == nil {
		t.Fatal("expected missing upstream artifact error")
	}
}

func TestBuildRetryPlanWorkflowInputBuildsReusableArtifacts(t *testing.T) {
	executionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	failedStepID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	now := time.Now().UTC()
	result, err := buildRetryPlanWorkflowInput(testRetrySnapshot(), &PlanExecution{
		ID:     executionID,
		Status: ExecutionStatusFailed,
		StepExecutions: []StepExecution{
			{
				ID:               uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				PlanExecutionID:  executionID,
				PlanStepKey:      "step-1",
				Status:           StepStatusCompleted,
				OutputArtifactID: "artifact-step-1",
				Attempt:          1,
				CreatedAt:        now,
			},
			{
				ID:               failedStepID,
				PlanExecutionID:  executionID,
				PlanStepKey:      "step-2",
				Status:           StepStatusFailed,
				OutputArtifactID: "",
				Attempt:          1,
				CreatedAt:        now.Add(time.Minute),
			},
		},
	}, failedStepID)
	if err != nil {
		t.Fatalf("build retry input: %v", err)
	}
	if result.retryFromStepKey != "step-2" {
		t.Fatalf("retry from step = %q", result.retryFromStepKey)
	}
	reused, ok := result.reusedArtifactsByStep["step-1"]
	if !ok {
		t.Fatal("missing reused artifact for step-1")
	}
	if reused.ArtifactID != "artifact-step-1" {
		t.Fatalf("artifact id = %q", reused.ArtifactID)
	}
}

func testRetrySnapshot() workflow.PlanExecutionSnapshot {
	return workflow.PlanExecutionSnapshot{
		SchemaVersion: 1,
		Configuration: &plansv1.PlanConfiguration{
			Id:     "cfg-1",
			Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		},
		Template: &plansv1.PlanTemplate{
			Key: "retry-template",
			Steps: []*plansv1.PlanStep{
				{Key: "step-1", OutputArtifactTypeId: "harpia.artifacts.v1.TypeA"},
				{Key: "step-2", OutputArtifactTypeId: "harpia.artifacts.v1.TypeB"},
			},
			Edges: []*plansv1.PlanStepDependency{
				{FromStepKey: "step-1", ToStepKey: "step-2"},
			},
		},
		ExecutorInstallations: map[string]workflow.ExecutorInstallationSnapshot{
			"step-1": {ID: "install-1", Kind: workflow.ExecutorKindIntegration},
			"step-2": {ID: "install-2", Kind: workflow.ExecutorKindAgent},
		},
	}
}
