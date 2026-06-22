package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"

	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/workflow"
)

type RuntimeRepository struct {
	chat      chat.Store
	plans     *Repository
	executors ExecutorLookup
}

func NewRuntimeRepository(planRepo *Repository, executors ExecutorLookup, chatStore chat.Store) *RuntimeRepository {
	return &RuntimeRepository{
		chat:      chatStore,
		plans:     planRepo,
		executors: executors,
	}
}

func (r *RuntimeRepository) CreateScheduledExecution(ctx context.Context, tenantID, configID, executionID uuid.UUID) (workflow.PlanWorkflowInput, error) {
	if r == nil || r.plans == nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("plan runtime repository is not configured")
	}

	config, err := r.plans.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("get plan configuration: %w", err)
	}
	if config.Status != ConfigurationStatusScheduled {
		return workflow.PlanWorkflowInput{}, nil
	}

	template, err := r.plans.GetTemplateByID(ctx, config.PlanTemplateID)
	if err != nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("get plan template: %w", err)
	}
	if err := NewBindingValidator(r.executors).ValidateConfigurationForExecution(ctx, tenantID, template, config); err != nil {
		return workflow.PlanWorkflowInput{}, err
	}
	if executionID == uuid.Nil {
		executionID = uuid.New()
	}

	snapshot, err := buildPlanExecutionSnapshot(ctx, tenantID, config, template, r.executors, time.Now().UTC())
	if err != nil {
		return workflow.PlanWorkflowInput{}, err
	}
	rawSnapshot, err := marshalPlanExecutionSnapshot(snapshot)
	if err != nil {
		return workflow.PlanWorkflowInput{}, err
	}

	now := time.Now().UTC()
	execution, err := r.plans.CreateExecution(ctx, &PlanExecution{
		ID:                        executionID,
		TenantID:                  tenantID,
		PlanConfigurationID:       configID,
		PlanConfigurationSnapshot: rawSnapshot,
		Status:                    ExecutionStatusPending,
		TriggeredAt:               &now,
	})
	if err != nil {
		return workflow.PlanWorkflowInput{}, fmt.Errorf("create scheduled plan execution: %w", err)
	}

	return workflow.PlanWorkflowInput{
		TenantID:        tenantID.String(),
		PlanExecutionID: execution.ID.String(),
	}, nil
}

func (r *RuntimeRepository) LoadPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) (workflow.LoadedPlanExecution, error) {
	if r == nil || r.plans == nil {
		return workflow.LoadedPlanExecution{}, fmt.Errorf("plan runtime repository is not configured")
	}

	execution, err := r.plans.GetExecution(ctx, tenantID, executionID)
	if err != nil {
		return workflow.LoadedPlanExecution{}, err
	}
	snapshot, err := unmarshalPlanExecutionSnapshot(execution.PlanConfigurationSnapshot)
	if err != nil {
		return workflow.LoadedPlanExecution{}, err
	}
	return workflow.LoadedPlanExecution{
		PlanExecutionID: execution.ID.String(),
		Status:          execution.Status,
		Snapshot:        snapshot,
	}, nil
}

func (r *RuntimeRepository) StartPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	if err := r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusRunning, nil); err != nil {
		return err
	}
	if r.chat != nil {
		execID := executionID
		configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_STARTED,
				Text:        "Run started.",
				PayloadJSON: chat.BuildRunStartedPayload(),
			})
		}
	}
	return nil
}

func (r *RuntimeRepository) CreateStepExecution(ctx context.Context, input workflow.CreateStepExecutionInput) (workflow.StepExecutionRecord, error) {
	tenantID, executionID, err := parseRuntimeTenantExecution(input.TenantID, input.PlanExecutionID)
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}
	rawInstallation, err := json.Marshal(input.ExecutorInstallationSnapshot)
	if err != nil {
		return workflow.StepExecutionRecord{}, fmt.Errorf("marshal executor installation snapshot: %w", err)
	}

	step, err := r.plans.CreateStepExecution(ctx, &StepExecution{
		TenantID:                     tenantID,
		PlanExecutionID:              executionID,
		PlanStepKey:                  input.PlanStepKey,
		Status:                       StepStatusRunning,
		InputArtifactID:              input.InputArtifactID,
		ExecutorInstallationSnapshot: rawInstallation,
	})
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}

	if r.chat != nil {
		execID := executionID
		configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_STARTED,
				Text:        "Step " + input.PlanStepKey + " started.",
				PayloadJSON: chat.BuildStepStartedPayload(input.PlanStepKey, step.ID.String()),
			})
		}
	}

	return workflow.StepExecutionRecord{
		ID:              step.ID.String(),
		PlanStepKey:     step.PlanStepKey,
		Attempt:         step.Attempt,
		InputArtifactID: step.InputArtifactID,
	}, nil
}

