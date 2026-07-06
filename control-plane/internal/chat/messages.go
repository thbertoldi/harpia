// Package chat owns durable chat-message persistence and streaming.
//
// Designed to be reusable beyond the M3 plan-thread use case: store is
// keyed by a generic thread_id (= plan_configuration_id in M3). See
// docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md.
package chat

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// BuildConfigurationSavedPayload returns the JSON payload for a
// CONFIGURATION_SAVED message. Currently carries no structured fields —
// the message's text is sufficient — but the shape is reserved for future
// fields (e.g., diff against previous configuration).
func BuildConfigurationSavedPayload() string {
	return "{}"
}

// BuildRunStartedPayload returns the JSON payload for a RUN_STARTED message.
// Currently carries no fields beyond the execution_id which lives on the
// row's execution_id column.
func BuildRunStartedPayload() string {
	return "{}"
}

// BuildRunCompletedPayload returns the JSON payload for a RUN_COMPLETED message.
func BuildRunCompletedPayload() string {
	return "{}"
}

// BuildRunFailedPayload returns the JSON payload for a RUN_FAILED message.
func BuildRunFailedPayload(errorMessage string) string {
	return mustEncodeJSON(map[string]string{"error": errorMessage})
}

// BuildStepBoundPayload returns the JSON payload for a STEP_BOUND message.
func BuildStepBoundPayload(stepKey, outputArtifactID string) string {
	return mustEncodeJSON(map[string]string{
		"step_key":           stepKey,
		"output_artifact_id": outputArtifactID,
	})
}

// BuildStepStartedPayload returns the JSON payload for a STEP_STARTED message.
// Mirrors STEP_BOUND but without an output artifact (which doesn't exist yet
// at start-of-step time).
func BuildStepStartedPayload(stepKey, stepExecutionID string) string {
	return mustEncodeJSON(map[string]string{
		"step_key":          stepKey,
		"step_execution_id": stepExecutionID,
	})
}

// BuildElicitationRaisedPayload returns the JSON payload for an
// ELICITATION_RAISED pointer message.
func BuildElicitationRaisedPayload(elicitationID uuid.UUID) string {
	return mustEncodeJSON(map[string]string{
		"elicitation_id": elicitationID.String(),
	})
}

// BuildElicitationAnsweredPayload returns the JSON payload for an
// ELICITATION_ANSWERED message. Outcome is one of: "answered", "timed_out",
// "cancelled".
func BuildElicitationAnsweredPayload(elicitationID uuid.UUID, outcome string) string {
	return mustEncodeJSON(map[string]string{
		"elicitation_id": elicitationID.String(),
		"outcome":        outcome,
	})
}

// BuildApprovalRaisedPayload returns the JSON payload for an
// APPROVAL_RAISED pointer message.
func BuildApprovalRaisedPayload(approvalRequestID string) string {
	return mustEncodeJSON(map[string]string{
		"approval_request_id": approvalRequestID,
	})
}

// BuildApprovalDecidedPayload returns the JSON payload for an
// APPROVAL_DECIDED message.
func BuildApprovalDecidedPayload(approvalRequestID string, approved bool) string {
	return mustEncodeJSON(map[string]any{
		"approval_request_id": approvalRequestID,
		"approved":            approved,
	})
}

func mustEncodeJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// All inputs here are well-formed maps; failure is a programmer error.
		panic(err)
	}
	return string(b)
}

// AssistantOption is one chip in an ASSISTANT_PROMPT payload.
type AssistantOption struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Sublabel string   `json:"sublabel,omitempty"`
	Value    string   `json:"value"`
	PriceBrl *float64 `json:"price_brl,omitempty"`
}

// Payload structs are typed so JSON output is stable in declaration order.
// Do NOT swap to map[string]string — Go's encoding/json sorts map keys
// alphabetically, which breaks the byte-exact tests in messages_test.go.

type configurationStartedPayload struct {
	TemplateID string `json:"template_id"`
}

type assistantPromptPayload struct {
	State   string            `json:"state"`
	StepKey string            `json:"step_key"`
	Options []AssistantOption `json:"options"`
}

