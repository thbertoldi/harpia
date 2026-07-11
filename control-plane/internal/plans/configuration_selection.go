package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/database"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/planassistant"
)

type selectionPromptPayload struct {
	ConfigurationID string                 `json:"configuration_id"`
	State           string                 `json:"state"`
	StepKey         string                 `json:"step_key"`
	PolicyKey       string                 `json:"policy_key"`
	Options         []chat.AssistantOption `json:"options"`
	Fields          []selectionPolicyField `json:"fields"`
}

type selectionPolicyField struct {
	Key          string                 `json:"key"`
	ParameterKey string                 `json:"parameter_key"`
	CurrentValue string                 `json:"current_value"`
	Options      []chat.AssistantOption `json:"options"`
}

func selectionPrompt(messages []*chatv1.ThreadMessage, promptID string) (*chatv1.ThreadMessage, selectionPromptPayload, error) {
	for _, msg := range messages {
		if msg.GetId() != promptID {
			continue
		}
		if msg.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			return nil, selectionPromptPayload{}, connect.NewError(connect.CodeInvalidArgument, errors.New("message is not an assistant prompt"))
		}
		var payload selectionPromptPayload
		if err := json.Unmarshal([]byte(msg.GetPayloadJson()), &payload); err != nil {
			return nil, selectionPromptPayload{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("assistant prompt payload is invalid: %w", err))
		}
		if payload.ConfigurationID == "" || payload.State == "" {
			return nil, selectionPromptPayload{}, connect.NewError(connect.CodeInvalidArgument, errors.New("assistant prompt payload is missing configuration_id or state"))
		}
		return msg, payload, nil
	}
	return nil, selectionPromptPayload{}, connect.NewError(connect.CodeNotFound, errors.New("assistant prompt message not found"))
}

func promptAlreadyAnswered(messages []*chatv1.ThreadMessage, promptID string) bool {
	for _, msg := range messages {
		if msg.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION {
			continue
		}
		if userSelectionResponseID(msg.GetPayloadJson()) == promptID {
			return true
		}
	}
	return false
}

func latestUnansweredPromptID(messages []*chatv1.ThreadMessage, configurationID string) string {
	answered := make(map[string]bool)
	for _, msg := range messages {
		if msg.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION {
			if id := userSelectionResponseID(msg.GetPayloadJson()); id != "" {
				answered[id] = true
			}
		}
	}
	latest := ""
	for _, msg := range messages {
		if msg.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT || answered[msg.GetId()] {
			continue
		}
		cid, _, _, ok := selectionPromptFingerprint(msg.GetPayloadJson())
		if ok && cid == configurationID {
			latest = msg.GetId()
		}
	}
	return latest
}

func userSelectionResponseID(payloadJSON string) string {
	var payload struct {
		InResponseToMessageID string `json:"in_response_to_message_id"`
	}
	if json.Unmarshal([]byte(payloadJSON), &payload) != nil {
		return ""
	}
	return payload.InResponseToMessageID
}

func selectionLabel(payload selectionPromptPayload, selection *plansv1.ConfigurationSelection) (string, bool) {
	value := strings.TrimSpace(selection.GetValue())
	optionID := strings.TrimSpace(selection.GetOptionId())
	for _, option := range payload.Options {
		if option.Value == value || option.ID == optionID {
			return option.Label, true
		}
	}
	for _, field := range payload.Fields {
		for _, option := range field.Options {
			if option.Value == value || option.ID == optionID {
				return option.Label, true
			}
		}
	}
	if payload.State == string(planassistant.StateBindingMatrix) {
		return selectionTextForValue(value), true
	}
	return "", false
}

func selectionTextForValue(value string) string {
	switch strings.TrimSpace(value) {
	case "save_draft", "draft":
		return "Save draft"
	case "save_runnable", "runnable", "promote":
		return "Save and make runnable"
	default:
		return value
	}
}