func (r *RuntimeRepository) ResumeStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusRunning, "", "", "")
}

func (r *RuntimeRepository) CompleteStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	if err := r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusCompleted, input.OutputArtifactID, "", ""); err != nil {
		return err
	}
	if r.chat != nil {
		planExecutionID, parseErr := uuid.Parse(strings.TrimSpace(input.PlanExecutionID))
		if parseErr == nil {
			execID := planExecutionID
			configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, planExecutionID)
			if lookupErr == nil {
				_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
					ThreadID:    configID.String(),
					ExecutionID: &execID,
					Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
					Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_BOUND,
					Text:        "Step " + input.PlanStepKey + " completed.",
					PayloadJSON: chat.BuildStepBoundPayload(input.PlanStepKey, input.OutputArtifactID),
				})
			}
		}
	}
	return nil
}

func (r *RuntimeRepository) FailStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusFailed, "", "", "")
}

func (r *RuntimeRepository) AwaitElicitationStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	if err := r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusAwaitingElicitation, "", input.ElicitationThreadID, ""); err != nil {
		return err
	}
	return r.persistElicitation(ctx, tenantID, stepID, input)
}

func (r *RuntimeRepository) persistElicitation(ctx context.Context, tenantID, stepID uuid.UUID, input workflow.StepStatusUpdateInput) error {
	threadID := strings.TrimSpace(input.ElicitationThreadID)
	planExecutionID := strings.TrimSpace(input.PlanExecutionID)
	if threadID == "" || planExecutionID == "" {
		// Without a thread id or plan execution id we cannot durably scope the
		// elicitation thread; the step status update above is sufficient.
		return nil
	}
	executionID, err := uuid.Parse(planExecutionID)
	if err != nil {
		return fmt.Errorf("parse plan execution id: %w", err)
	}

	var expiresAt *time.Time
	if trimmed := strings.TrimSpace(input.ElicitationExpiresAt); trimmed != "" {
		parsed, parseErr := time.Parse(time.RFC3339, trimmed)
		if parseErr == nil {
			expiresAt = &parsed
		}
	}

	var schema json.RawMessage
	if trimmed := strings.TrimSpace(input.ElicitationSchemaJSON); trimmed != "" {
		schema = json.RawMessage(trimmed)
	}

	overseer := r.resolveOverseer(ctx, tenantID, executionID, input.PlanStepKey)

	created, err := r.plans.UpsertElicitation(ctx, &Elicitation{
		TenantID:            tenantID,
		PlanExecutionID:     executionID,
		StepExecutionID:     stepID,
		PlanStepKey:         input.PlanStepKey,
		ElicitationThreadID: threadID,
		Status:              ElicitationStatusPending,
		Prompt:              input.ElicitationPrompt,
		SchemaJSON:          schema,
		TimeoutBehavior:     input.ElicitationTimeout,
		OverseerUserID:      overseer,
		ExpiresAt:           expiresAt,
	})
	if err != nil {
		return err
	}
	if r.chat != nil {
		execID := executionID
		configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ELICITATION_RAISED,
				Text:        "Elicitation raised on step " + input.PlanStepKey + ".",
				PayloadJSON: chat.BuildElicitationRaisedPayload(created.ID),
			})
		}
	}
	return nil
}

// resolveOverseer looks up the overseer bound to the step from the execution
// snapshot so the elicitation can be addressed and authorized. A missing binding
// is non-fatal (leaders can still respond).
func (r *RuntimeRepository) resolveOverseer(ctx context.Context, tenantID, executionID uuid.UUID, stepKey string) uuid.NullUUID {
	execution, err := r.plans.GetExecution(ctx, tenantID, executionID)
	if err != nil {
		return uuid.NullUUID{}
	}
	snapshot, err := unmarshalPlanExecutionSnapshot(execution.PlanConfigurationSnapshot)
	if err != nil || snapshot.Configuration == nil {
		return uuid.NullUUID{}
	}
	for _, binding := range snapshot.Configuration.GetOverseerBindings() {
		if binding == nil {
			continue
		}
		if strings.TrimSpace(binding.GetStepKey()) != strings.TrimSpace(stepKey) {
			continue
		}
		parsed, parseErr := uuid.Parse(strings.TrimSpace(binding.GetOverseerUserId()))
		if parseErr != nil {
			return uuid.NullUUID{}
		}
		return uuid.NullUUID{UUID: parsed, Valid: true}
	}
	return uuid.NullUUID{}
}

