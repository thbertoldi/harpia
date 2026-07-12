// Package planrules contains pure plan-domain predicates shared by adapters.
package planrules

import plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"

// StepWillRun reports whether a step participates in a configuration's run.
// A step with optional capabilities participates only when every optional
// capability is included by the configuration.
func StepWillRun(step *plansv1.PlanStep, includedOptionalCapabilities []string) bool {
	if step == nil {
		return true
	}

	optional := step.GetExecutorRequirement().GetOptionalCapabilities()
	if len(optional) == 0 {
		return true
	}

	included := make(map[string]struct{}, len(includedOptionalCapabilities))
	for _, capability := range includedOptionalCapabilities {
		included[capability] = struct{}{}
	}
	for _, capability := range optional {
		if _, ok := included[capability]; !ok {
			return false
		}
	}
	return true
}
