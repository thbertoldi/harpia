package planassistant

import (
	"fmt"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

// ExecutorOption is one available executor installation for a step, loaded by
// the controller before calling BuildPrompt.
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
//
// M6: BINDING_MATRIX renders every step at once, so candidates are supplied
// for all steps keyed by step key (was a single-step slice in M5).
type PromptInput struct {
	Template         *plansv1.PlanTemplate
	Config           *plansv1.PlanConfiguration
	CandidatesByStep map[string][]ExecutorOption
	// CurrentUserLabel is the display name used as the per-row overseer
	// default ("You" when empty).
	CurrentUserLabel string
}

// BuildPrompt returns the assistant's prose text and the JSON payload for the
// given state. Pure.
func BuildPrompt(state AssistantState, in PromptInput) (text, payload string) {
	switch state.Kind {
	case StateAwaitingTemplate:
		// Unreachable in v1 (the template is always bound at creation), but
		// kept as a defensive fallback that surfaces a template picker.
		return "Pick a template to start.",
			chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)

	case StateBindingMatrix:
		return buildMatrixPrompt(in)

	case StateSaved:
		return buildLandingPrompt()
	}
	return "", chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)
}

func buildMatrixPrompt(in PromptInput) (string, string) {
	user := in.CurrentUserLabel
	if user == "" {
		user = "You"
	}

	bindings := map[string]string{}
	for _, sb := range in.Config.GetSlotBindings() {
		if sb.GetExecutorInstallationId() != "" {
			bindings[sb.GetStepKey()] = sb.GetExecutorInstallationId()
		}
	}
	overseers := map[string]string{}
	for _, ob := range in.Config.GetOverseerBindings() {
		if ob.GetOverseerUserId() != "" {
			overseers[ob.GetStepKey()] = ob.GetOverseerUserId()
		}
	}

	rows := make([]chat.AssistantMatrixRow, 0, len(in.Template.GetSteps()))
	for _, step := range in.Template.GetSteps() {
		overseerID := overseers[step.GetKey()]
		overseerLabel := user
		if overseerID != "" && overseerID != "self" {
			overseerLabel = overseerID
		}
		rows = append(rows, chat.AssistantMatrixRow{
			StepKey:   step.GetKey(),
			StepTitle: stepTitle(step),
			Contracts: chat.MatrixContracts{
				Input:  step.GetInputArtifactTypeId(),
				Output: step.GetOutputArtifactTypeId(),
			},
			Options:              executorOptions(in.CandidatesByStep[step.GetKey()]),
			CurrentExecutorID:    bindings[step.GetKey()],
			CurrentOverseerID:    overseerID,
			CurrentOverseerLabel: overseerLabel,
		})
	}

	text := fmt.Sprintf("Here's the plan. Pick an executor for each task — overseer defaults to %s.", user)
	payload := chat.BuildAssistantMatrixPayload(rows, policiesSet(in.Config.GetBehaviorPolicies()))
	return text, payload
}

func buildLandingPrompt() (string, string) {
	text := "Saved. Run it now, schedule a recurring run, or walk away — it'll be here when you come back."
	payload := chat.BuildAssistantLandingPayload([]chat.AssistantAction{
		{ID: "run-now", Label: "Run now"},
		{ID: "schedule", Label: "Schedule…"},
		{ID: "walk-away", Label: "Save and walk away"},
	})
	return text, payload
}

func executorOptions(cands []ExecutorOption) []chat.AssistantOption {
	opts := make([]chat.AssistantOption, 0, len(cands))
	for _, c := range cands {
		opts = append(opts, chat.AssistantOption{
			ID:       c.InstallationID,
			Label:    c.DisplayName,
			Sublabel: formatExecutorSublabel(c),
			Value:    c.InstallationID,
			PriceBrl: c.PriceBrl,
		})
	}
	return opts
}

func stepTitle(step *plansv1.PlanStep) string {
	if step.GetTitle() != "" {
		return step.GetTitle()
	}
	return step.GetKey()
}

func formatExecutorSublabel(c ExecutorOption) string {
	if c.PriceBrl != nil {
		return fmt.Sprintf("%s · R$ %.2f/run", c.SkuKey, *c.PriceBrl)
	}
	return c.SkuKey
}
