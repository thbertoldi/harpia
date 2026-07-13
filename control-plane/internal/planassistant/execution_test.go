package planassistant_test

import (
	"testing"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/planassistant"
)

func TestDeriveConversationTurn_ConfigurationStates(t *testing.T) {
	template := executionTemplate()
	tests := []struct {
		name            string
		status          plansv1.PlanConfigurationStatus
		want            planassistant.ConversationTurnKind
		wantConfigState planassistant.StateKind
	}{
		{"draft", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, planassistant.ConversationTurnConfiguring, planassistant.StateBindingMatrix},
		{"runnable", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, planassistant.ConversationTurnReadyToRun, ""},
		{"scheduled", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED, planassistant.ConversationTurnWaitingForSchedule, ""},
		{"disabled", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DISABLED, planassistant.ConversationTurnConfigurationDisabled, ""},
		{"archived", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_ARCHIVED, planassistant.ConversationTurnConfigurationArchived, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configuration := executionConfiguration(tt.status)
			turn, err := planassistant.DeriveConversationTurn(planassistant.TurnInput{
				Template:      template,
				Configuration: configuration,
			})
			if err != nil {
				t.Fatalf("DeriveConversationTurn() error = %v", err)
			}
			if turn.Kind != tt.want {
				t.Fatalf("Kind = %s, want %s", turn.Kind, tt.want)
			}
			if turn.ConfigurationState.Kind != tt.wantConfigState {
				t.Fatalf("ConfigurationState.Kind = %s, want %s", turn.ConfigurationState.Kind, tt.wantConfigState)
			}
			if turn.Execution != nil {
				t.Fatal("configuration turn must not include an execution view")
			}
		})
	}
}

func TestDeriveConversationTurn_ExecutionStates(t *testing.T) {
	tests := []struct {
		name       string
		execution  *plansv1.PlanExecution
		pending    []planassistant.PendingInteraction
		want       planassistant.ConversationTurnKind
		wantStepID string
	}{
		{"queued", executionForTest("exec-queued", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_PENDING), nil, planassistant.ConversationTurnExecutionQueued, ""},
		{"running", executionForTest("exec-running", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING), nil, planassistant.ConversationTurnExecutionRunning, "step-publish"},
		{"awaiting elicitation", executionForTest("exec-elicit", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING), []planassistant.PendingInteraction{pendingInteraction("exec-elicit", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_ELICITATION, "elicitation-1", "")}, planassistant.ConversationTurnAwaitingElicitation, "step-publish"},
		{"awaiting review", executionForTest("exec-review", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING), []planassistant.PendingInteraction{pendingInteraction("exec-review", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW, "review-1", "version-1")}, planassistant.ConversationTurnAwaitingReview, "step-publish"},
		{"awaiting approval", executionForTest("exec-approval", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING), []planassistant.PendingInteraction{pendingInteraction("exec-approval", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_APPROVAL, "approval-1", "version-1")}, planassistant.ConversationTurnAwaitingApproval, "step-publish"},
		{"failing", failedRunningExecution(), nil, planassistant.ConversationTurnExecutionFailing, "step-publish"},
		{"failed", failedExecution(), nil, planassistant.ConversationTurnExecutionFailed, "step-publish"},
		{"completed", executionForTest("exec-completed", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_COMPLETED), nil, planassistant.ConversationTurnExecutionCompleted, ""},
		{"cancelled", executionForTest("exec-cancelled", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_CANCELLED), nil, planassistant.ConversationTurnExecutionCancelled, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turn, err := planassistant.DeriveConversationTurn(planassistant.TurnInput{
				Configuration: executionConfiguration(plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE),
				Executions:    []*plansv1.PlanExecution{tt.execution},
				Target: planassistant.ConversationTarget{
					PlanExecutionID: tt.execution.GetId(),
				},
				PendingInteractions: tt.pending,
			})
			if err != nil {
				t.Fatalf("DeriveConversationTurn() error = %v", err)
			}
			if turn.Kind != tt.want {
				t.Fatalf("Kind = %s, want %s", turn.Kind, tt.want)
			}
			if turn.Execution == nil {
				t.Fatal("execution turn must include a view")
			}
			if turn.Execution.StepExecutionID != tt.wantStepID {
				t.Fatalf("StepExecutionID = %q, want %q", turn.Execution.StepExecutionID, tt.wantStepID)
			}
			if got, want := len(turn.Execution.OrderedActiveSteps), 2; got != want {
				t.Fatalf("ordered active steps = %d, want %d", got, want)
			}
			if got, want := turn.Execution.OrderedActiveSteps[0].GetKey(), "draft"; got != want {
				t.Fatalf("first frozen ordered step = %q, want %q", got, want)
			}
		})
	}
}

