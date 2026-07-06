// Package planassistant owns the deterministic configuration-assistant
// state machine. See docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md.
package planassistant

import (
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// StateKind identifies the assistant's position in the configuration flow.
type StateKind string

const (
	StateAwaitingTemplate StateKind = "AWAITING_TEMPLATE"
	// StateBindingStep asks for one unbound PlanStep's SlotBinding at a time.
	StateBindingStep StateKind = "BINDING_STEP"
	// StateBindingMatrix is the review/save gate once all PlanSteps have an
	// executor SlotBinding but the configuration is still DRAFT.
	StateBindingMatrix StateKind = "BINDING_MATRIX"
	// StateOverseerStep asks who oversees one agent-backed PlanStep at a time.
	StateOverseerStep StateKind = "OVERSEER_STEP"
	// StatePoliciesStep asks for required PlanBehaviorPolicies before review.
	StatePoliciesStep StateKind = "POLICIES_STEP"
	// StateSaved is returned once status leaves DRAFT (the matrix card's
	// Save button promoted it). The controller emits the LandingCard.
	StateSaved StateKind = "SAVED"
)

// AssistantState is the derived state for a single PlanConfiguration.
type AssistantState struct {
	Kind      StateKind
	StepKey   string
	PolicyKey string
}

// DeriveState is a pure function over (template, configuration, messages)
// returning the assistant's current state. Messages are accepted for future
// use (e.g., detecting in-progress rewind) but are unused in v1 derivation —
// the configuration alone is authoritative.
func DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState {
	_ = messages
	if config == nil || config.GetPlanTemplateId() == "" {
		return AssistantState{Kind: StateAwaitingTemplate}
	}

	switch config.GetStatus() {
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED:
		if stepKey := firstUnboundStepKey(template, config); stepKey != "" {
			return AssistantState{Kind: StateBindingStep, StepKey: stepKey}
		}
		if stepKey := firstUnboundOverseerStepKey(template, config); stepKey != "" {
			return AssistantState{Kind: StateOverseerStep, StepKey: stepKey}
		}
		if policyKey := firstUnsetPolicyKey(config.GetBehaviorPolicies()); policyKey != "" {
			return AssistantState{Kind: StatePoliciesStep, PolicyKey: policyKey}
		}
		return AssistantState{Kind: StateBindingMatrix}
	default:
		// RUNNABLE / SCHEDULED / DISABLED / ARCHIVED — the Save button fired
		// and promoted the configuration. Next assistant turn is the landing.
		return AssistantState{Kind: StateSaved}
	}
}

func firstUnsetPolicyKey(p *plansv1.PlanBehaviorPolicies) string {
	if p == nil || p.GetPublishApprovalMode() == plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED {
		return "publish_approval_mode"
	}
	if p.GetElicitationTimeoutBehavior() == plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED {
		return "elicitation_timeout_behavior"
	}
	return ""
}

func firstUnboundStepKey(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration) string {
	if template == nil || config == nil {
		return ""
	}
	bound := make(map[string]bool, len(config.GetSlotBindings()))
	for _, sb := range config.GetSlotBindings() {
		if sb.GetStepKey() != "" && sb.GetExecutorInstallationId() != "" {
			bound[sb.GetStepKey()] = true
		}
	}
	for _, step := range template.GetSteps() {
		if step.GetKey() != "" && !bound[step.GetKey()] {
			return step.GetKey()
		}
	}
	return ""
}

func firstUnboundOverseerStepKey(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration) string {
	if template == nil || config == nil {
		return ""
	}
	bound := make(map[string]bool, len(config.GetOverseerBindings()))
	for _, ob := range config.GetOverseerBindings() {
		if ob.GetStepKey() != "" && ob.GetOverseerUserId() != "" {
			bound[ob.GetStepKey()] = true
		}
	}
	for _, step := range template.GetSteps() {
		if step.GetKey() != "" && isAgentBackedStep(step) && !bound[step.GetKey()] {
			return step.GetKey()
		}
	}
	return ""
}

func requiredOverseerStepKeys(template *plansv1.PlanTemplate) []string {
	if template == nil {
		return nil
	}
	keys := make([]string, 0, len(template.GetSteps()))
	for _, step := range template.GetSteps() {
		if step.GetKey() != "" && isAgentBackedStep(step) {
			keys = append(keys, step.GetKey())
		}
	}
	return keys
}

func isAgentBackedStep(step *plansv1.PlanStep) bool {
	return step.GetExecutorRequirement().GetExecutorKind() == plansv1.ExecutorKind_EXECUTOR_KIND_AGENT
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
