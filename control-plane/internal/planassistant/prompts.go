package planassistant

import (
	"fmt"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

// ExecutorOption is one available executor installation for a step,
// loaded by the controller before calling BuildPrompt.
type ExecutorOption struct {
	StepKey        string
	InstallationID string
	DisplayName    string
	SkuKey         string
	Tier           string // "junior" / "senior" / "specialist" / ""
	PriceBrl       *float64
}

// PromptInput carries everything BuildPrompt needs that isn't already in
// AssistantState. The controller assembles it once per turn.
type PromptInput struct {
	Template           *plansv1.PlanTemplate
	Config             *plansv1.PlanConfiguration
	CandidateExecutors []ExecutorOption // filtered for the current step only when state is BINDING_STEP
}

// BuildPrompt returns the assistant's prose text and the JSON payload
// (chat.BuildAssistantPromptPayload(state, stepKey, options)) for the
// given state. Pure.
func BuildPrompt(state AssistantState, in PromptInput) (text, payload string) {
	switch state.Kind {
	case StateAwaitingTemplate:
		return "Pick a template to start.",
			chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)

	case StateBindingStep:
		step := findStep(in.Template, state.StepKey)
		var opts []chat.AssistantOption
		for _, c := range in.CandidateExecutors {
			if c.StepKey != state.StepKey {
				continue
			}
			opts = append(opts, chat.AssistantOption{
				ID:       c.InstallationID,
				Label:    c.DisplayName,
				Sublabel: formatExecutorSublabel(c),
				Value:    c.InstallationID,
				PriceBrl: c.PriceBrl,
			})
		}
		title := state.StepKey
		if step != nil && step.GetTitle() != "" {
			title = step.GetTitle()
		}
		text = fmt.Sprintf("Pick an executor for %s.", title)
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), state.StepKey, opts)
		return

	case StateSetOverseer:
		opts := []chat.AssistantOption{
			{ID: "self", Label: "You", Value: "self"},
		}
		text = "Who answers questions from this step?"
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), state.StepKey, opts)
		return

	case StateSetPolicies:
		opts := []chat.AssistantOption{
			{ID: "balanced", Label: "Balanced", Sublabel: "Pause until answered · Require approval", Value: "PAUSE_UNTIL_ANSWERED+REQUIRE_APPROVAL"},
			{ID: "hands_off", Label: "Hands-off", Sublabel: "Pause until answered · Auto-publish", Value: "PAUSE_UNTIL_ANSWERED+AUTO_PUBLISH"},
			{ID: "strict", Label: "Strict", Sublabel: "Fail step on timeout · Require approval", Value: "FAIL_STEP+REQUIRE_APPROVAL"},
		}
		text = "How should this plan behave when something needs attention?"
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), "", opts)
		return

	case StateConfirm:
		opts := []chat.AssistantOption{
			{ID: "save", Label: "Save", Value: "save"},
		}
		text = "Ready to save?"
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), "", opts)
		return

	case StateSaved:
		text = "Saved. Run it from the canvas or set a schedule."
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)
		return
	}
	return "", chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)
}

func findStep(tpl *plansv1.PlanTemplate, key string) *plansv1.PlanStep {
	for _, s := range tpl.GetSteps() {
		if s.GetKey() == key {
			return s
		}
	}
	return nil
}

func formatExecutorSublabel(c ExecutorOption) string {
	if c.PriceBrl != nil {
		return fmt.Sprintf("%s · R$ %.2f/run", c.SkuKey, *c.PriceBrl)
	}
	return c.SkuKey
}