func TestDeriveConversationTurn_TargetResolutionPrefersExactThenLatestDisplayFallback(t *testing.T) {
	older := executionForTest("exec-older", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	older.UpdatedAt = "2026-07-12T10:00:00Z"
	newer := executionForTest("exec-newer", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	newer.UpdatedAt = "2026-07-12T11:00:00Z"
	input := planassistant.TurnInput{
		Configuration: executionConfiguration(plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE),
		Executions:    []*plansv1.PlanExecution{older, newer},
	}

	exactInput := input
	exactInput.Target = planassistant.ConversationTarget{PlanExecutionID: older.GetId()}
	exact, err := planassistant.DeriveConversationTurn(exactInput)
	if err != nil {
		t.Fatalf("DeriveConversationTurn(exact) error = %v", err)
	}
	if exact.Execution.PlanExecutionID != older.GetId() || exact.Target.DisplayFallback {
		t.Fatalf("exact target = %+v, want exact older execution", exact.Target)
	}

	fallback, err := planassistant.DeriveConversationTurn(input)
	if err != nil {
		t.Fatalf("DeriveConversationTurn(fallback) error = %v", err)
	}
	if fallback.Execution.PlanExecutionID != newer.GetId() || !fallback.Target.DisplayFallback {
		t.Fatalf("fallback target = %+v, want display fallback to newer execution", fallback.Target)
	}
}

func TestDeriveConversationTurn_FailsSafeForInteractionConflicts(t *testing.T) {
	tests := []struct {
		name      string
		execution *plansv1.PlanExecution
		pending   []planassistant.PendingInteraction
	}{
		{
			name:      "more than one pending interaction",
			execution: executionForTest("exec-many", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING),
			pending: []planassistant.PendingInteraction{
				pendingInteraction("exec-many", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW, "review-1", "version-1"),
				pendingInteraction("exec-many", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_APPROVAL, "approval-1", "version-1"),
			},
		},
		{
			name:      "terminal execution with stale request",
			execution: executionForTest("exec-terminal", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_COMPLETED),
			pending: []planassistant.PendingInteraction{
				pendingInteraction("exec-terminal", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW, "review-1", "version-1"),
			},
		},
		{
			name:      "missing request identity",
			execution: executionForTest("exec-missing-request", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING),
			pending: []planassistant.PendingInteraction{
				pendingInteraction("exec-missing-request", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW, "", "version-1"),
			},
		},
		{
			name:      "cross execution interaction identity",
			execution: executionForTest("exec-focused", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING),
			pending: []planassistant.PendingInteraction{
				pendingInteraction("exec-other", "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_APPROVAL, "approval-1", "version-1"),
			},
		},
		{
			name:      "missing step execution identity",
			execution: executionForTest("exec-missing-step", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING),
			pending: []planassistant.PendingInteraction{
				pendingInteraction("exec-missing-step", "", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_APPROVAL, "approval-1", "version-1"),
			},
		},
		{
			name:      "duplicate action identity",
			execution: executionForTest("exec-duplicate-action", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING),
			pending: []planassistant.PendingInteraction{
				{
					Kind:            chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW,
					RequestID:       "review-1",
					PlanExecutionID: "exec-duplicate-action",
					StepExecutionID: "step-publish",
					PlanStepKey:     "publish",
					Actions: []*chatv1.ExecutionPromptAction{
						{ActionId: "review.accept", LabelKey: "execution.review.accept"},
						{ActionId: "review.accept", LabelKey: "execution.review.accept"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turn, err := planassistant.DeriveConversationTurn(planassistant.TurnInput{
				Configuration:       executionConfiguration(plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE),
				Executions:          []*plansv1.PlanExecution{tt.execution},
				Target:              planassistant.ConversationTarget{PlanExecutionID: tt.execution.GetId()},
				PendingInteractions: tt.pending,
			})
			if err != nil {
				t.Fatalf("DeriveConversationTurn() error = %v", err)
			}
			if turn.Kind != planassistant.ConversationTurnExecutionNeedsAttention {
				t.Fatalf("Kind = %s, want EXECUTION_NEEDS_ATTENTION", turn.Kind)
			}
			if turn.Execution.PendingInteraction != nil || len(turn.Execution.Actions) != 0 {
				t.Fatalf("needs-attention turn must suppress unsafe actions: %+v", turn.Execution)
			}
		})
	}
}

func TestDeriveConversationTurn_UsesFrozenActiveGraphForProgressAndArtifact(t *testing.T) {
	execution := executionForTest("exec-progress", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	turn, err := planassistant.DeriveConversationTurn(planassistant.TurnInput{
		Configuration: executionConfiguration(plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE),
		Executions:    []*plansv1.PlanExecution{execution},
		Target:        planassistant.ConversationTarget{PlanExecutionID: execution.GetId()},
	})
	if err != nil {
		t.Fatalf("DeriveConversationTurn() error = %v", err)
	}
	if got, want := turn.Execution.CompletedStepCount, int32(1); got != want {
		t.Fatalf("CompletedStepCount = %d, want %d", got, want)
	}
	if got, want := turn.Execution.ActiveStepCount, int32(2); got != want {
		t.Fatalf("ActiveStepCount = %d, want %d", got, want)
	}
	if got, want := turn.Execution.LatestArtifactRef.GetArtifactVersionId(), "version-draft"; got != want {
		t.Fatalf("LatestArtifactRef.artifact_version_id = %q, want %q", got, want)
	}
}

func TestDeriveConversationTurn_RevisionHasNewFingerprint(t *testing.T) {
	execution := executionForTest("exec-review", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	derive := func(requestID, versionID string) string {
		t.Helper()
		turn, err := planassistant.DeriveConversationTurn(planassistant.TurnInput{
			Configuration: executionConfiguration(plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE),
			Executions:    []*plansv1.PlanExecution{execution},
			Target:        planassistant.ConversationTarget{PlanExecutionID: execution.GetId()},
			PendingInteractions: []planassistant.PendingInteraction{
				pendingInteraction(execution.GetId(), "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW, requestID, versionID),
			},
		})
		if err != nil {
			t.Fatalf("DeriveConversationTurn() error = %v", err)
		}
		return turn.Execution.ExecutionPromptFingerprint
	}

	if first, revision := derive("review-1", "version-1"), derive("review-2", "version-2"); first == revision {
		t.Fatalf("revision fingerprint %q must differ from prior request/version", revision)
	}
}

func executionTemplate() *plansv1.PlanTemplate {
	return &plansv1.PlanTemplate{
		Id: "template-frozen",
		Steps: []*plansv1.PlanStep{
			{Key: "draft", Title: "Frozen draft"},
			{Key: "publish", Title: "Frozen publish"},
		},
	}
}

func executionConfiguration(status plansv1.PlanConfigurationStatus) *plansv1.PlanConfiguration {
	return &plansv1.PlanConfiguration{
		Id:             "configuration-1",
		PlanTemplateId: "template-current",
		Status:         status,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "draft", ExecutorInstallationId: "installation-draft"},
			{StepKey: "publish", ExecutorInstallationId: "installation-publish"},
		},
	}
}

func executionForTest(id string, status plansv1.PlanExecutionStatus) *plansv1.PlanExecution {
	return &plansv1.PlanExecution{
		Id:                   id,
		PlanConfigurationId:  "configuration-1",
		Status:               status,
		PlanTemplateSnapshot: executionTemplate(),
		ActiveStepKeys:       []string{"draft", "publish"},
		StepExecutions: []*plansv1.StepExecution{
			{Id: "step-draft", PlanExecutionId: id, PlanStepKey: "draft", Status: plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_COMPLETED, OutputArtifactRef: &artifactsv1.ArtifactRef{ArtifactId: "artifact-draft", ArtifactVersionId: "version-draft"}},
			{Id: "step-publish", PlanExecutionId: id, PlanStepKey: "publish", Status: plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_RUNNING},
		},
	}
}

func failedRunningExecution() *plansv1.PlanExecution {
	execution := executionForTest("exec-failing", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	execution.StepExecutions[1].Status = plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_FAILED
	return execution
}

func failedExecution() *plansv1.PlanExecution {
	execution := executionForTest("exec-failed", plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_FAILED)
	execution.StepExecutions[1].Status = plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_FAILED
	return execution
}

func pendingInteraction(executionID, stepExecutionID, stepKey string, kind chatv1.ExecutionInteractionKind, requestID, artifactVersionID string) planassistant.PendingInteraction {
	pending := planassistant.PendingInteraction{
		Kind:            kind,
		RequestID:       requestID,
		PlanExecutionID: executionID,
		StepExecutionID: stepExecutionID,
		PlanStepKey:     stepKey,
		Actions: []*chatv1.ExecutionPromptAction{
			{ActionId: kind.String() + ".respond", LabelKey: "execution.action.respond"},
		},
	}
	if artifactVersionID != "" {
		pending.SubjectArtifactRef = &artifactsv1.ArtifactRef{ArtifactId: "artifact-subject", ArtifactVersionId: artifactVersionID}
	}
	return pending
}
