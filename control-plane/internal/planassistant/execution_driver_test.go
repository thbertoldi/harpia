package planassistant_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/planassistant"
)

type fakeExecutionStore struct {
	exact  *planassistant.ExecutionProjection
	latest *planassistant.ExecutionProjection
}

func (f *fakeExecutionStore) LoadExactExecution(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) (*planassistant.ExecutionProjection, error) {
	return f.exact, nil
}

func (f *fakeExecutionStore) LoadLatestNonTerminalExecution(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*planassistant.ExecutionProjection, error) {
	return f.latest, nil
}

func TestOnExecutionEventEmitsOneSemanticExecutionPrompt(t *testing.T) {
	tenantID := uuid.New()
	executionID := uuid.New()
	configurationID := uuid.New()
	threadID := uuid.New()
	configuration := &plansv1.PlanConfiguration{Id: configurationID.String(), OriginThreadId: threadID.String(), Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE}
	execution := executionForTest(executionID.String(), plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	execution.PlanConfigurationId = configurationID.String()
	pending := pendingInteraction(executionID.String(), "step-publish", "publish", chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW, "review-1", "version-1")
	store := &fakeExecutionStore{exact: &planassistant.ExecutionProjection{Execution: execution, PendingInteractions: []planassistant.PendingInteraction{pending}}}
	chatStore := &fakeChat{}
	controller := &planassistant.Controller{Chat: chatStore, Configs: &fakeConfigs{cur: configuration}, Executions: store}

	if err := controller.OnExecutionEvent(context.Background(), tenantID, executionID, "REVIEW_RAISED"); err != nil {
		t.Fatalf("OnExecutionEvent: %v", err)
	}
	if err := controller.OnExecutionEvent(context.Background(), tenantID, executionID, "REVIEW_RAISED"); err != nil {
		t.Fatalf("duplicate OnExecutionEvent: %v", err)
	}
	if got := len(chatStore.appended); got != 1 {
		t.Fatalf("execution prompt appends = %d, want 1", got)
	}
	message := chatStore.appended[0]
	if message.Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_EXECUTION_PROMPT {
		t.Fatalf("kind = %s, want EXECUTION_PROMPT", message.Kind)
	}
	payload, ok := chat.ParseExecutionPromptPayload(message.PayloadJSON)
	if !ok {
		t.Fatalf("invalid execution prompt payload: %s", message.PayloadJSON)
	}
	if payload.GetPlanExecutionId() != executionID.String() || payload.GetPendingInteraction().GetRequestId() != "review-1" || payload.GetPendingInteraction().GetSubjectArtifactRef().GetArtifactVersionId() != "version-1" {
		t.Fatalf("prompt did not preserve exact execution/request/version: %+v", payload)
	}
}

func TestNextTurnUsesLatestNonTerminalExecutionBeforeConfigurationReducer(t *testing.T) {
	tenantID := uuid.New()
	configurationID := uuid.New()
	threadID := uuid.New()
	executionID := uuid.New()
	configuration := &plansv1.PlanConfiguration{Id: configurationID.String(), OriginThreadId: threadID.String(), Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE}
	execution := executionForTest(executionID.String(), plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING)
	execution.PlanConfigurationId = configurationID.String()
	chatStore := &fakeChat{}
	controller := &planassistant.Controller{
		Chat:    chatStore,
		Configs: &fakeConfigs{cur: configuration},
		Executions: &fakeExecutionStore{latest: &planassistant.ExecutionProjection{
			Execution: execution,
		}},
	}

	if err := controller.NextTurn(context.Background(), tenantID, configurationID); err != nil {
		t.Fatalf("NextTurn: %v", err)
	}
	if got := len(chatStore.appended); got != 1 || chatStore.appended[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_EXECUTION_PROMPT {
		t.Fatalf("NextTurn must emit the execution turn before config fallback, got %+v", chatStore.appended)
	}
}
