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
