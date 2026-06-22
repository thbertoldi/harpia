package chat

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildElicitationRaisedPayload(t *testing.T) {
	id := uuid.New()
	got := BuildElicitationRaisedPayload(id)

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["elicitation_id"] != id.String() {
		t.Fatalf("expected elicitation_id=%s, got %s", id.String(), decoded["elicitation_id"])
	}
}

func TestBuildElicitationAnsweredPayload(t *testing.T) {
	id := uuid.New()
	got := BuildElicitationAnsweredPayload(id, "answered")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["elicitation_id"] != id.String() {
		t.Fatalf("expected elicitation_id=%s, got %s", id.String(), decoded["elicitation_id"])
	}
	if decoded["outcome"] != "answered" {
		t.Fatalf("expected outcome=answered, got %s", decoded["outcome"])
	}
}

func TestBuildApprovalRaisedPayload(t *testing.T) {
	got := BuildApprovalRaisedPayload("abc-123")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["approval_request_id"] != "abc-123" {
		t.Fatalf("expected approval_request_id=abc-123, got %s", decoded["approval_request_id"])
	}
}

func TestBuildApprovalDecidedPayload(t *testing.T) {
	got := BuildApprovalDecidedPayload("abc-123", true)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["approval_request_id"] != "abc-123" {
		t.Fatalf("expected approval_request_id=abc-123, got %v", decoded["approval_request_id"])
	}
	if decoded["approved"] != true {
		t.Fatalf("expected approved=true, got %v", decoded["approved"])
	}
}

func TestBuildStepBoundPayload(t *testing.T) {
	got := BuildStepBoundPayload("write-draft", "art-uuid")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["step_key"] != "write-draft" {
		t.Fatalf("expected step_key=write-draft, got %s", decoded["step_key"])
	}
	if decoded["output_artifact_id"] != "art-uuid" {
		t.Fatalf("expected output_artifact_id=art-uuid, got %s", decoded["output_artifact_id"])
	}
}

func TestBuildStepStartedPayload(t *testing.T) {
	got := BuildStepStartedPayload("write-draft", "step-exec-uuid")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["step_key"] != "write-draft" {
		t.Fatalf("expected step_key=write-draft, got %s", decoded["step_key"])
	}
	if decoded["step_execution_id"] != "step-exec-uuid" {
		t.Fatalf("expected step_execution_id=step-exec-uuid, got %s", decoded["step_execution_id"])
	}
}

func TestBuildRunFailedPayloadCarriesErrorMessage(t *testing.T) {
	got := BuildRunFailedPayload("step write-draft failed: timeout")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["error"] != "step write-draft failed: timeout" {
		t.Fatalf("expected error message preserved, got %s", decoded["error"])
	}
}
