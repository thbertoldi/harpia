package planassistant

import (
	"fmt"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/planrules"
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
// ConfigurationState. The controller assembles it once per turn.
//
// M6: BINDING_MATRIX renders every step at once, so candidates are supplied
// for all steps keyed by step key (was a single-step slice in M5).
type PromptInput struct {
	Template         *plansv1.PlanTemplate
	Config           *plansv1.PlanConfiguration
	CandidatesByStep map[string][]ExecutorOption
	// CurrentUserID is the MVP overseer candidate value for OVERSEER_STEP.
	CurrentUserID string
	// CurrentUserLabel is the display name used as the per-row overseer
	// default ("You" when empty).
	CurrentUserLabel string
}

// BuildPrompt returns the assistant's prose text and the JSON payload for the
// given state. Pure.
func BuildPrompt(state ConfigurationState, in PromptInput) (text, payload string) {
	switch state.Kind {
	case StateAwaitingTemplate:
		// Unreachable in v1 (the template is always bound at creation), but
		// kept as a defensive fallback that surfaces a template picker.
		return "Pick a template to start.",
			chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)

	case StateBindingStep:
		return buildBindingStepPrompt(state, in)

	case StateOverseerStep:
		return buildOverseerStepPrompt(state, in)

	case StatePoliciesStep:
		return buildPoliciesStepPrompt(state, in)

	case StateBindingMatrix:
		return buildMatrixPrompt(in)

	}
	return "", chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)
}

func buildMatrixPrompt(in PromptInput) (string, string) {
	user := currentUserLabel(in)
	rows := matrixRows(in)
	text := fmt.Sprintf("Here's the plan. Pick an executor for each task — overseer defaults to %s.", user)
	payload := chat.BuildAssistantMatrixPayload(rows, policiesSet(in.Template, in.Config.GetBehaviorPolicies()))
	return text, payload
}

func buildBindingStepPrompt(state ConfigurationState, in PromptInput) (string, string) {
	rows := matrixRows(in)
	var focusedTitle string
	var focusedOptions []chat.AssistantOption
	for _, row := range rows {
		if row.StepKey == state.StepKey {
			focusedTitle = row.StepTitle
			focusedOptions = row.Options
			break
		}
	}
	if focusedTitle == "" {
		focusedTitle = state.StepKey
	}
	text := fmt.Sprintf("Who should handle %s?", focusedTitle)
	payload := chat.BuildAssistantBindingStepPayload(
		state.StepKey,
		focusedOptions,
		rows,
		policiesSet(in.Template, in.Config.GetBehaviorPolicies()),
	)
	return text, payload
}

func buildOverseerStepPrompt(state ConfigurationState, in PromptInput) (string, string) {
	rows := matrixRows(in)
	focusedTitle := state.StepKey
	for _, row := range rows {
		if row.StepKey == state.StepKey {
			focusedTitle = row.StepTitle
			break
		}
	}
	text := fmt.Sprintf("Who should oversee %s?", focusedTitle)
	payload := chat.BuildAssistantOverseerStepPayload(
		state.StepKey,
		overseerOptions(in),
		requiredOverseerStepKeys(in.Template, in.Config),
		rows,
	)
	return text, payload
}