func (h *PlanHandler) applyConfigurationSelection(
	ctx context.Context,
	q database.Querier,
	tenantID uuid.UUID,
	current *PlanConfiguration,
	template *PlanTemplate,
	prompt selectionPromptPayload,
	selection *plansv1.ConfigurationSelection,
	label string,
) (*PlanConfiguration, *chat.AppendInput, error) {
	protoCurrent := configurationToProto(current, template)
	status := stringToConfigurationStatus(current.Status)
	kind := stringToConfigurationKind(current.Kind)
	schedule := protoCurrent.GetSchedule()
	overseers := protoCurrent.GetOverseerBindings()
	value := strings.TrimSpace(selection.GetValue())

	switch prompt.State {
	case string(planassistant.StateBindingStep):
		stepKey := strings.TrimSpace(prompt.StepKey)
		parameterKey, ok := FindSlotParameterForStep(template, stepKey)
		if !ok {
			return nil, nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("step %q is not backed by a selectable slot parameter", stepKey))
		}
		previous := currentSlotBinding(protoCurrent.GetSlotBindings(), stepKey)
		nextParams := MergeParameterValue(string(current.ParameterValues), parameterKey, value)
		updated, err := h.applyConfigurationUpdate(ctx, q, tenantID, current.ID, template, nextParams, overseers, status, kind, schedule)
		if err != nil {
			return nil, nil, err
		}
		return updated, &chat.AppendInput{
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_REBOUND,
			Text:        label,
			PayloadJSON: chat.BuildStepReboundPayload(stepKey, previous, value),
		}, nil

	case string(planassistant.StateOverseerStep):
		stepKey := strings.TrimSpace(prompt.StepKey)
		previous := currentOverseerBinding(overseers, stepKey)
		nextOverseers := MergeOverseerBinding(overseers, stepKey, value)
		updated, err := h.applyConfigurationUpdate(ctx, q, tenantID, current.ID, template, string(current.ParameterValues), nextOverseers, status, kind, schedule)
		if err != nil {
			return nil, nil, err
		}
		return updated, &chat.AppendInput{
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_REBOUND,
			Text:        label,
			PayloadJSON: chat.BuildOverseerReboundPayload(stepKey, previous, value),
		}, nil

	case string(planassistant.StatePoliciesStep):
		policyKey, parameterKey, err := policySelectionKeys(template, prompt)
		if err != nil {
			return nil, nil, err
		}
		previous := parameterValueString(current.ParameterValues, parameterKey)
		nextParams := MergeParameterValue(string(current.ParameterValues), parameterKey, value)
		updated, err := h.applyConfigurationUpdate(ctx, q, tenantID, current.ID, template, nextParams, overseers, status, kind, schedule)
		if err != nil {
			return nil, nil, err
		}
		return updated, &chat.AppendInput{
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_REBOUND,
			Text:        label,
			PayloadJSON: chat.BuildPolicyReboundPayload(policyKey, previous, value),
		}, nil

	case string(planassistant.StateBindingMatrix):
		nextStatus, err := matrixSelectionStatus(value, current.Status, schedule)
		if err != nil {
			return nil, nil, err
		}
		updated, err := h.applyConfigurationUpdate(ctx, q, tenantID, current.ID, template, string(current.ParameterValues), overseers, nextStatus, kind, schedule)
		if err != nil {
			return nil, nil, err
		}
		return updated, nil, nil
	}

	return nil, nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("assistant state %q is not selectable", prompt.State))
}

