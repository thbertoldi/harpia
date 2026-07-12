package plans

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/workflow"
)

func TestRuntimeInteractionEventsUseConfigurationOriginThread(t *testing.T) {
	tenantID := uuid.New()
	configurationID := uuid.New()
	executionID := uuid.New()
	stepID := uuid.New()
	originThreadID := uuid.New()
	store := &fakeRuntimePlanStore{
		configuration: &PlanConfiguration{
			ID:             configurationID,
			TenantID:       tenantID,
			OriginThreadID: originThreadID,
		},
		execution: &PlanExecution{
			ID:                  executionID,
			TenantID:            tenantID,
			PlanConfigurationID: configurationID,
			Status:              ExecutionStatusRunning,
		},
		stepID: stepID,
	}
	messages := &fakeRuntimeChat{}
	runtime := &RuntimeRepository{plans: store, chat: messages}

	_, err := runtime.CreateStepExecution(context.Background(), workflow.CreateStepExecutionInput{
		TenantID:                     tenantID.String(),
		PlanExecutionID:              executionID.String(),
		PlanStepKey:                  "publish",
		ExecutorInstallationSnapshot: workflow.ExecutorInstallationSnapshot{ID: "installation-publish"},
	})
	if err != nil {
		t.Fatalf("CreateStepExecution: %v", err)
	}
	if err := runtime.CreateApprovalRequest(context.Background(), workflow.CreateApprovalRequestInput{
		TenantID:          tenantID.String(),
		PlanExecutionID:   executionID.String(),
		StepExecutionID:   stepID.String(),
		PlanStepKey:       "publish",
		ApprovalRequestID: "approval-1",
		InputArtifactID:   "artifact-1",
	}); err != nil {
		t.Fatalf("CreateApprovalRequest: %v", err)
	}

	if len(messages.appended) != 2 {
		t.Fatalf("appended messages = %d, want 2", len(messages.appended))
	}
	wantThreadID := originThreadID.String()
	for _, input := range messages.appended {
		if input.ThreadID != wantThreadID {
			t.Fatalf("message kind %s thread = %q, want %q", input.Kind, input.ThreadID, wantThreadID)
		}
	}
	if messages.appended[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_STARTED {
		t.Fatalf("first message kind = %s, want STEP_STARTED", messages.appended[0].Kind)
	}
	if messages.appended[1].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_APPROVAL_RAISED {
		t.Fatalf("second message kind = %s, want APPROVAL_RAISED", messages.appended[1].Kind)
	}
}

func TestFailStepExecutionEmitsStepFailedChatMessage(t *testing.T) {
	tenantID := uuid.New()
	configurationID := uuid.New()
	executionID := uuid.New()
	stepID := uuid.New()
	originThreadID := uuid.New()
	store := &fakeRuntimePlanStore{
		configuration: &PlanConfiguration{
			ID:             configurationID,
			TenantID:       tenantID,
			OriginThreadID: originThreadID,
		},
		execution: &PlanExecution{
			ID:                  executionID,
			TenantID:            tenantID,
			PlanConfigurationID: configurationID,
			Status:              ExecutionStatusRunning,
		},
		stepID: stepID,
	}
	messages := &fakeRuntimeChat{}
	runtime := &RuntimeRepository{plans: store, chat: messages}

	if err := runtime.FailStepExecution(context.Background(), workflow.StepStatusUpdateInput{
		TenantID:        tenantID.String(),
		StepExecutionID: stepID.String(),
	}); err != nil {
		t.Fatalf("FailStepExecution: %v", err)
	}

	if len(messages.appended) != 1 {
		t.Fatalf("appended messages = %d, want 1", len(messages.appended))
	}
	msg := messages.appended[0]
	if msg.Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_FAILED {
		t.Fatalf("message kind = %s, want STEP_FAILED", msg.Kind)
	}
	if msg.ThreadID != originThreadID.String() {
		t.Fatalf("message thread = %q, want %q", msg.ThreadID, originThreadID.String())
	}
	if !strings.Contains(msg.Text, "publish") {
		t.Fatalf("message text = %q, want it to contain the step key", msg.Text)
	}
}

type fakeRuntimePlanStore struct {
	configuration *PlanConfiguration
	execution     *PlanExecution
	stepID        uuid.UUID
}

func (f *fakeRuntimePlanStore) GetConfiguration(_ context.Context, _, _ uuid.UUID) (*PlanConfiguration, error) {
	return f.configuration, nil
}

func (f *fakeRuntimePlanStore) GetTemplateByID(context.Context, uuid.UUID) (*PlanTemplate, error) {
	return &PlanTemplate{}, nil
}

func (f *fakeRuntimePlanStore) CreateExecution(_ context.Context, execution *PlanExecution) (*PlanExecution, error) {
	return execution, nil
}

func (f *fakeRuntimePlanStore) GetExecution(context.Context, uuid.UUID, uuid.UUID) (*PlanExecution, error) {
	return f.execution, nil
}

func (f *fakeRuntimePlanStore) UpdateExecutionStatus(context.Context, uuid.UUID, uuid.UUID, string, *time.Time) error {
	return nil
}

func (f *fakeRuntimePlanStore) GetPlanConfigurationIDForExecution(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return f.configuration.ID, nil
}

func (f *fakeRuntimePlanStore) CreateStepExecution(_ context.Context, step *StepExecution) (*StepExecution, error) {
	created := *step
	created.ID = f.stepID
	created.Attempt = 1
	return &created, nil
}

func (f *fakeRuntimePlanStore) GetStepExecution(_ context.Context, _ uuid.UUID, stepID uuid.UUID) (*StepExecution, error) {
	return &StepExecution{
		ID:               stepID,
		PlanExecutionID:  f.execution.ID,
		PlanStepKey:      "publish",
		Status:           StepStatusFailed,
	}, nil
}

func (f *fakeRuntimePlanStore) UpdateStepExecutionStatus(context.Context, uuid.UUID, uuid.UUID, string, string, string, string) error {
	return nil
}

func (f *fakeRuntimePlanStore) UpsertElicitation(_ context.Context, elicitation *Elicitation) (*Elicitation, error) {
	return elicitation, nil
}

func (f *fakeRuntimePlanStore) MarkElicitationTimedOutByStep(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}

func (f *fakeRuntimePlanStore) CreatePlanApprovalRequest(context.Context, *PlanApprovalRequest) error {
	return nil
}

func (f *fakeRuntimePlanStore) ResolvePlanApprovalRequest(context.Context, uuid.UUID, string, uuid.UUID, bool, string) error {
	return nil
}

type fakeRuntimeChat struct {
	appended []chat.AppendInput
}

func (f *fakeRuntimeChat) AppendMessage(_ context.Context, _ uuid.UUID, input chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, input)
	return &chatv1.ThreadMessage{Id: uuid.NewString(), ThreadId: input.ThreadID, Kind: input.Kind, PayloadJson: input.PayloadJSON}, nil
}