func (r *RuntimeRepository) TimeoutElicitationStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.MarkElicitationTimedOutByStep(ctx, tenantID, stepID, strings.TrimSpace(input.ElicitationThreadID))
}

func (r *RuntimeRepository) CreateApprovalRequest(ctx context.Context, input workflow.CreateApprovalRequestInput) error {
	tenantID, executionID, err := parseRuntimeTenantExecution(input.TenantID, input.PlanExecutionID)
	if err != nil {
		return err
	}
	stepID, err := uuid.Parse(input.StepExecutionID)
	if err != nil {
		return fmt.Errorf("parse step execution id: %w", err)
	}
	if err := r.plans.CreatePlanApprovalRequest(ctx, &PlanApprovalRequest{
		ID:              input.ApprovalRequestID,
		TenantID:        tenantID,
		PlanExecutionID: executionID,
		StepExecutionID: stepID,
		PlanStepKey:     input.PlanStepKey,
		InputArtifactID: input.InputArtifactID,
		Status:          ApprovalRequestStatusPending,
	}); err != nil {
		return err
	}
	if r.chat != nil {
		execID := executionID
		configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_APPROVAL_RAISED,
				Text:        "Approval required on step " + input.PlanStepKey + ".",
				PayloadJSON: chat.BuildApprovalRaisedPayload(input.ApprovalRequestID),
			})
		}
	}
	return r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusAwaitingApproval, "", "", input.ApprovalRequestID)
}

func (r *RuntimeRepository) ResolveApprovalRequest(ctx context.Context, input workflow.ResolveApprovalRequestInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	return r.plans.ResolvePlanApprovalRequest(ctx, tenantID, input.ApprovalRequestID, stepID, input.Approved, input.Reason)
}

func (r *RuntimeRepository) CompletePlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	now := time.Now().UTC()
	if err := r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusCompleted, &now); err != nil {
		return err
	}
	r.appendRunFinishedChatMessage(ctx, tenantID, executionID)
	return nil
}

func (r *RuntimeRepository) FailPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	now := time.Now().UTC()
	if err := r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusFailed, &now); err != nil {
		return err
	}
	r.appendRunFinishedChatMessage(ctx, tenantID, executionID)
	return nil
}

func (r *RuntimeRepository) appendRunFinishedChatMessage(ctx context.Context, tenantID, executionID uuid.UUID) {
	if r.chat == nil {
		return
	}
	execID := executionID
	exec, lookupErr := r.plans.GetExecution(ctx, tenantID, executionID)
	if lookupErr != nil {
		return
	}
	configID := exec.PlanConfigurationID
	var kind chatv1.ThreadMessageKind
	var text string
	var payload string
	if exec.Status == ExecutionStatusFailed {
		kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_FAILED
		failureReason := executionFailureReason(exec)
		text = "Run failed."
		if failureReason != "" {
			text = "Run failed: " + failureReason + "."
		}
		payload = chat.BuildRunFailedPayload(failureReason)
	} else {
		kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_COMPLETED
		text = "Run completed."
		payload = chat.BuildRunCompletedPayload()
	}
	_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    configID.String(),
		ExecutionID: &execID,
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
		Kind:        kind,
		Text:        text,
		PayloadJSON: payload,
	})
}

// executionFailureReason derives a short failure summary from failed step executions.
// PlanExecution has no dedicated failure_reason column; the latest failed step key
// is the best signal available for RUN_FAILED payload text.
func executionFailureReason(exec *PlanExecution) string {
	if exec == nil {
		return ""
	}
	var failedStep *StepExecution
	for i := range exec.StepExecutions {
		step := &exec.StepExecutions[i]
		if step.Status != StepStatusFailed {
			continue
		}
		if failedStep == nil || step.CreatedAt.After(failedStep.CreatedAt) {
			failedStep = step
		}
	}
	if failedStep == nil {
		return ""
	}
	return "step " + failedStep.PlanStepKey + " failed"
}

