package plans

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

func TestLatestUnansweredPromptIDIsConfigurationScoped(t *testing.T) {
	configID := uuid.NewString()
	answeredPrompt := assistantPromptMessage("prompt-1", configID, "BINDING_STEP", "fetch-news", "")
	currentPrompt := assistantPromptMessage("prompt-2", configID, "BINDING_STEP", "write-draft", "")
	otherConfigPrompt := assistantPromptMessage("prompt-3", uuid.NewString(), "BINDING_STEP", "publish", "")
	messages := []*chatv1.ThreadMessage{
		answeredPrompt,
		userSelectionMessage("selection-1", answeredPrompt.GetId()),
		currentPrompt,
		otherConfigPrompt,
	}

	if got := latestUnansweredPromptID(messages, configID); got != currentPrompt.GetId() {
		t.Fatalf("latestUnansweredPromptID = %q, want %q", got, currentPrompt.GetId())
	}
	if !promptAlreadyAnswered(messages, answeredPrompt.GetId()) {
		t.Fatal("promptAlreadyAnswered = false, want true")
	}
}

func TestSelectionLabelRequiresPromptOption(t *testing.T) {
	payload := selectionPromptPayload{
		State: "BINDING_STEP",
		Options: []chat.AssistantOption{
			{ID: "rss", Label: "RSS", Value: "inst-rss"},
		},
	}

	label, ok := selectionLabel(payload, &plansv1.ConfigurationSelection{OptionId: "rss", Value: "inst-rss"})
	if !ok || label != "RSS" {
		t.Fatalf("selectionLabel matched = %v,%q; want true,RSS", ok, label)
	}
	if _, ok := selectionLabel(payload, &plansv1.ConfigurationSelection{OptionId: "other", Value: "inst-other"}); ok {
		t.Fatal("selectionLabel accepted an option that was not offered")
	}
}

func TestDuplicateAssistantPromptUsesPolicyKey(t *testing.T) {
	configID := uuid.NewString()
	messages := []*chatv1.ThreadMessage{
		assistantPromptMessage("prompt-1", configID, "POLICIES_STEP", "", "publish_approval_mode"),
	}
	samePolicy := stampSelectionPromptConfigurationID(chat.BuildAssistantPoliciesStepPayload(
		"publish_approval_mode",
		[]chat.AssistantPolicyField{{Key: "publish_approval_mode"}},
		false,
	), configID)
	nextPolicy := stampSelectionPromptConfigurationID(chat.BuildAssistantPoliciesStepPayload(
		"elicitation_timeout_behavior",
		[]chat.AssistantPolicyField{{Key: "elicitation_timeout_behavior"}},
		false,
	), configID)

	if !duplicateAssistantPrompt(messages, samePolicy) {
		t.Fatal("same policy prompt should dedup")
	}
	if duplicateAssistantPrompt(messages, nextPolicy) {
		t.Fatal("different policy_key must not dedup")
	}
}

func assistantPromptMessage(id, configID, state, stepKey, policyKey string) *chatv1.ThreadMessage {
	payload := map[string]string{
		"configuration_id": configID,
		"state":            state,
	}
	if stepKey != "" {
		payload["step_key"] = stepKey
	}
	if policyKey != "" {
		payload["policy_key"] = policyKey
	}
	return &chatv1.ThreadMessage{
		Id:          id,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
		PayloadJson: mustSelectionTestJSON(payload),
	}
}

func userSelectionMessage(id, inResponseTo string) *chatv1.ThreadMessage {
	return &chatv1.ThreadMessage{
		Id:          id,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION,
		PayloadJson: chat.BuildUserSelectionPayload(inResponseTo, "option", "value"),
	}
}

func mustSelectionTestJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}