func (f *fakeRuntimeChat) ListMessages(_ context.Context, _ uuid.UUID, threadID string, _ int64, _ int) ([]*chatv1.ThreadMessage, error) {
	messages := make([]*chatv1.ThreadMessage, 0, len(f.appended))
	for _, input := range f.appended {
		if input.ThreadID != threadID {
			continue
		}
		messages = append(messages, &chatv1.ThreadMessage{ThreadId: input.ThreadID, Kind: input.Kind, PayloadJson: input.PayloadJSON})
	}
	return messages, nil
}

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

func TestBuildRetryPlanWorkflowInputToleratesSkippedUpstreamSteps(t *testing.T) {
	executionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	failedStepID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	now := time.Now().UTC()
	snapshot := workflow.PlanExecutionSnapshot{
		SchemaVersion: 1,
		Configuration: &plansv1.PlanConfiguration{
			Id:     "cfg-1",
			Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		},
		Template: &plansv1.PlanTemplate{
			Key: "retry-template-skipped",
			Steps: []*plansv1.PlanStep{
				{Key: "skipped-step", OutputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft"},
				{Key: "completed-step", OutputArtifactTypeId: "harpia.artifacts.v1.TextDraft"},
				{Key: "failed-step", OutputArtifactTypeId: "harpia.artifacts.v1.NewsList"},
			},
			Edges: []*plansv1.PlanStepDependency{
				{FromStepKey: "skipped-step", ToStepKey: "completed-step"},
				{FromStepKey: "completed-step", ToStepKey: "failed-step"},
			},
		},
		ExecutorInstallations: map[string]workflow.ExecutorInstallationSnapshot{
			"skipped-step":   {ID: "install-skipped", Kind: workflow.ExecutorKindAgent},
			"completed-step": {ID: "install-completed", Kind: workflow.ExecutorKindAgent},
			"failed-step":    {ID: "install-failed", Kind: workflow.ExecutorKindIntegration},
		},
	}
	result, err := buildRetryPlanWorkflowInput(snapshot, &PlanExecution{
		ID:     executionID,
		Status: ExecutionStatusFailed,
		StepExecutions: []StepExecution{
			{
				ID:               uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				PlanExecutionID:  executionID,
				PlanStepKey:      "skipped-step",
				Status:           StepStatusSkipped,
				OutputArtifactID: "", // skipped steps have no artifact
				Attempt:          1,
				CreatedAt:        now,
			},
			{
				ID:               uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				PlanExecutionID:  executionID,
				PlanStepKey:      "completed-step",
				Status:           StepStatusCompleted,
				OutputArtifactID: "artifact-completed-step",
				Attempt:          1,
				CreatedAt:        now.Add(time.Minute),
			},
			{
				ID:               failedStepID,
				PlanExecutionID:  executionID,
				PlanStepKey:      "failed-step",
				Status:           StepStatusFailed,
				Attempt:          1,
				CreatedAt:        now.Add(2 * time.Minute),
			},
		},
	}, failedStepID)
	if err != nil {
		t.Fatalf("build retry input: %v", err)
	}
	if result.retryFromStepKey != "failed-step" {
		t.Fatalf("retry from step = %q, want failed-step", result.retryFromStepKey)
	}
	// The skipped upstream must not appear in the reusable artifacts map, and
	// the builder must not error on its missing output artifact.
	if _, ok := result.reusedArtifactsByStep["skipped-step"]; ok {
		t.Fatal("skipped-step should not contribute a reusable artifact")
	}
	reused, ok := result.reusedArtifactsByStep["completed-step"]
	if !ok {
		t.Fatal("missing reused artifact for completed-step")
	}
	if reused.ArtifactID != "artifact-completed-step" {
		t.Fatalf("artifact id = %q, want artifact-completed-step", reused.ArtifactID)
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
