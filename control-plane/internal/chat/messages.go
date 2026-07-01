// Package chat owns durable chat-message persistence and streaming.
//
// Designed to be reusable beyond the M3 plan-thread use case: store is
// keyed by a generic thread_id (= plan_configuration_id in M3). See
// docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md.
package chat

import (
	"encoding/json"

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

type userSelectionPayload struct {
	InResponseToMessageID string `json:"in_response_to_message_id"`
	OptionID              string `json:"option_id"`
	Value                 string `json:"value"`
}

type stepReboundPayload struct {
	StepKey                        string `json:"step_key"`
	PreviousExecutorInstallationID string `json:"previous_executor_installation_id"`
	NewExecutorInstallationID      string `json:"new_executor_installation_id"`
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
	TemplateID      string  `json:"template_id"`
	TemplateKey     string  `json:"template_key"`
	TemplateName    string  `json:"template_name"`
	Confidence      float64 `json:"confidence"`
	InputValuesJSON string  `json:"input_values_json"`
}

type planProposedPayload struct {
	SourceMessageID string                  `json:"source_message_id"`
	Candidates      []PlanProposalCandidate `json:"candidates"`
}

// BuildPlanProposedPayload returns the JSON payload for a PLAN_PROPOSED message.
// candidates may be empty when the router found no confident match.
func BuildPlanProposedPayload(sourceMessageID string, candidates []PlanProposalCandidate) string {
	if candidates == nil {
		candidates = []PlanProposalCandidate{}
	}
	return mustEncodeJSON(planProposedPayload{SourceMessageID: sourceMessageID, Candidates: candidates})
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
