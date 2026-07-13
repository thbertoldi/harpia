package planassistant

import (
	"fmt"
	"time"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// ConversationTurnKind is the stable execution-prompt state projected into a
// Conversation. It deliberately shares the wire contract's enum values.
type ConversationTurnKind = chatv1.ExecutionPromptState

const (
	ConversationTurnConfiguring             = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_CONFIGURING
	ConversationTurnReadyToRun              = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_READY_TO_RUN
	ConversationTurnWaitingForSchedule      = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_WAITING_FOR_SCHEDULE
	ConversationTurnConfigurationDisabled   = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_CONFIGURATION_DISABLED
	ConversationTurnConfigurationArchived   = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_CONFIGURATION_ARCHIVED
	ConversationTurnExecutionQueued         = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_QUEUED
	ConversationTurnExecutionRunning        = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_RUNNING
	ConversationTurnAwaitingElicitation     = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_ELICITATION
	ConversationTurnAwaitingReview          = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_REVIEW
	ConversationTurnAwaitingApproval        = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_APPROVAL
	ConversationTurnExecutionFailing        = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_FAILING
	ConversationTurnExecutionFailed         = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_FAILED
	ConversationTurnExecutionCompleted      = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_COMPLETED
	ConversationTurnExecutionCancelled      = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_CANCELLED
	ConversationTurnExecutionNeedsAttention = chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_NEEDS_ATTENTION
)

// ConversationTarget records the selected configuration and, when an event or
// action supplied one, the exact execution identity. A display fallback is
// intentionally marked so an adapter cannot treat it as action resolution.
type ConversationTarget struct {
	ConfigurationID string
	PlanExecutionID string
	DisplayFallback bool
}

// PendingInteraction is one durable, request-scoped human checkpoint. The
// execution and StepExecution IDs are both mandatory: matching PlanStep keys
// alone is unsafe when concurrent executions share the same template.
type PendingInteraction struct {
	Kind               chatv1.ExecutionInteractionKind
	RequestID          string
	PlanExecutionID    string
	StepExecutionID    string
	PlanStepKey        string
	SubjectArtifactRef *artifactsv1.ArtifactRef
	Actions            []*chatv1.ExecutionPromptAction
}

// ExecutionAssistantView is the execution portion of a projected turn. Its
// ordered steps and counts come from the frozen execution snapshot, not a
// current PlanTemplate lookup.
type ExecutionAssistantView struct {
	ConfigurationID            string
	PlanExecutionID            string
	State                      ConversationTurnKind
	StepExecutionID            string
	PlanStepKey                string
	CompletedStepCount         int32
	ActiveStepCount            int32
	PendingInteraction         *PendingInteraction
	LatestArtifactRef          *artifactsv1.ArtifactRef
	Actions                    []*chatv1.ExecutionPromptAction
	OtherActiveExecutionCount  int32
	PlanTemplateSnapshot       *plansv1.PlanTemplate
	ActiveStepKeys             []string
	OrderedActiveSteps         []*plansv1.PlanStep
	DisplayFallback            bool
	ExecutionPromptFingerprint string
}

// ConversationTurn is one deterministic primary turn for a selected
// PlanConfiguration. ConfigurationState is populated only for CONFIGURING;
// Execution is populated only when a PlanExecution is selected or displayed.
type ConversationTurn struct {
	Kind               ConversationTurnKind
	Target             ConversationTarget
	ConfigurationState ConfigurationState
	Execution          *ExecutionAssistantView
}

// TurnInput is a dependency-free snapshot of the state needed to project a
// Conversation turn. Executions must belong to Configuration; PendingInteractions
// are the durable pending rows considered for the resolved execution.
type TurnInput struct {
	Template            *plansv1.PlanTemplate
	Configuration       *plansv1.PlanConfiguration
	Executions          []*plansv1.PlanExecution
	Target              ConversationTarget
	PendingInteractions []PendingInteraction
}

// DeriveConversationTurn resolves focus as exact execution, then latest
// non-terminal execution for display, then configuration. It never resolves an
// action through the display fallback. Contradictory or incomplete pending
// interaction identity fails closed as EXECUTION_NEEDS_ATTENTION.
func DeriveConversationTurn(input TurnInput) (ConversationTurn, error) {
	configuration := input.Configuration
	if configuration == nil || configuration.GetId() == "" {
		return ConversationTurn{}, fmt.Errorf("planassistant: configuration is required")
	}
	if input.Target.ConfigurationID != "" && input.Target.ConfigurationID != configuration.GetId() {
		return ConversationTurn{}, fmt.Errorf("planassistant: target configuration %q does not match selected configuration %q", input.Target.ConfigurationID, configuration.GetId())
	}

	target := ConversationTarget{ConfigurationID: configuration.GetId()}
	execution, displayFallback, err := resolveExecutionTarget(configuration.GetId(), input.Executions, input.Target.PlanExecutionID)
	if err != nil {
		return ConversationTurn{}, err
	}
	if execution == nil {
		return deriveConfigurationTurn(input, target), nil
	}
	target.PlanExecutionID = execution.GetId()
	target.DisplayFallback = displayFallback
	return deriveExecutionTurn(input, target, execution), nil
}

func deriveConfigurationTurn(input TurnInput, target ConversationTarget) ConversationTurn {
	configurationState := DeriveConfigurationState(input.Template, input.Configuration)
	switch input.Configuration.GetStatus() {
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED:
		return ConversationTurn{
			Kind:               ConversationTurnConfiguring,
			Target:             target,
			ConfigurationState: configurationState,
		}
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE:
		return ConversationTurn{Kind: ConversationTurnReadyToRun, Target: target}
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED:
		return ConversationTurn{Kind: ConversationTurnWaitingForSchedule, Target: target}
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DISABLED:
		return ConversationTurn{Kind: ConversationTurnConfigurationDisabled, Target: target}
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_ARCHIVED:
		return ConversationTurn{Kind: ConversationTurnConfigurationArchived, Target: target}
	default:
		return ConversationTurn{Kind: ConversationTurnExecutionNeedsAttention, Target: target}
	}
}

func deriveExecutionTurn(input TurnInput, target ConversationTarget, execution *plansv1.PlanExecution) ConversationTurn {
	view := executionView(execution, target, input.Executions)
	pending, needsAttention := validatePendingInteractions(execution, input.PendingInteractions)
	if needsAttention {
		view.State = ConversationTurnExecutionNeedsAttention
		return ConversationTurn{Kind: view.State, Target: target, Execution: view}
	}
	if isTerminalExecution(execution.GetStatus()) && pending != nil {
		view.State = ConversationTurnExecutionNeedsAttention
		return ConversationTurn{Kind: view.State, Target: target, Execution: view}
	}
	if pending != nil {
		view.PendingInteraction = pending
		view.Actions = pending.Actions
		view.StepExecutionID = pending.StepExecutionID
		view.PlanStepKey = pending.PlanStepKey
		view.LatestArtifactRef = pending.SubjectArtifactRef
		switch pending.Kind {
		case chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_ELICITATION:
			view.State = ConversationTurnAwaitingElicitation
		case chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW:
			view.State = ConversationTurnAwaitingReview
		case chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_APPROVAL:
			view.State = ConversationTurnAwaitingApproval
		default:
			view.State = ConversationTurnExecutionNeedsAttention
		}
		view.ExecutionPromptFingerprint = executionPromptFingerprint(view)
		return ConversationTurn{Kind: view.State, Target: target, Execution: view}
	}

	switch execution.GetStatus() {
	case plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_PENDING:
		view.State = ConversationTurnExecutionQueued
	case plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING:
		if failed := firstStepWithStatus(execution, plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_FAILED); failed != nil {
			view.State = ConversationTurnExecutionFailing
			view.StepExecutionID = failed.GetId()
			view.PlanStepKey = failed.GetPlanStepKey()
		} else {
			view.State = ConversationTurnExecutionRunning
			if running := firstStepWithStatus(execution, plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_RUNNING); running != nil {
				view.StepExecutionID = running.GetId()
				view.PlanStepKey = running.GetPlanStepKey()
			}
		}
	case plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_COMPLETED:
		view.State = ConversationTurnExecutionCompleted
	case plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_FAILED:
		view.State = ConversationTurnExecutionFailed
		if failed := firstStepWithStatus(execution, plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_FAILED); failed != nil {
			view.StepExecutionID = failed.GetId()
			view.PlanStepKey = failed.GetPlanStepKey()
		}
	case plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_CANCELLED:
		view.State = ConversationTurnExecutionCancelled
	default:
		view.State = ConversationTurnExecutionNeedsAttention
	}
	view.ExecutionPromptFingerprint = executionPromptFingerprint(view)
	return ConversationTurn{Kind: view.State, Target: target, Execution: view}
}

func resolveExecutionTarget(configurationID string, executions []*plansv1.PlanExecution, exactExecutionID string) (*plansv1.PlanExecution, bool, error) {
	if exactExecutionID != "" {
		for _, execution := range executions {
			if execution != nil && execution.GetId() == exactExecutionID {
				if execution.GetPlanConfigurationId() != configurationID {
					return nil, false, fmt.Errorf("planassistant: execution %q does not belong to configuration %q", exactExecutionID, configurationID)
				}
				return execution, false, nil
			}
		}
		return nil, false, fmt.Errorf("planassistant: exact execution %q not found", exactExecutionID)
	}

	var latest *plansv1.PlanExecution
	for _, execution := range executions {
		if execution == nil || execution.GetPlanConfigurationId() != configurationID || isTerminalExecution(execution.GetStatus()) {
			continue
		}
		if latest == nil || executionIsLater(execution, latest) {
			latest = execution
		}
	}
	return latest, latest != nil, nil
}

func validatePendingInteractions(execution *plansv1.PlanExecution, interactions []PendingInteraction) (*PendingInteraction, bool) {
	if len(interactions) > 1 {
		return nil, true
	}
	if len(interactions) == 0 {
		if hasAwaitingInteractionStep(execution) {
			return nil, true
		}
		return nil, false
	}

	pending := interactions[0]
	if pending.RequestID == "" || pending.PlanExecutionID == "" || pending.StepExecutionID == "" || pending.PlanStepKey == "" ||
		pending.PlanExecutionID != execution.GetId() ||
		pending.Kind == chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_UNSPECIFIED {
		return nil, true
	}
	step := stepExecutionByID(execution, pending.StepExecutionID)
	if step == nil || (step.GetPlanExecutionId() != "" && step.GetPlanExecutionId() != execution.GetId()) || step.GetPlanStepKey() != pending.PlanStepKey {
		return nil, true
	}
	if !validActions(pending.Actions) {
		return nil, true
	}
	return &pending, false
}

func executionView(execution *plansv1.PlanExecution, target ConversationTarget, executions []*plansv1.PlanExecution) *ExecutionAssistantView {
	orderedSteps := orderedActiveSteps(execution.GetPlanTemplateSnapshot(), execution.GetActiveStepKeys())
	return &ExecutionAssistantView{
		ConfigurationID:           execution.GetPlanConfigurationId(),
		PlanExecutionID:           execution.GetId(),
		CompletedStepCount:        completedStepCount(execution),
		ActiveStepCount:           int32(len(execution.GetActiveStepKeys())),
		LatestArtifactRef:         latestArtifactRef(execution, execution.GetActiveStepKeys()),
		OtherActiveExecutionCount: otherActiveExecutionCount(execution, executions),
		PlanTemplateSnapshot:      execution.GetPlanTemplateSnapshot(),
		ActiveStepKeys:            execution.GetActiveStepKeys(),
		OrderedActiveSteps:        orderedSteps,
		DisplayFallback:           target.DisplayFallback,
	}
}

func orderedActiveSteps(template *plansv1.PlanTemplate, activeKeys []string) []*plansv1.PlanStep {
	if template == nil {
		return nil
	}
	active := make(map[string]bool, len(activeKeys))
	for _, key := range activeKeys {
		active[key] = true
	}
	steps := make([]*plansv1.PlanStep, 0, len(activeKeys))
	for _, step := range template.GetSteps() {
		if step != nil && active[step.GetKey()] {
			steps = append(steps, step)
		}
	}
	return steps
}

func completedStepCount(execution *plansv1.PlanExecution) int32 {
	var count int32
	for _, step := range execution.GetStepExecutions() {
		if step.GetStatus() == plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_COMPLETED || step.GetStatus() == plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_SKIPPED {
			count++
		}
	}
	return count
}

func latestArtifactRef(execution *plansv1.PlanExecution, activeKeys []string) *artifactsv1.ArtifactRef {
	byKey := make(map[string]*plansv1.StepExecution, len(execution.GetStepExecutions()))
	for _, step := range execution.GetStepExecutions() {
		if step != nil {
			byKey[step.GetPlanStepKey()] = step
		}
	}
	for i := len(activeKeys) - 1; i >= 0; i-- {
		if ref := byKey[activeKeys[i]].GetOutputArtifactRef(); ref != nil && ref.GetArtifactVersionId() != "" {
			return ref
		}
	}
	return nil
}

func otherActiveExecutionCount(focus *plansv1.PlanExecution, executions []*plansv1.PlanExecution) int32 {
	var count int32
	for _, execution := range executions {
		if execution != nil && execution.GetId() != focus.GetId() && execution.GetPlanConfigurationId() == focus.GetPlanConfigurationId() && !isTerminalExecution(execution.GetStatus()) {
			count++
		}
	}
	return count
}

func hasAwaitingInteractionStep(execution *plansv1.PlanExecution) bool {
	for _, step := range execution.GetStepExecutions() {
		if step.GetStatus() == plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_AWAITING_ELICITATION || step.GetStatus() == plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_AWAITING_APPROVAL {
			return true
		}
	}
	return false
}

func stepExecutionByID(execution *plansv1.PlanExecution, id string) *plansv1.StepExecution {
	for _, step := range execution.GetStepExecutions() {
		if step != nil && step.GetId() == id {
			return step
		}
	}
	return nil
}

func firstStepWithStatus(execution *plansv1.PlanExecution, status plansv1.StepExecutionStatus) *plansv1.StepExecution {
	for _, step := range execution.GetStepExecutions() {
		if step != nil && step.GetStatus() == status {
			return step
		}
	}
	return nil
}

func validActions(actions []*chatv1.ExecutionPromptAction) bool {
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		if action == nil || action.GetActionId() == "" || action.GetLabelKey() == "" || seen[action.GetActionId()] {
			return false
		}
		seen[action.GetActionId()] = true
	}
	return true
}