func buildPoliciesStepPrompt(state ConfigurationState, in PromptInput) (string, string) {
	allFields := []chat.AssistantPolicyField{
		{
			Key:          "content_output_format",
			ParameterKey: policyParameterKey(in.Template, "content_output_format", "output_format"),
			CurrentValue: contentOutputFormatValue(in.Config.GetBehaviorPolicies().GetContentOutputFormat()),
			Options: []chat.AssistantOption{
				{ID: "text_post", Label: "Text post", Value: "text_post"},
				{ID: "carousel", Label: "Carousel", Value: "carousel"},
				{ID: "approval_only", Label: "Approval only (text)", Value: "approval_only"},
			},
		},
		{
			Key:          "publish_approval_mode",
			ParameterKey: policyParameterKey(in.Template, "publish_approval_mode", "approval_mode"),
			CurrentValue: publishApprovalModeValue(in.Config.GetBehaviorPolicies().GetPublishApprovalMode()),
			Options: []chat.AssistantOption{
				{ID: "require_approval", Label: "Require approval", Value: "require_approval"},
				{ID: "auto_publish", Label: "Auto publish", Value: "auto_publish"},
			},
		},
		{
			Key:          "elicitation_timeout_behavior",
			ParameterKey: policyParameterKey(in.Template, "elicitation_timeout_behavior", "elicitation_timeout_behavior"),
			CurrentValue: elicitationTimeoutBehaviorValue(in.Config.GetBehaviorPolicies().GetElicitationTimeoutBehavior()),
			Options: []chat.AssistantOption{
				{ID: "pause_until_answered", Label: "Pause until answered", Value: "pause_until_answered"},
				{ID: "fail_step", Label: "Fail step", Value: "fail_step"},
				{ID: "fail_plan", Label: "Fail plan", Value: "fail_plan"},
			},
		},
	}
	// Only offer policies the template actually declares.
	declared := make([]chat.AssistantPolicyField, 0, len(allFields))
	for _, field := range allFields {
		if templateDeclaresPolicy(in.Template, field.Key) {
			declared = append(declared, field)
		}
	}

	policyKey := state.PolicyKey
	if policyKey == "" {
		policyKey = firstUnsetPolicyKey(in.Template, in.Config.GetBehaviorPolicies())
	}
	fields := make([]chat.AssistantPolicyField, 0, 1)
	for _, field := range declared {
		if field.Key == policyKey {
			fields = append(fields, field)
			break
		}
	}
	if len(fields) == 0 {
		fields = declared
	}
	text := "How should this plan behave at runtime?"
	payload := chat.BuildAssistantPoliciesStepPayload(policyKey, fields, policiesSet(in.Template, in.Config.GetBehaviorPolicies()))
	return text, payload
}

func currentUserLabel(in PromptInput) string {
	user := in.CurrentUserLabel
	if user == "" {
		user = "You"
	}
	return user
}

func currentUserID(in PromptInput) string {
	userID := in.CurrentUserID
	if userID == "" {
		userID = "self"
	}
	return userID
}

func policyParameterKey(template *plansv1.PlanTemplate, policyKey, fallback string) string {
	for _, parameter := range template.GetInputParameters() {
		for _, mapping := range parameter.GetRuntimeMappings() {
			if mapping.GetTarget() == plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY &&
				mapping.GetPolicyKey() == policyKey &&
				parameter.GetKey() != "" {
				return parameter.GetKey()
			}
		}
	}
	return fallback
}

func publishApprovalModeValue(mode plansv1.PublishApprovalMode) string {
	switch mode {
	case plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL:
		return "require_approval"
	case plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH:
		return "auto_publish"
	default:
		return ""
	}
}

func contentOutputFormatValue(format plansv1.ContentOutputFormat) string {
	switch format {
	case plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_TEXT_POST:
		return "text_post"
	case plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_CAROUSEL:
		return "carousel"
	case plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_IMAGE_BACKED_POST:
		return "image_backed_post"
	case plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_APPROVAL_ONLY:
		return "approval_only"
	default:
		return ""
	}
}

func elicitationTimeoutBehaviorValue(behavior plansv1.ElicitationTimeoutBehavior) string {
	switch behavior {
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED:
		return "pause_until_answered"
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_STEP:
		return "fail_step"
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_PLAN:
		return "fail_plan"
	default:
		return ""
	}
}

func matrixRows(in PromptInput) []chat.AssistantMatrixRow {
	user := currentUserLabel(in)
	userID := currentUserID(in)
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
		if !planrules.StepWillRun(step, in.Config.GetIncludedOptionalCapabilities()) {
			continue
		}
		overseerID := overseers[step.GetKey()]
		overseerLabel := user
		if overseerID != "" && overseerID != "self" && overseerID != userID {
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
	return rows
}

func overseerOptions(in PromptInput) []chat.AssistantOption {
	userID := currentUserID(in)
	return []chat.AssistantOption{{
		ID:    userID,
		Label: currentUserLabel(in),
		Value: userID,
	}}
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