type assistantBindingStepPayload struct {
	State       string               `json:"state"`
	StepKey     string               `json:"step_key"`
	Options     []AssistantOption    `json:"options"`
	PoliciesSet bool                 `json:"policies_set"`
	Rows        []AssistantMatrixRow `json:"rows"`
}

type assistantOverseerStepPayload struct {
	State            string               `json:"state"`
	StepKey          string               `json:"step_key"`
	Options          []AssistantOption    `json:"options"`
	RequiredStepKeys []string             `json:"required_step_keys"`
	Rows             []AssistantMatrixRow `json:"rows"`
}

// AssistantPolicyField is one behavior-policy field in a POLICIES_STEP
// assistant prompt.
type AssistantPolicyField struct {
	Key          string            `json:"key"`
	ParameterKey string            `json:"parameter_key"`
	CurrentValue string            `json:"current_value"`
	Options      []AssistantOption `json:"options"`
}

type assistantPoliciesStepPayload struct {
	State       string                 `json:"state"`
	PolicyKey   string                 `json:"policy_key,omitempty"`
	Fields      []AssistantPolicyField `json:"fields"`
	PoliciesSet bool                   `json:"policies_set"`
}

type userSelectionPayload struct {
	InResponseToMessageID string `json:"in_response_to_message_id"`
	OptionID              string `json:"option_id"`
	Value                 string `json:"value"`
}

type stepReboundPayload struct {
	StepKey                        string `json:"step_key"`
	PreviousExecutorInstallationID string `json:"previous_executor_installation_id,omitempty"`
	NewExecutorInstallationID      string `json:"new_executor_installation_id,omitempty"`
	PreviousOverseerUserID         string `json:"previous_overseer_user_id,omitempty"`
	NewOverseerUserID              string `json:"new_overseer_user_id,omitempty"`
	PolicyKey                      string `json:"policy_key,omitempty"`
	PreviousPolicyValue            string `json:"previous_policy_value,omitempty"`
	NewPolicyValue                 string `json:"new_policy_value,omitempty"`
}

type scheduleSetPayload struct {
	ScheduleCron string `json:"schedule_cron"`
	Timezone     string `json:"timezone"`
}

// BuildConfigurationStartedPayload returns the JSON payload for a
// CONFIGURATION_STARTED message — written once on PlanConfiguration insert.
func BuildConfigurationStartedPayload(templateID string) string {
	return mustEncodeJSON(configurationStartedPayload{TemplateID: templateID})
}

// BuildAssistantPromptPayload returns the JSON payload for an
// ASSISTANT_PROMPT message — the assistant's turn with quick-reply chips.
func BuildAssistantPromptPayload(state, stepKey string, options []AssistantOption) string {
	if options == nil {
		options = []AssistantOption{}
	}
	return mustEncodeJSON(assistantPromptPayload{State: state, StepKey: stepKey, Options: options})
}

// BuildAssistantBindingStepPayload returns the JSON payload for a focused
// conversational SlotBinding prompt. The focused options feed the chips; rows
// feed the collapsible edit-all matrix fallback.
func BuildAssistantBindingStepPayload(stepKey string, options []AssistantOption, rows []AssistantMatrixRow, policiesSet bool) string {
	if options == nil {
		options = []AssistantOption{}
	}
	if rows == nil {
		rows = []AssistantMatrixRow{}
	}
	return mustEncodeJSON(assistantBindingStepPayload{
		State:       "BINDING_STEP",
		StepKey:     stepKey,
		Options:     options,
		PoliciesSet: policiesSet,
		Rows:        rows,
	})
}

// BuildAssistantOverseerStepPayload returns the JSON payload for a focused
// conversational OverseerBinding prompt. The focused options feed the chips;
// rows feed the matrix review context after selection.
func BuildAssistantOverseerStepPayload(stepKey string, options []AssistantOption, requiredStepKeys []string, rows []AssistantMatrixRow) string {
	if options == nil {
		options = []AssistantOption{}
	}
	if requiredStepKeys == nil {
		requiredStepKeys = []string{}
	}
	if rows == nil {
		rows = []AssistantMatrixRow{}
	}
	return mustEncodeJSON(assistantOverseerStepPayload{
		State:            "OVERSEER_STEP",
		StepKey:          stepKey,
		Options:          options,
		RequiredStepKeys: requiredStepKeys,
		Rows:             rows,
	})
}