func isTerminalExecution(status plansv1.PlanExecutionStatus) bool {
	return status == plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_COMPLETED ||
		status == plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_FAILED ||
		status == plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_CANCELLED
}

func executionIsLater(a, b *plansv1.PlanExecution) bool {
	aTime, aOK := executionTimestamp(a)
	bTime, bOK := executionTimestamp(b)
	if aOK && bOK && !aTime.Equal(bTime) {
		return aTime.After(bTime)
	}
	if aOK != bOK {
		return aOK
	}
	return a.GetId() > b.GetId()
}

func executionTimestamp(execution *plansv1.PlanExecution) (time.Time, bool) {
	for _, value := range []string{execution.GetUpdatedAt(), execution.GetCreatedAt()} {
		if value == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func executionPromptFingerprint(view *ExecutionAssistantView) string {
	interactionKind := chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_UNSPECIFIED.String()
	requestID := ""
	artifactVersionID := ""
	if view.PendingInteraction != nil {
		interactionKind = view.PendingInteraction.Kind.String()
		requestID = view.PendingInteraction.RequestID
		if view.PendingInteraction.SubjectArtifactRef != nil {
			artifactVersionID = view.PendingInteraction.SubjectArtifactRef.GetArtifactVersionId()
		}
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", view.PlanExecutionID, view.State.String(), view.StepExecutionID, interactionKind, requestID, artifactVersionID)
}
