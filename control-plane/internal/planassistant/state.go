// Package planassistant owns the deterministic configuration-assistant
// state machine. See docs/superpowers/specs/2026-06-22-harpia-ux-m5-chat-config-design.md.
package planassistant

import (
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// StateKind identifies the assistant's position in the configuration flow.
type StateKind string

const (
	StateAwaitingTemplate StateKind = "AWAITING_TEMPLATE"
	StateBindingStep      StateKind = "BINDING_STEP"
	StateSetOverseer      StateKind = "SET_OVERSEER"
	StateSetPolicies      StateKind = "SET_POLICIES"
	StateConfirm          StateKind = "CONFIRM"
	// StateSaved is the post-save acknowledgment. NEVER returned by
	// DeriveState — emitted only by Controller.handleSavedAck immediately
	// after the user clicks Save on a CONFIRM prompt. Per spec §2.2 the
	// assistant state is orthogonal to PlanConfiguration.status.
	StateSaved StateKind = "SAVED"
)

// AssistantState is the derived state for a single PlanConfiguration.
// StepKey is the step the current state applies to (empty for non-per-step states).
type AssistantState struct {
	Kind    StateKind
	StepKey string
}

// DeriveState is a pure function over (template, configuration, messages)
// that returns the assistant's next state. Messages are accepted for
// future use (e.g., detecting in-progress rewind) but are unused in v1
// derivation — the configuration alone is authoritative.
//
// Per spec §2.2: status (DRAFT/RUNNABLE/SCHEDULED) is ORTHOGONAL to the
// returned state. After an edit on a RUNNABLE configuration, derivation
// re-lands at CONFIRM so the user re-confirms. SAVED is never returned
// here — see Controller.NextTurn for the one-shot ack.
func DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState {
	_ = messages
	if config == nil || config.GetPlanTemplateId() == "" {
		return AssistantState{Kind: StateAwaitingTemplate}
	}

	steps := template.GetSteps()

	// 1. Walk steps in order; first unbound step → BINDING_STEP.
	bindings := map[string]string{}
	for _, sb := range config.GetSlotBindings() {
		if sb.GetExecutorInstallationId() != "" {
			bindings[sb.GetStepKey()] = sb.GetExecutorInstallationId()
		}
	}
	for _, step := range steps {
		if _, ok := bindings[step.GetKey()]; !ok {
			return AssistantState{Kind: StateBindingStep, StepKey: step.GetKey()}
		}
	}

	// 2. All bound — first step without an overseer → SET_OVERSEER.
	overseers := map[string]string{}
	for _, ob := range config.GetOverseerBindings() {
		if ob.GetOverseerUserId() != "" {
			overseers[ob.GetStepKey()] = ob.GetOverseerUserId()
		}
	}
	for _, step := range steps {
		if _, ok := overseers[step.GetKey()]; !ok {
			return AssistantState{Kind: StateSetOverseer, StepKey: step.GetKey()}
		}
	}

	// 3. Policies unset → SET_POLICIES.
	if !policiesSet(config.GetBehaviorPolicies()) {
		return AssistantState{Kind: StateSetPolicies}
	}

	// 4. Everything bound and policies set, status still DRAFT → CONFIRM.
	return AssistantState{Kind: StateConfirm}
}

func policiesSet(p *plansv1.PlanBehaviorPolicies) bool {
	if p == nil {
		return false
	}
	return p.GetElicitationTimeoutBehavior() != plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED &&
		p.GetPublishApprovalMode() != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED
}
