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
	"github.com/harpia/control-plane/internal/identity"
)

// ExecutorCatalog returns the candidate executors for a step.
type ExecutorCatalog interface {
	CandidatesForStep(ctx context.Context, tenantID uuid.UUID, template *plansv1.PlanTemplate, stepKey string) ([]ExecutorOption, error)
}

// ConfigurationStore reads PlanConfigurations from the controller's
// perspective. The card mutates bindings/overseer/policies/status directly
// via the plans handler's UpdatePlanConfiguration — the controller reads
// current state and emits the matching prompt.
type ConfigurationStore interface {
	GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*plansv1.PlanConfiguration, error)
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

// SeedThread is called once when a PlanConfiguration is first created. It
// writes CONFIGURATION_STARTED followed by the first ASSISTANT_PROMPT.
func (c *Controller) SeedThread(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("planassistant: load configuration: %w", err)
	}
	if cfg == nil {
		return errors.New("planassistant: configuration not found")
	}
	threadID := cfg.GetThreadId()
	if threadID == "" {
		return errors.New("planassistant: configuration has no owning thread")
	}
	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    threadID,
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED,
		Text:        "Let's set up your plan.",
		PayloadJSON: chat.BuildConfigurationStartedPayload(cfg.GetPlanTemplateId()),
	}); err != nil {
		return fmt.Errorf("planassistant: emit CONFIGURATION_STARTED: %w", err)
	}
	return c.emitCurrentPrompt(ctx, tenantID, cfg)
}

// NextTurn is called after the configuration changes. It re-derives the state
// from the current configuration and emits the matching prompt.
//
// Idempotent: emitting the same matrix payload twice is suppressed, and the
// LandingCard is emitted at most once per configuration.
func (c *Controller) NextTurn(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("planassistant: load configuration: %w", err)
	}
	if cfg == nil {
		return errors.New("planassistant: configuration not found")
	}
	return c.emitCurrentPrompt(ctx, tenantID, cfg)
}

func (c *Controller) emitCurrentPrompt(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration) error {
	threadID := cfg.GetThreadId()
	if threadID == "" {
		return errors.New("planassistant: configuration has no owning thread")
	}
	tplID, err := uuid.Parse(cfg.GetPlanTemplateId())
	if err != nil {
		return fmt.Errorf("planassistant: parse template id: %w", err)
	}
	tpl, err := c.Templates.GetTemplateByID(ctx, tplID)
	if err != nil {
		return fmt.Errorf("planassistant: load template: %w", err)
	}

	state := DeriveState(tpl, cfg, nil)

	if state.Kind == StateSaved {
		emitted, err := c.landingAlreadyEmitted(ctx, tenantID, threadID)
		if err != nil {
			return err
		}
		if emitted {
			return nil
		}
	}

	in := PromptInput{Template: tpl, Config: cfg}
	if rc, ok := identity.RequestContextFrom(ctx); ok {
		in.CurrentUserID = rc.UserID
		// The identity context carries only the user id (no display name), so
		// use a human label here; the frontend overrides it with the real name
		// when the session user is available. Never emit the raw id as a label.
		in.CurrentUserLabel = "You"
	}
	if state.Kind == StateBindingStep || state.Kind == StateOverseerStep || state.Kind == StateBindingMatrix {
		byStep := make(map[string][]ExecutorOption, len(tpl.GetSteps()))
		for _, step := range tpl.GetSteps() {
			cands, err := c.Catalog.CandidatesForStep(ctx, tenantID, tpl, step.GetKey())
			if err != nil {
				return fmt.Errorf("planassistant: load candidates for %s: %w", step.GetKey(), err)
			}
			byStep[step.GetKey()] = cands
		}
		in.CandidatesByStep = byStep
	}

	text, payload := BuildPrompt(state, in)

	// Suppress duplicate prompts. NextTurn can fire on incremental edits, but
	// we only want a fresh prompt when the rendered payload actually differs
	// from the most recent assistant prompt.
	if dup, err := c.isDuplicateAssistantPrompt(ctx, tenantID, threadID, payload); err != nil {
		return err
	} else if dup {
		return nil
	}

	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    threadID,
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
		Text:        text,
		PayloadJSON: payload,
	}); err != nil {
		return fmt.Errorf("planassistant: emit prompt: %w", err)
	}
	return nil
}

// landingAlreadyEmitted reports whether an ASSISTANT_PROMPT with state
// "landing" has already been written to the thread.
func (c *Controller) landingAlreadyEmitted(ctx context.Context, tenantID uuid.UUID, threadID string) (bool, error) {
	msgs, err := c.Chat.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		return false, fmt.Errorf("planassistant: list messages: %w", err)
	}
	for _, m := range msgs {
		if m.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			continue
		}
		if promptState(m.GetPayloadJson()) == "landing" {
			return true, nil
		}
	}
	return false, nil
}

// isDuplicateAssistantPrompt reports whether the most recent ASSISTANT_PROMPT
// already carries the given payload (so re-emitting would be a no-op).
func (c *Controller) isDuplicateAssistantPrompt(ctx context.Context, tenantID uuid.UUID, threadID, payload string) (bool, error) {
	msgs, err := c.Chat.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		return false, fmt.Errorf("planassistant: list messages: %w", err)
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			return msgs[i].GetPayloadJson() == payload, nil
		}
	}
	return false, nil
}

func promptState(payloadJSON string) string {
	var p struct {
		State string `json:"state"`
	}
	if json.Unmarshal([]byte(payloadJSON), &p) != nil {
		return ""
	}
	return p.State
}
