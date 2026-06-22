package planassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

// ExecutorCatalog returns the candidate executors for a step.
type ExecutorCatalog interface {
	CandidatesForStep(ctx context.Context, tenantID uuid.UUID, template *plansv1.PlanTemplate, stepKey string) ([]ExecutorOption, error)
}

// ConfigurationStore reads and writes PlanConfigurations from the controller's
// perspective. UpdateFromSelection applies the user's selection to the
// stored configuration (binding / overseer / policies / status) and returns
// the new state.
type ConfigurationStore interface {
	GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*plansv1.PlanConfiguration, error)
	UpdateFromSelection(ctx context.Context, tenantID, configID uuid.UUID, state AssistantState, value string) (*plansv1.PlanConfiguration, error)
}

// TemplateStore returns a PlanTemplate by ID.
type TemplateStore interface {
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*plansv1.PlanTemplate, error)
}

// Controller orchestrates the assistant: it derives state, builds the next
// prompt, and writes chat messages via the injected chat.Store.
type Controller struct {
	Chat      chat.Store
	Catalog   ExecutorCatalog
	Configs   ConfigurationStore
	Templates TemplateStore
}

// SeedThread is called once when a PlanConfiguration is first created.
// It writes CONFIGURATION_STARTED followed by the first ASSISTANT_PROMPT.
func (c *Controller) SeedThread(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("planassistant: load configuration: %w", err)
	}
	if cfg == nil {
		return errors.New("planassistant: configuration not found")
	}
	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    cfg.GetId(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED,
		Text:        "Let's set up your plan.",
		PayloadJSON: chat.BuildConfigurationStartedPayload(cfg.GetPlanTemplateId()),
	}); err != nil {
		return fmt.Errorf("planassistant: emit CONFIGURATION_STARTED: %w", err)
	}
	return c.emitCurrentPrompt(ctx, tenantID, cfg)
}

// NextTurn is called after a USER_SELECTION has been appended (by the
// AppendPlanThreadMessage handler). It:
//  1. Loads the latest USER_SELECTION and the ASSISTANT_PROMPT it answers.
//  2. Parses the answered prompt's state from its payload_json.
//  3. Calls Configs.UpdateFromSelection(state, value) — server-authoritative
//     mutation of binding / overseer / policies / status (see Task 7).
//  4. Re-loads the configuration, derives the new state, emits the next
//     ASSISTANT_PROMPT (or ASSISTANT_TEXT for SAVED).
//
// Best-effort & idempotent: if no recent USER_SELECTION is found, NextTurn
// just emits the prompt for the current derived state.
func (c *Controller) NextTurn(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("planassistant: load configuration: %w", err)
	}
	state, value, hasSel := c.findSelectionAndState(ctx, tenantID, cfg)
	if hasSel {
		mutated, err := c.Configs.UpdateFromSelection(ctx, tenantID, configID, state, value)
		if err != nil {
			return fmt.Errorf("planassistant: apply selection: %w", err)
		}
		if mutated != nil {
			cfg = mutated
		}
	}
	if hasSel && state.Kind == StateConfirm && value == "save" {
		switch cfg.GetStatus() {
		case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
			plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED:
			return c.emitSavedAck(ctx, tenantID, cfg)
		}
	}
	return c.emitCurrentPrompt(ctx, tenantID, cfg)
}

func (c *Controller) emitSavedAck(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration) error {
	text, payload := BuildPrompt(AssistantState{Kind: StateSaved}, PromptInput{Config: cfg})
	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    cfg.GetId(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_TEXT,
		Text:        text,
		PayloadJSON: payload,
	}); err != nil {
		return fmt.Errorf("planassistant: emit saved ack: %w", err)
	}
	return nil
}

func (c *Controller) findSelectionAndState(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration) (AssistantState, string, bool) {
	const lookback = 50
	msgs, err := c.Chat.ListMessages(ctx, tenantID, cfg.GetId(), 0, lookback)
	if err != nil || len(msgs) == 0 {
		return AssistantState{}, "", false
	}
	var sel *chatv1.ThreadMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION {
			sel = msgs[i]
			break
		}
	}
	if sel == nil {
		return AssistantState{}, "", false
	}
	var selPayload struct {
		InResponseToMessageID string `json:"in_response_to_message_id"`
		Value                 string `json:"value"`
	}
	if err := json.Unmarshal([]byte(sel.GetPayloadJson()), &selPayload); err != nil {
		return AssistantState{}, "", false
	}
	var prompt *chatv1.ThreadMessage
	for _, m := range msgs {
		if m.GetId() == selPayload.InResponseToMessageID {
			prompt = m
			break
		}
	}
	if prompt == nil {
		return AssistantState{}, "", false
	}
	var promptPayload struct {
		State   string `json:"state"`
		StepKey string `json:"step_key"`
	}
	if err := json.Unmarshal([]byte(prompt.GetPayloadJson()), &promptPayload); err != nil {
		return AssistantState{}, "", false
	}
	return AssistantState{Kind: StateKind(promptPayload.State), StepKey: promptPayload.StepKey}, selPayload.Value, true
}

func (c *Controller) emitCurrentPrompt(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration) error {
	tplID, err := uuid.Parse(cfg.GetPlanTemplateId())
	if err != nil {
		return fmt.Errorf("planassistant: parse template id: %w", err)
	}
	tpl, err := c.Templates.GetTemplateByID(ctx, tplID)
	if err != nil {
		return fmt.Errorf("planassistant: load template: %w", err)
	}
	state := DeriveState(tpl, cfg, nil)
	in := PromptInput{Template: tpl, Config: cfg}
	if state.Kind == StateBindingStep {
		cands, err := c.Catalog.CandidatesForStep(ctx, tenantID, tpl, state.StepKey)
		if err != nil {
			return fmt.Errorf("planassistant: load candidates: %w", err)
		}
		in.CandidateExecutors = cands
	}
	text, payload := BuildPrompt(state, in)
	kind := chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT
	if state.Kind == StateSaved {
		kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_TEXT
	}
	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    cfg.GetId(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        kind,
		Text:        text,
		PayloadJSON: payload,
	}); err != nil {
		return fmt.Errorf("planassistant: emit prompt: %w", err)
	}
	return nil
}