// BuildAssistantPoliciesStepPayload returns the JSON payload for a focused
// conversational PlanBehaviorPolicies prompt. policyKey identifies the single
// field this turn asks about (one prompt = one selection); it is also carried
// at the top level so the assistant's semantic dedup can distinguish two
// policy-field prompts for the same configuration.
func BuildAssistantPoliciesStepPayload(policyKey string, fields []AssistantPolicyField, policiesSet bool) string {
	if fields == nil {
		fields = []AssistantPolicyField{}
	}
	for i := range fields {
		if fields[i].Options == nil {
			fields[i].Options = []AssistantOption{}
		}
	}
	return mustEncodeJSON(assistantPoliciesStepPayload{
		State:       "POLICIES_STEP",
		PolicyKey:   policyKey,
		Fields:      fields,
		PoliciesSet: policiesSet,
	})
}

// BuildUserSelectionPayload returns the JSON payload for a USER_SELECTION
// message — Ana's chip click in response to an ASSISTANT_PROMPT.
func BuildUserSelectionPayload(inResponseToMessageID, optionID, value string) string {
	return mustEncodeJSON(userSelectionPayload{
		InResponseToMessageID: inResponseToMessageID,
		OptionID:              optionID,
		Value:                 value,
	})
}

// BuildStepReboundPayload returns the JSON payload for a STEP_REBOUND
// system message — Ana edited a previously-bound executor.
func BuildStepReboundPayload(stepKey, previousInstallationID, newInstallationID string) string {
	return mustEncodeJSON(stepReboundPayload{
		StepKey:                        stepKey,
		PreviousExecutorInstallationID: previousInstallationID,
		NewExecutorInstallationID:      newInstallationID,
	})
}

// BuildOverseerReboundPayload returns the JSON payload for an overseer change
// echoed into the thread as STEP_REBOUND.
func BuildOverseerReboundPayload(stepKey, previousOverseerUserID, newOverseerUserID string) string {
	return mustEncodeJSON(stepReboundPayload{
		StepKey:                stepKey,
		PreviousOverseerUserID: previousOverseerUserID,
		NewOverseerUserID:      newOverseerUserID,
	})
}

// BuildPolicyReboundPayload returns the JSON payload for a behavior-policy
// change echoed into the thread as STEP_REBOUND.
func BuildPolicyReboundPayload(policyKey, previousPolicyValue, newPolicyValue string) string {
	return mustEncodeJSON(stepReboundPayload{
		PolicyKey:           policyKey,
		PreviousPolicyValue: previousPolicyValue,
		NewPolicyValue:      newPolicyValue,
	})
}

// BuildScheduleSetPayload returns the JSON payload for a SCHEDULE_SET
// system message — the schedule dialog persisted a cron expression.
func BuildScheduleSetPayload(scheduleCron, timezone string) string {
	return mustEncodeJSON(scheduleSetPayload{ScheduleCron: scheduleCron, Timezone: timezone})
}

// AssistantMatrixRow is one task row in a BINDING_MATRIX ASSISTANT_PROMPT
// payload (M6 spec §2.1). The matrix card renders every row at once with an
// executor picker and an overseer cell defaulted per row.
type AssistantMatrixRow struct {
	StepKey              string            `json:"step_key"`
	StepTitle            string            `json:"step_title"`
	Contracts            MatrixContracts   `json:"contracts"`
	Options              []AssistantOption `json:"options"`
	CurrentExecutorID    string            `json:"current_executor_id"`
	CurrentOverseerID    string            `json:"current_overseer_id"`
	CurrentOverseerLabel string            `json:"current_overseer_label"`
}

// MatrixContracts carries a step's input/output artifact-type contract,
// rendered as a chip on the matrix row.
type MatrixContracts struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// AssistantAction is one primary action on the landing card (M6 spec §2.2).
type AssistantAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type assistantMatrixPayload struct {
	State       string               `json:"state"`
	PoliciesSet bool                 `json:"policies_set"`
	Rows        []AssistantMatrixRow `json:"rows"`
}

type assistantLandingPayload struct {
	State   string            `json:"state"`
	Actions []AssistantAction `json:"actions"`
}

