// Package planassistant owns the deterministic configuration-assistant
// state machine. See docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md.
package planassistant

import (
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// StateKind identifies the assistant's position in the configuration flow.
//
// M6 (spec §2.1) collapsed the per-step walk (BINDING_STEP / SET_OVERSEER /
// SET_POLICIES) and the CONFIRM gate into the single BINDING_MATRIX state.
// The matrix card is the configuration surface: it renders every step row,
// the overseer-per-row default, and the policy fields at once. Per-step
// assistant prompts no longer exist.
type StateKind string

const (
	StateAwaitingTemplate StateKind = "AWAITING_TEMPLATE"
	// StateBindingMatrix is the sole pre-SAVED state. Derivation returns it
	// whenever the configuration is still DRAFT — whatever is unbound or
	// unset, the matrix card surfaces it inline.
	StateBindingMatrix StateKind = "BINDING_MATRIX"
	// StateSaved is returned once status leaves DRAFT (the matrix card's
	// Save button promoted it). The controller emits the LandingCard.
	StateSaved StateKind = "SAVED"
)

// AssistantState is the derived state for a single PlanConfiguration.
// StepKey is retained in the struct for future per-step prompts but is
// unused (empty) in v1 — both BindingMatrix and Saved leave it empty.
type AssistantState struct {
	Kind    StateKind
	StepKey string
}

// DeriveState is a pure function over (template, configuration, messages)
// returning the assistant's current state. Messages are accepted for future
// use (e.g., detecting in-progress rewind) but are unused in v1 derivation —
// the configuration alone is authoritative.
//
// Per spec §2.1: the matrix card mutates bindings/overseer/policies directly
// via UpdatePlanConfiguration while status stays DRAFT. Once the Save button
// promotes status past DRAFT, derivation returns SAVED and the controller
// emits the LandingCard.
func DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState {
	_ = template
	_ = messages
	if config == nil || config.GetPlanTemplateId() == "" {
		return AssistantState{Kind: StateAwaitingTemplate}
	}

	switch config.GetStatus() {
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED:
		return AssistantState{Kind: StateBindingMatrix}
	default:
		// RUNNABLE / SCHEDULED / DISABLED / ARCHIVED — the Save button fired
		// and promoted the configuration. Next assistant turn is the landing.
		return AssistantState{Kind: StateSaved}
	}
}

// policiesSet reports whether both behavior-policy fields are set. Consumed by
// prompts.go to gate the matrix card's Save button.
func policiesSet(p *plansv1.PlanBehaviorPolicies) bool {
	if p == nil {
		return false
	}
	return p.GetElicitationTimeoutBehavior() != plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED &&
		p.GetPublishApprovalMode() != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED
}