func (r *RuntimeRepository) PrepareRetryFromStep(
	ctx context.Context,
	tenantID uuid.UUID,
	planExecutionID uuid.UUID,
	failedStepExecutionID uuid.UUID,
) (*PlanExecution, workflow.PlanWorkflowInput, error) {
	if r == nil || r.plans == nil {
		return nil, workflow.PlanWorkflowInput{}, fmt.Errorf("plan runtime repository is not configured")
	}

	execution, err := r.plans.GetExecution(ctx, tenantID, planExecutionID)
	if err != nil {
		return nil, workflow.PlanWorkflowInput{}, fmt.Errorf("load plan execution: %w", err)
	}
	if execution.Status != ExecutionStatusFailed {
		return nil, workflow.PlanWorkflowInput{}, fmt.Errorf("plan execution status %q is not failed", execution.Status)
	}

	snapshot, err := unmarshalPlanExecutionSnapshot(execution.PlanConfigurationSnapshot)
	if err != nil {
		return nil, workflow.PlanWorkflowInput{}, fmt.Errorf("load plan execution snapshot: %w", err)
	}
	retryPreparation, err := buildRetryPlanWorkflowInput(snapshot, execution, failedStepExecutionID)
	if err != nil {
		return nil, workflow.PlanWorkflowInput{}, err
	}

	now := time.Now().UTC()
	retryExecution, err := r.plans.CreateExecution(ctx, &PlanExecution{
		ID:                        retryPreparation.retryExecutionID,
		TenantID:                  tenantID,
		PlanConfigurationID:       execution.PlanConfigurationID,
		PlanConfigurationSnapshot: execution.PlanConfigurationSnapshot,
		Status:                    ExecutionStatusPending,
		TriggeredAt:               &now,
	})
	if err != nil {
		return nil, workflow.PlanWorkflowInput{}, fmt.Errorf("create retry plan execution: %w", err)
	}

	return retryExecution, workflow.PlanWorkflowInput{
		TenantID:                tenantID.String(),
		PlanExecutionID:         retryExecution.ID.String(),
		RetryFromStepKey:        retryPreparation.retryFromStepKey,
		RetryStepArtifactsByKey: retryPreparation.reusedArtifactsByStep,
	}, nil
}

type retryPlanInput struct {
	retryExecutionID      uuid.UUID
	retryFromStepKey      string
	reusedArtifactsByStep map[string]workflow.ArtifactRef
}

func buildRetryPlanWorkflowInput(
	snapshot workflow.PlanExecutionSnapshot,
	execution *PlanExecution,
	failedStepExecutionID uuid.UUID,
) (retryPlanInput, error) {
	if execution == nil {
		return retryPlanInput{}, fmt.Errorf("plan execution is required")
	}
	if execution.Status != ExecutionStatusFailed {
		return retryPlanInput{}, fmt.Errorf("plan execution status %q is not failed", execution.Status)
	}
	order, err := workflowTopologicalSteps(snapshot.Template)
	if err != nil {
		return retryPlanInput{}, err
	}

	failedStep, ok := findExecutionStepByID(execution.StepExecutions, failedStepExecutionID)
	if !ok {
		return retryPlanInput{}, fmt.Errorf("step execution %q does not belong to plan execution %q", failedStepExecutionID, execution.ID)
	}
	if failedStep.Status != StepStatusFailed {
		return retryPlanInput{}, fmt.Errorf("step execution %q status %q is not failed", failedStepExecutionID, failedStep.Status)
	}
	retryStepKey := strings.TrimSpace(failedStep.PlanStepKey)
	if retryStepKey == "" {
		return retryPlanInput{}, fmt.Errorf("step execution %q has empty plan step key", failedStepExecutionID)
	}

	retryIndex := -1
	stepByKey := make(map[string]*plansStepTemplate, len(order))
	for i, step := range order {
		stepByKey[step.Key] = step
		if step.Key == retryStepKey {
			retryIndex = i
		}
	}
	if retryIndex < 0 {
		return retryPlanInput{}, fmt.Errorf("retry step %q is not part of the snapshot template", retryStepKey)
	}
	if retryIndex == 0 {
		return retryPlanInput{
			retryExecutionID:      retryExecutionID(execution.ID, failedStepExecutionID),
			retryFromStepKey:      retryStepKey,
			reusedArtifactsByStep: map[string]workflow.ArtifactRef{},
		}, nil
	}

	latestByStep := latestStepAttempts(execution.StepExecutions)
	reusedArtifacts := make(map[string]workflow.ArtifactRef, retryIndex)
	for _, step := range order[:retryIndex] {
		attempt, exists := latestByStep[step.Key]
		if !exists {
			return retryPlanInput{}, fmt.Errorf("cannot retry from step %q: upstream step %q has no execution attempt", retryStepKey, step.Key)
		}
		if attempt.Status != StepStatusCompleted {
			return retryPlanInput{}, fmt.Errorf("cannot retry from step %q: upstream step %q status is %q", retryStepKey, step.Key, attempt.Status)
		}
		artifactID := strings.TrimSpace(attempt.OutputArtifactID)
		if artifactID == "" {
			return retryPlanInput{}, fmt.Errorf("cannot retry from step %q: upstream step %q has no output artifact", retryStepKey, step.Key)
		}
		reusedArtifacts[step.Key] = workflow.ArtifactRef{
			Source:          "step_output",
			StepKey:         step.Key,
			ArtifactID:      artifactID,
			ArtifactTypeKey: step.OutputArtifactTypeID,
		}
	}

	return retryPlanInput{
		retryExecutionID:      retryExecutionID(execution.ID, failedStepExecutionID),
		retryFromStepKey:      retryStepKey,
		reusedArtifactsByStep: reusedArtifacts,
	}, nil
}