// BuildAssistantMatrixPayload returns the JSON payload for a BINDING_MATRIX
// ASSISTANT_PROMPT — the single card that surfaces every step row, the
// overseer-per-row default, and the policy state. Reuses the ASSISTANT_PROMPT
// kind; the state field distinguishes it from a chip prompt.
func BuildAssistantMatrixPayload(rows []AssistantMatrixRow, policiesSet bool) string {
	if rows == nil {
		rows = []AssistantMatrixRow{}
	}
	return mustEncodeJSON(assistantMatrixPayload{State: "BINDING_MATRIX", PoliciesSet: policiesSet, Rows: rows})
}

// BuildAssistantLandingPayload returns the JSON payload for the landing-state
// ASSISTANT_PROMPT — the post-save card with Run now / Schedule / Walk away.
func BuildAssistantLandingPayload(actions []AssistantAction) string {
	if actions == nil {
		actions = []AssistantAction{}
	}
	return mustEncodeJSON(assistantLandingPayload{State: "landing", Actions: actions})
}

// PlanProposalCandidate is one template the router proposes for a thread's
// latest user message, with inferred input values.
type PlanProposalCandidate struct {
	TemplateID           string  `json:"template_id"`
	TemplateKey          string  `json:"template_key"`
	TemplateName         string  `json:"template_name"`
	Confidence           float64 `json:"confidence"`
	InputValuesJSON      string  `json:"input_values_json"`
	RecommendationReason string  `json:"recommendation_reason,omitempty"`
	CompatibilityLabel   string  `json:"compatibility_label,omitempty"`
}

// PlanRefinementDefaults carries deterministic assistant suggestions for the
// pre-create refinement loop.
type PlanRefinementDefaults struct {
	Audience       string   `json:"audience,omitempty"`
	Themes         []string `json:"themes,omitempty"`
	TopicsToAvoid  []string `json:"topics_to_avoid,omitempty"`
	SourceGroups   []string `json:"source_groups,omitempty"`
	Language       string   `json:"language,omitempty"`
	Tone           string   `json:"tone,omitempty"`
	DateRangeStart string   `json:"date_range_start,omitempty"`
	DateRangeEnd   string   `json:"date_range_end,omitempty"`
}

type planProposedPayload struct {
	SourceMessageID    string                  `json:"source_message_id"`
	Summary            string                  `json:"summary,omitempty"`
	BestCandidateID    string                  `json:"best_candidate_id,omitempty"`
	RefinementDefaults *PlanRefinementDefaults `json:"refinement_defaults,omitempty"`
	Candidates         []PlanProposalCandidate `json:"candidates"`
}

// BuildPlanProposedPayload returns the JSON payload for a PLAN_PROPOSED message.
// candidates may be empty when the router found no confident match.
func BuildPlanProposedPayload(sourceMessageID, summary string, candidates []PlanProposalCandidate, defaults ...PlanRefinementDefaults) string {
	if candidates == nil {
		candidates = []PlanProposalCandidate{}
	}
	bestCandidateID := bestPlanProposalCandidateID(candidates)
	var refinementDefaults *PlanRefinementDefaults
	if len(defaults) > 0 {
		refinementDefaults = &defaults[0]
	}
	return mustEncodeJSON(planProposedPayload{
		SourceMessageID:    sourceMessageID,
		Summary:            strings.TrimSpace(summary),
		BestCandidateID:    bestCandidateID,
		RefinementDefaults: refinementDefaults,
		Candidates:         candidates,
	})
}

func bestPlanProposalCandidateID(candidates []PlanProposalCandidate) string {
	if len(candidates) == 0 {
		return ""
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Confidence > best.Confidence {
			best = candidate
		}
	}
	return best.TemplateID
}

type planAttachedPayload struct {
	PlanConfigurationID string `json:"plan_configuration_id"`
	TemplateID          string `json:"template_id"`
}

// BuildPlanAttachedPayload returns the JSON payload for a PLAN_ATTACHED message,
// emitted when a plan is created from a thread.
func BuildPlanAttachedPayload(planConfigurationID, templateID string) string {
	return mustEncodeJSON(planAttachedPayload{PlanConfigurationID: planConfigurationID, TemplateID: templateID})
}
