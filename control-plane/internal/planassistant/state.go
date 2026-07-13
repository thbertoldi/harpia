// Package planassistant owns the deterministic configuration-assistant
// state machine. See docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md.
package planassistant

import (
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/planrules"
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
)

// ConfigurationState is the derived state for a DRAFT PlanConfiguration.
type ConfigurationState struct {
	Kind      StateKind
	StepKey   string
	PolicyKey string
}

// DeriveConfigurationState is a pure reducer for a DRAFT PlanConfiguration.
// Configuration lifecycle states after DRAFT are conversation-level concerns;
// the zero state signals that this reducer has no configuration prompt to emit.
func DeriveConfigurationState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration) ConfigurationState {
	if config == nil {
		return ConfigurationState{}
	}
	if config.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT &&
		config.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED {
		return ConfigurationState{}
	}
	if config.GetPlanTemplateId() == "" {
		return ConfigurationState{Kind: StateAwaitingTemplate}
	}

	if stepKey := firstUnboundStepKey(template, config); stepKey != "" {
		return ConfigurationState{Kind: StateBindingStep, StepKey: stepKey}
	}
	if stepKey := firstUnboundOverseerStepKey(template, config); stepKey != "" {
		return ConfigurationState{Kind: StateOverseerStep, StepKey: stepKey}
	}
	if policyKey := firstUnsetPolicyKey(template, config.GetBehaviorPolicies()); policyKey != "" {
		return ConfigurationState{Kind: StatePoliciesStep, PolicyKey: policyKey}
	}
	return ConfigurationState{Kind: StateBindingMatrix}
}

// orderedPolicyKeys is the stable order behavior policies are prompted in.
var orderedPolicyKeys = []string{"content_output_format", "publish_approval_mode", "elicitation_timeout_behavior"}

// templateDeclaresPolicy reports whether the template has an input parameter
// mapped to the given behavior policy. Only declared policies are prompted for:
// a template that never publishes (e.g. news-digest-draft) declares no
// publish_approval_mode parameter, so the assistant must not ask for it —
// otherwise the selection has nowhere to materialize and the journey stalls.
func templateDeclaresPolicy(template *plansv1.PlanTemplate, policyKey string) bool {
	for _, parameter := range template.GetInputParameters() {
		for _, mapping := range parameter.GetRuntimeMappings() {
			if mapping.GetTarget() == plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY &&
				mapping.GetPolicyKey() == policyKey {
				return true
			}
		}
	}
	return false
}

// policyUnset reports whether the given behavior policy still holds its
// unspecified zero value on the configuration.
func policyUnset(p *plansv1.PlanBehaviorPolicies, policyKey string) bool {
	switch policyKey {
	case "content_output_format":
		return p.GetContentOutputFormat() == plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED
	case "publish_approval_mode":
		return p.GetPublishApprovalMode() == plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED
	case "elicitation_timeout_behavior":
		return p.GetElicitationTimeoutBehavior() == plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED
	default:
		return false
	}
}

func firstUnsetPolicyKey(template *plansv1.PlanTemplate, p *plansv1.PlanBehaviorPolicies) string {
	for _, key := range orderedPolicyKeys {
		if templateDeclaresPolicy(template, key) && policyUnset(p, key) {
			return key
		}
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
		if step.GetKey() == "" {
			continue
		}
		// Opt-out steps are excluded from the run (ADR-018 D4) and need no
		// SlotBinding, so they never block configuration from reaching RUNNABLE.
		if !planrules.StepWillRun(step, config.GetIncludedOptionalCapabilities()) {
			continue
		}
		if !bound[step.GetKey()] {
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
		if step.GetKey() == "" || !isAgentBackedStep(step) {
			continue
		}
		// An opt-out step runs no executor and so needs no overseer.
		if !planrules.StepWillRun(step, config.GetIncludedOptionalCapabilities()) {
			continue
		}
		if !bound[step.GetKey()] {
			return step.GetKey()
		}
	}
	return ""
}

func requiredOverseerStepKeys(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration) []string {
	if template == nil {
		return nil
	}
	keys := make([]string, 0, len(template.GetSteps()))
	for _, step := range template.GetSteps() {
		if step.GetKey() == "" || !isAgentBackedStep(step) {
			continue
		}
		// Opt-out steps are excluded from the run and so demand no overseer.
		if !planrules.StepWillRun(step, config.GetIncludedOptionalCapabilities()) {
			continue
		}
		keys = append(keys, step.GetKey())
	}
	return keys
}

func isAgentBackedStep(step *plansv1.PlanStep) bool {
	return step.GetExecutorRequirement().GetExecutorKind() == plansv1.ExecutorKind_EXECUTOR_KIND_AGENT
}

// policiesSet reports whether every behavior policy the template declares is
// set. Consumed by prompts.go to gate the matrix card's Save button. A template
// that declares no behavior policies is trivially satisfied.
func policiesSet(template *plansv1.PlanTemplate, p *plansv1.PlanBehaviorPolicies) bool {
	for _, key := range orderedPolicyKeys {
		if templateDeclaresPolicy(template, key) && policyUnset(p, key) {
			return false
		}
	}
	return true
}
