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
		// Landing idempotency is per-configuration: a landing for config A in
		// a shared thread must not suppress config B's landing.
		emitted, err := c.landingAlreadyEmitted(ctx, tenantID, threadID, cfg.GetId())
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
	// Stamp the owning configuration into every prompt payload so dedup can be
	// config-scoped — two configurations sharing one thread must not suppress
	// each other's prompts. BuildPrompt does not emit this field today; we
	// round-trip through a map so every prompt variant is covered uniformly.
	payload = stampConfigurationID(payload, cfg.GetId())

	// Suppress duplicate prompts semantically. NextTurn can fire on every
	// incremental edit, but two prompts with the same
	// (configuration_id, state, step_key) are the same turn to the user even
	// when the candidate/price/current-user-label body differs. Comparing raw
	// payload bytes is brittle and caused every binding/overseer/policies
	// prompt to be duplicated after a page reload.
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

// landingAlreadyEmitted reports whether an ASSISTANT_PROMPT whose payload
// carries the given configuration_id and state "landing" has already been
// written to the thread. Landing idempotency is per-configuration so two
// configurations sharing a thread do not suppress each other's landing.
func (c *Controller) landingAlreadyEmitted(ctx context.Context, tenantID uuid.UUID, threadID, configurationID string) (bool, error) {
	msgs, err := c.Chat.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		return false, fmt.Errorf("planassistant: list messages: %w", err)
	}
	for _, m := range msgs {
		if m.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			continue
		}
		cid, st, _ := promptFingerprint(m.GetPayloadJson())
		if cid == configurationID && st == "landing" {
			return true, nil
		}
	}
	return false, nil
}

// isDuplicateAssistantPrompt reports whether any stored ASSISTANT_PROMPT
// already carries the same semantic fingerprint (configuration_id, state,
// step_key) as the candidate payload. The full payload body may differ in
// non-semantic fields (candidate labels, price lookups, current-user label)
// without meaningfully changing the prompt, so byte comparison was too brittle
// and duplicated every prompt after a page reload.
func (c *Controller) isDuplicateAssistantPrompt(ctx context.Context, tenantID uuid.UUID, threadID, payload string) (bool, error) {
	msgs, err := c.Chat.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		return false, fmt.Errorf("planassistant: list messages: %w", err)
	}
	wantCid, wantState, wantStep := promptFingerprint(payload)
	for _, m := range msgs {
		if m.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			continue
		}
		cid, st, sk := promptFingerprint(m.GetPayloadJson())
		if cid == wantCid && st == wantState && sk == wantStep {
			return true, nil
		}
	}
	return false, nil
}

// promptFingerprint extracts the semantic identity of an ASSISTANT_PROMPT
// payload: the owning configuration, the assistant state, and (for step
// prompts) the focused step key. Two payloads with the same fingerprint are
// the same turn to the user. Missing fields compare as "".
func promptFingerprint(payloadJSON string) (configurationID, state, stepKey string) {
	var p struct {
		ConfigurationID string `json:"configuration_id"`
		State           string `json:"state"`
		StepKey         string `json:"step_key"`
	}
	if json.Unmarshal([]byte(payloadJSON), &p) != nil {
		return "", "", ""
	}
	return p.ConfigurationID, p.State, p.StepKey
}

// stampConfigurationID merges configuration_id into the prompt payload. The
// payload is opaque JSON built by BuildPrompt across several typed variants,
// so we round-trip it through a map rather than mutate every builder. If the
// payload already carries a configuration_id it is overwritten to guarantee
// the stamped value equals the owning configuration.
func stampConfigurationID(payload, configurationID string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		// Fall back to the raw payload rather than dropping a prompt entirely
		// over a serialization detail; dedup then degrades to byte comparison.
		return payload
	}
	if m == nil {
		m = map[string]any{}
	}
	m["configuration_id"] = configurationID
	out, err := json.Marshal(m)
	if err != nil {
		return payload
	}
	return string(out)
}