func (h *PlanHandler) appendNextAssistantPromptTx(ctx context.Context, q database.Querier, tenantID uuid.UUID, template *PlanTemplate, config *PlanConfiguration, messages []*chatv1.ThreadMessage) (*chatv1.ThreadMessage, error) {
	if template == nil || config == nil {
		return nil, nil
	}
	tpl := templateToProto(template)
	cfg := configurationToProto(config, template)
	state := planassistant.DeriveState(tpl, cfg, nil)
	in := planassistant.PromptInput{Template: tpl, Config: cfg}
	if rc, ok := identity.RequestContextFrom(ctx); ok {
		in.CurrentUserID = rc.UserID
		in.CurrentUserLabel = "You"
	}
	if state.Kind == planassistant.StateBindingStep || state.Kind == planassistant.StateOverseerStep || state.Kind == planassistant.StateBindingMatrix {
		catalog := AssistantCatalog{Executors: h.executors}
		byStep := make(map[string][]planassistant.ExecutorOption, len(tpl.GetSteps()))
		for _, step := range tpl.GetSteps() {
			candidates, err := catalog.CandidatesForStep(ctx, tenantID, tpl, step.GetKey())
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load candidates for %s: %w", step.GetKey(), err))
			}
			byStep[step.GetKey()] = candidates
		}
		in.CandidatesByStep = byStep
	}
	text, payload := planassistant.BuildPrompt(state, in)
	payload = stampSelectionPromptConfigurationID(payload, config.ID.String())
	// messages contains the prompts that existed before this candidate plus the
	// USER_SELECTION/STEP_REBOUND appended earlier in this transaction. The
	// candidate prompt is intentionally not in the slice yet; matching an older
	// fingerprint means this turn did not advance and should not re-emit it.
	if duplicateAssistantPrompt(messages, payload) {
		return nil, nil
	}
	msg, err := chat.AppendMessageTx(ctx, q, tenantID, chat.AppendInput{
		ThreadID:    configurationThreadID(config).String(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
		Text:        text,
		PayloadJSON: payload,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return msg, nil
}

func currentSlotBinding(bindings []*plansv1.SlotBinding, stepKey string) string {
	for _, binding := range bindings {
		if binding.GetStepKey() == stepKey {
			return binding.GetExecutorInstallationId()
		}
	}
	return ""
}

func currentOverseerBinding(bindings []*plansv1.OverseerBinding, stepKey string) string {
	for _, binding := range bindings {
		if binding.GetStepKey() == stepKey {
			return binding.GetOverseerUserId()
		}
	}
	return ""
}

func policySelectionKeys(template *PlanTemplate, prompt selectionPromptPayload) (string, string, error) {
	policyKey := strings.TrimSpace(prompt.PolicyKey)
	parameterKey := ""
	for _, field := range prompt.Fields {
		if policyKey == "" || field.Key == policyKey {
			policyKey = field.Key
			parameterKey = field.ParameterKey
			break
		}
	}
	if policyKey == "" {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("policy prompt is missing policy_key"))
	}
	if parameterKey == "" {
		if found, ok := FindPolicyParameter(template, policyKey); ok {
			parameterKey = found
		}
	}
	if parameterKey == "" {
		return "", "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("policy %q is not backed by a selectable parameter", policyKey))
	}
	return policyKey, parameterKey, nil
}

func parameterValueString(raw json.RawMessage, key string) string {
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return ""
	}
	if value, ok := values[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func matrixSelectionStatus(value, currentStatus string, schedule *plansv1.PlanSchedule) (plansv1.PlanConfigurationStatus, error) {
	switch strings.TrimSpace(value) {
	case "save_draft", "draft":
		return stringToConfigurationStatus(currentStatus), nil
	case "save_runnable", "runnable", "promote":
		return stringToConfigurationStatus(PromoteStatus(currentStatus, schedule)), nil
	default:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED,
			connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported matrix selection %q", value))
	}
}

func duplicateAssistantPrompt(messages []*chatv1.ThreadMessage, payload string) bool {
	wantCID, wantState, wantStep, ok := selectionPromptFingerprint(payload)
	if !ok {
		return false
	}
	for _, msg := range messages {
		if msg.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			continue
		}
		cid, state, step, ok := selectionPromptFingerprint(msg.GetPayloadJson())
		if ok && cid == wantCID && state == wantState && step == wantStep {
			return true
		}
	}
	return false
}

func selectionPromptFingerprint(payloadJSON string) (configurationID, state, stepKey string, ok bool) {
	var p struct {
		ConfigurationID string `json:"configuration_id"`
		State           string `json:"state"`
		StepKey         string `json:"step_key"`
		PolicyKey       string `json:"policy_key"`
	}
	if json.Unmarshal([]byte(payloadJSON), &p) != nil {
		return "", "", "", false
	}
	if p.StepKey == "" {
		p.StepKey = p.PolicyKey
	}
	return p.ConfigurationID, p.State, p.StepKey, true
}

func stampSelectionPromptConfigurationID(payload, configurationID string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return payload
	}
	if m == nil {
		m = map[string]any{}
	}
	m["configuration_id"] = configurationID
	out, err := json.Marshal(m)
	if err != nil {
		return payload
	}
	return string(out)
}