type plansStepTemplate struct {
	Key                  string
	OutputArtifactTypeID string
	Position             int
}

func workflowTopologicalSteps(template *plansv1.PlanTemplate) ([]*plansStepTemplate, error) {
	if template == nil {
		return nil, fmt.Errorf("plan template is required")
	}
	steps := make([]*plansStepTemplate, 0, len(template.Steps))
	indexByKey := make(map[string]int, len(template.Steps))
	for idx, step := range template.Steps {
		if step == nil {
			return nil, fmt.Errorf("plan template step at index %d is nil", idx)
		}
		key := strings.TrimSpace(step.Key)
		if key == "" {
			return nil, fmt.Errorf("plan template step at index %d has empty key", idx)
		}
		if _, exists := indexByKey[key]; exists {
			return nil, fmt.Errorf("duplicate plan step key %q", key)
		}
		indexByKey[key] = idx
		steps = append(steps, &plansStepTemplate{Key: key, OutputArtifactTypeID: step.OutputArtifactTypeId, Position: idx})
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("plan template has no steps")
	}

	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps))
	for _, step := range steps {
		inDegree[step.Key] = 0
	}
	for _, edge := range template.Edges {
		if edge == nil {
			continue
		}
		from := strings.TrimSpace(edge.FromStepKey)
		to := strings.TrimSpace(edge.ToStepKey)
		if _, ok := inDegree[from]; !ok {
			return nil, fmt.Errorf("dependency references unknown from_step_key %q", from)
		}
		if _, ok := inDegree[to]; !ok {
			return nil, fmt.Errorf("dependency references unknown to_step_key %q", to)
		}
		dependents[from] = append(dependents[from], to)
		inDegree[to]++
	}

	stepByKey := make(map[string]*plansStepTemplate, len(steps))
	for _, step := range steps {
		stepByKey[step.Key] = step
	}
	ready := make([]string, 0, len(steps))
	for key, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, key)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool {
		return indexByKey[ready[i]] < indexByKey[ready[j]]
	})

	ordered := make([]*plansStepTemplate, 0, len(steps))
	for len(ready) > 0 {
		key := ready[0]
		ready = ready[1:]
		ordered = append(ordered, stepByKey[key])
		for _, dependent := range dependents[key] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.SliceStable(ready, func(i, j int) bool {
					return indexByKey[ready[i]] < indexByKey[ready[j]]
				})
			}
		}
	}
	if len(ordered) != len(steps) {
		return nil, fmt.Errorf("plan template dependency graph contains a cycle")
	}
	return ordered, nil
}

func latestStepAttempts(steps []StepExecution) map[string]StepExecution {
	latest := make(map[string]StepExecution, len(steps))
	for _, step := range steps {
		current, exists := latest[step.PlanStepKey]
		if !exists || step.Attempt > current.Attempt || (step.Attempt == current.Attempt && step.CreatedAt.After(current.CreatedAt)) {
			latest[step.PlanStepKey] = step
		}
	}
	return latest
}

func findExecutionStepByID(steps []StepExecution, stepExecutionID uuid.UUID) (StepExecution, bool) {
	for _, step := range steps {
		if step.ID == stepExecutionID {
			return step, true
		}
	}
	return StepExecution{}, false
}

func retryExecutionID(planExecutionID uuid.UUID, failedStepExecutionID uuid.UUID) uuid.UUID {
	seed := fmt.Sprintf("retry:%s:%s", planExecutionID.String(), failedStepExecutionID.String())
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(seed))
}

func parseRuntimeTenantExecution(tenantIDRaw, executionIDRaw string) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(tenantIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse tenant id: %w", err)
	}
	executionID, err := uuid.Parse(executionIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse plan execution id: %w", err)
	}
	return tenantID, executionID, nil
}

func parseRuntimeTenantStep(tenantIDRaw, stepIDRaw string) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(tenantIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse tenant id: %w", err)
	}
	stepID, err := uuid.Parse(stepIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse step execution id: %w", err)
	}
	return tenantID, stepID, nil
}
