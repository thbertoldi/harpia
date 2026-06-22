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
