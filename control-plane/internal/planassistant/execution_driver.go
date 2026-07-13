package planassistant

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

// ExecutionProjection is the tenant-scoped durable state needed to derive an
// execution conversation turn. It deliberately contains generated contracts
// only: database and workflow types belong to the adapter, not this package.
type ExecutionProjection struct {
	Execution           *plansv1.PlanExecution
	PendingInteractions []PendingInteraction
}

// ExecutionStore reads execution state for conversation projection. Exact reads
// are configuration-scoped so a latest-execution display fallback can never be
// mistaken for command resolution.
type ExecutionStore interface {
	LoadExactExecution(ctx context.Context, tenantID, configurationID, executionID uuid.UUID) (*ExecutionProjection, error)
	LoadLatestNonTerminalExecution(ctx context.Context, tenantID, configurationID uuid.UUID) (*ExecutionProjection, error)
}

// OnExecutionEvent projects one durable execution transition into a typed
// EXECUTION_PROMPT. It is intentionally idempotent: Temporal activities may be
// retried and each event is reconstructible from the durable projection.
func (c *Controller) OnExecutionEvent(ctx context.Context, tenantID, executionID uuid.UUID, _ string) error {
	if c == nil || c.Executions == nil {
		return errors.New("planassistant: execution store is not configured")
	}
	if executionID == uuid.Nil {
		return errors.New("planassistant: execution id is required")
	}

	// The adapter enforces that the exact execution belongs to the selected
	// configuration and its owning thread. Load the configuration only after the
	// execution is found, so no event can borrow another configuration's state.
	projection, err := c.Executions.LoadExactExecution(ctx, tenantID, uuid.Nil, executionID)
	if err != nil {
		return fmt.Errorf("planassistant: load exact execution: %w", err)
	}
	if projection == nil || projection.Execution == nil {
		return errors.New("planassistant: execution not found")
	}
	configurationID, err := uuid.Parse(projection.Execution.GetPlanConfigurationId())
	if err != nil {
		return fmt.Errorf("planassistant: parse execution configuration id: %w", err)
	}
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configurationID)
	if err != nil {
		return fmt.Errorf("planassistant: load execution configuration: %w", err)
	}
	return c.emitExecutionTurn(ctx, tenantID, cfg, projection, projection.Execution.GetId())
}

func (c *Controller) emitExecutionTurn(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration, projection *ExecutionProjection, exactExecutionID string) error {
	if cfg == nil || cfg.GetOriginThreadId() == "" {
		return errors.New("planassistant: execution configuration has no owning thread")
	}
	if projection == nil || projection.Execution == nil {
		return errors.New("planassistant: execution projection is required")
	}
	turn, err := DeriveConversationTurn(TurnInput{
		Configuration:       cfg,
		Executions:          []*plansv1.PlanExecution{projection.Execution},
		Target:              ConversationTarget{ConfigurationID: cfg.GetId(), PlanExecutionID: exactExecutionID},
		PendingInteractions: projection.PendingInteractions,
	})
	if err != nil {
		return fmt.Errorf("planassistant: derive execution turn: %w", err)
	}
	if turn.Execution == nil {
		return nil
	}
	payload := executionPromptPayload(turn.Execution)
	if duplicate, err := c.isDuplicateExecutionPrompt(ctx, tenantID, cfg.GetOriginThreadId(), payload); err != nil {
		return err
	} else if duplicate {
		return nil
	}
	executionID, err := uuid.Parse(turn.Execution.PlanExecutionID)
	if err != nil {
		return fmt.Errorf("planassistant: parse execution id: %w", err)
	}
	_, err = c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    cfg.GetOriginThreadId(),
		ExecutionID: &executionID,
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_EXECUTION_PROMPT,
		PayloadJSON: chat.BuildExecutionPromptPayload(payload),
	})
	if err != nil {
		return fmt.Errorf("planassistant: emit execution prompt: %w", err)
	}
	return nil
}

func executionPromptPayload(view *ExecutionAssistantView) *chatv1.ExecutionPromptPayload {
	payload := &chatv1.ExecutionPromptPayload{
		ConfigurationId:           view.ConfigurationID,
		PlanExecutionId:           view.PlanExecutionID,
		State:                     view.State,
		StepExecutionId:           view.StepExecutionID,
		PlanStepKey:               view.PlanStepKey,
		CompletedStepCount:        view.CompletedStepCount,
		ActiveStepCount:           view.ActiveStepCount,
		LatestArtifactRef:         view.LatestArtifactRef,
		Actions:                   view.Actions,
		OtherActiveExecutionCount: view.OtherActiveExecutionCount,
	}
	if pending := view.PendingInteraction; pending != nil {
		payload.PendingInteraction = &chatv1.ExecutionInteractionPointer{
			Kind:               pending.Kind,
			RequestId:          pending.RequestID,
			StepExecutionId:    pending.StepExecutionID,
			PlanStepKey:        pending.PlanStepKey,
			SubjectArtifactRef: pending.SubjectArtifactRef,
		}
	}
	return payload
}

func (c *Controller) isDuplicateExecutionPrompt(ctx context.Context, tenantID uuid.UUID, threadID string, payload *chatv1.ExecutionPromptPayload) (bool, error) {
	msgs, err := c.Chat.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		return false, fmt.Errorf("planassistant: list execution prompts: %w", err)
	}
	want := executionPromptPayloadFingerprint(payload)
	if want == "" {
		return false, nil
	}
	for _, msg := range msgs {
		if msg.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_EXECUTION_PROMPT {
			continue
		}
		stored, ok := chat.ParseExecutionPromptPayload(msg.GetPayloadJson())
		if ok && executionPromptPayloadFingerprint(stored) == want {
			return true, nil
		}
	}
	return false, nil
}

func executionPromptPayloadFingerprint(payload *chatv1.ExecutionPromptPayload) string {
	if payload == nil || payload.GetPlanExecutionId() == "" || payload.GetState() == chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_UNSPECIFIED {
		return ""
	}
	interactionKind, requestID, subjectVersionID := chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_UNSPECIFIED.String(), "", ""
	if pending := payload.GetPendingInteraction(); pending != nil {
		interactionKind = pending.GetKind().String()
		requestID = pending.GetRequestId()
		subjectVersionID = pending.GetSubjectArtifactRef().GetArtifactVersionId()
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", payload.GetPlanExecutionId(), payload.GetState().String(), payload.GetStepExecutionId(), interactionKind, requestID, subjectVersionID)
}
