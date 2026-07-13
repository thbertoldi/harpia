package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/jackc/pgx/v5"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/workflow"
)

type RuntimeRepository struct {
	chat                      chat.Store
	plans                     runtimePlanStore
	executors                 ExecutorLookup
	artifacts                 ArtifactVersionVerifier
	executionConversationSink ExecutionConversationSink
}

// ExecutionConversationSink receives a notification after durable execution
// state changes. It is deliberately best effort; its projection can always be
// rebuilt from PlanExecution and pending interaction rows.
type ExecutionConversationSink interface {
	OnExecutionEvent(ctx context.Context, tenantID, executionID uuid.UUID, causeKind string) error
}

type ArtifactVersionVerifier interface {
	GetArtifactVersion(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*artifacts.ArtifactVersion, error)
}

type runtimePlanStore interface {
	GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*PlanConfiguration, error)
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*PlanTemplate, error)
	CreateExecution(ctx context.Context, execution *PlanExecution) (*PlanExecution, error)
	GetExecution(ctx context.Context, tenantID, executionID uuid.UUID) (*PlanExecution, error)
	UpdateExecutionStatus(ctx context.Context, tenantID, executionID uuid.UUID, status string, completedAt *time.Time) error
	GetPlanConfigurationIDForExecution(ctx context.Context, tenantID, executionID uuid.UUID) (uuid.UUID, error)
	CreateStepExecution(ctx context.Context, step *StepExecution) (*StepExecution, error)
	GetStepExecution(ctx context.Context, tenantID, stepID uuid.UUID) (*StepExecution, error)
	UpdateStepExecutionStatus(ctx context.Context, tenantID, stepID uuid.UUID, status, outputArtifactID, elicitationThreadID, approvalRequestID string) error
	UpdateStepExecutionRefs(ctx context.Context, tenantID, stepExecutionID uuid.UUID, status string, inputRef, outputRef *StepExecution) error
	UpsertElicitation(ctx context.Context, elicitation *Elicitation) (*Elicitation, error)
	MarkElicitationTimedOutByStep(ctx context.Context, tenantID, stepID uuid.UUID, threadID string) error
	CreatePlanApprovalRequest(ctx context.Context, request *PlanApprovalRequest) error
	ResolvePlanApprovalRequest(ctx context.Context, tenantID uuid.UUID, requestID string, stepExecutionID uuid.UUID, approved bool, reason string) error
	GetPlanApprovalRequest(ctx context.Context, tenantID uuid.UUID, approvalID string) (*PlanApprovalRequest, error)
	CreatePlanReviewRequest(ctx context.Context, request *PlanReviewRequest) (*PlanReviewRequest, error)
	GetPlanReviewRequest(ctx context.Context, tenantID, reviewID uuid.UUID) (*PlanReviewRequest, error)
	MarkReviewDecided(ctx context.Context, tenantID, reviewID uuid.UUID, decision, feedback string, decidedBy uuid.NullUUID) (*PlanReviewRequest, error)
}

func NewRuntimeRepository(planRepo *Repository, executors ExecutorLookup, chatStore chat.Store, artifactVersions ...ArtifactVersionVerifier) *RuntimeRepository {
	var verifier ArtifactVersionVerifier
	if len(artifactVersions) > 0 {
		verifier = artifactVersions[0]
	}
	return &RuntimeRepository{
		chat:      chatStore,
		plans:     planRepo,
		executors: executors,
		artifacts: verifier,
	}
}

// WithExecutionConversationSink injects the passive execution conversation
// driver at composition time without coupling the runtime package to its
// planassistant adapter.
func (r *RuntimeRepository) WithExecutionConversationSink(sink ExecutionConversationSink) *RuntimeRepository {
	if r != nil {
		r.executionConversationSink = sink
	}
	return r
}

func (r *RuntimeRepository) notifyExecutionConversation(ctx context.Context, tenantID, executionID uuid.UUID, causeKind string) {
	if r == nil || r.executionConversationSink == nil {
		return
	}
	if err := r.executionConversationSink.OnExecutionEvent(ctx, tenantID, executionID, causeKind); err != nil {
		slog.Warn("execution conversation projection failed", "tenant_id", tenantID, "execution_id", executionID, "cause_kind", causeKind, "error", err)
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
		if lookupErr != nil {
			return fmt.Errorf("resolve plan configuration for execution: %w", lookupErr)
		}
		threadID, err := r.owningThreadID(ctx, tenantID, configID)
		if err != nil {
			return err
		}
		_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    threadID,
			ExecutionID: &execID,
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_STARTED,
			Text:        "Run started.",
			PayloadJSON: chat.BuildRunStartedPayload(),
		})
	}
	r.notifyExecutionConversation(ctx, tenantID, executionID, "RUN_STARTED")
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
		InputArtifactVersionID:       nullableRuntimeUUID(input.InputArtifactRef.ArtifactVersionID),
		InputArtifactTypeKey:         input.InputArtifactRef.ArtifactTypeKey,
		InputContentHash:             input.InputArtifactRef.ContentHash,
		ExecutorInstallationSnapshot: rawInstallation,
	})
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}

	if r.chat != nil && step.Attempt == 1 {
		execID := executionID
		configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr != nil {
			return workflow.StepExecutionRecord{}, fmt.Errorf("resolve plan configuration for execution: %w", lookupErr)
		}
		threadID, err := r.owningThreadID(ctx, tenantID, configID)
		if err != nil {
			return workflow.StepExecutionRecord{}, err
		}
		if !r.stepStartedChatMessageExists(ctx, tenantID, threadID, step.ID.String()) {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    threadID,
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

// CreateSkippedStepExecution records a SKIPPED step row for a branch the run did
// not select (e.g. a non-selected content_output_format branch). Skipped steps
// carry no input artifact and emit no chat message — they are silent audit rows.
func (r *RuntimeRepository) CreateSkippedStepExecution(ctx context.Context, input workflow.CreateSkippedStepInput) (workflow.StepExecutionRecord, error) {
	if r == nil || r.plans == nil {
		return workflow.StepExecutionRecord{}, fmt.Errorf("plan runtime repository is not configured")
	}
	tenantID, executionID, err := parseRuntimeTenantExecution(input.TenantID, input.PlanExecutionID)
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}
	step, err := r.plans.CreateStepExecution(ctx, &StepExecution{
		TenantID:                tenantID,
		PlanExecutionID:         executionID,
		PlanStepKey:             input.PlanStepKey,
		Status:                  StepStatusSkipped,
		InputArtifactID:         input.ArtifactRef.ArtifactID,
		InputArtifactVersionID:  nullableRuntimeUUID(input.ArtifactRef.ArtifactVersionID),
		InputArtifactTypeKey:    input.ArtifactRef.ArtifactTypeKey,
		InputContentHash:        input.ArtifactRef.ContentHash,
		OutputArtifactID:        input.ArtifactRef.ArtifactID,
		OutputArtifactVersionID: nullableRuntimeUUID(input.ArtifactRef.ArtifactVersionID),
		OutputArtifactTypeKey:   input.ArtifactRef.ArtifactTypeKey,
		OutputContentHash:       input.ArtifactRef.ContentHash,
		// The step_executions.executor_installation_snapshot column is NOT NULL,
		// but a skipped branch step runs no executor. Record a self-documenting
		// placeholder so the row satisfies the constraint without implying an
		// installation exists.
		ExecutorInstallationSnapshot: json.RawMessage(`{"skipped":true}`),
	})
	if err != nil {
		return workflow.StepExecutionRecord{}, err
	}
	return workflow.StepExecutionRecord{ID: step.ID.String(), PlanStepKey: step.PlanStepKey, Attempt: step.Attempt}, nil
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
	output := &StepExecution{OutputArtifactID: input.OutputArtifactRef.ArtifactID, OutputArtifactVersionID: nullableRuntimeUUID(input.OutputArtifactRef.ArtifactVersionID), OutputArtifactTypeKey: input.OutputArtifactRef.ArtifactTypeKey, OutputContentHash: input.OutputArtifactRef.ContentHash}
	if output.OutputArtifactID == "" {
		output.OutputArtifactID = input.OutputArtifactID
	}
	if err := r.plans.UpdateStepExecutionRefs(ctx, tenantID, stepID, StepStatusCompleted, &StepExecution{}, output); err != nil {
		return err
	}
	if r.chat != nil {
		planExecutionID, parseErr := uuid.Parse(strings.TrimSpace(input.PlanExecutionID))
		if parseErr == nil {
			execID := planExecutionID
			configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, planExecutionID)
			if lookupErr != nil {
				return fmt.Errorf("resolve plan configuration for execution: %w", lookupErr)
			}
			threadID, err := r.owningThreadID(ctx, tenantID, configID)
			if err != nil {
				return err
			}
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    threadID,
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_BOUND,
				Text:        "Step " + input.PlanStepKey + " completed.",
				PayloadJSON: chat.BuildStepBoundPayloadRef(input.PlanStepKey, output.OutputArtifactID, nullableUUIDString(output.OutputArtifactVersionID), output.OutputArtifactTypeKey, output.OutputContentHash),
			})
		}
	}
	if executionID, parseErr := uuid.Parse(strings.TrimSpace(input.PlanExecutionID)); parseErr == nil {
		r.notifyExecutionConversation(ctx, tenantID, executionID, "STEP_BOUND")
	}
	return nil
}

func (r *RuntimeRepository) CreateReviewRequest(ctx context.Context, input workflow.CreateReviewRequestInput) (string, error) {
	tenantID, executionID, err := parseRuntimeTenantExecution(input.TenantID, input.PlanExecutionID)
	if err != nil {
		return "", err
	}
	stepID, err := uuid.Parse(input.StepExecutionID)
	if err != nil {
		return "", err
	}
	ref := input.SubjectArtifactRef
	artifactID, err := uuid.Parse(ref.ArtifactID)
	if err != nil {
		return "", err
	}
	versionID, err := uuid.Parse(ref.ArtifactVersionID)
	if err != nil {
		return "", err
	}
	review, err := r.plans.CreatePlanReviewRequest(ctx, &PlanReviewRequest{TenantID: tenantID, PlanExecutionID: executionID, StepExecutionID: stepID, PlanStepKey: input.PlanStepKey, SubjectArtifactID: artifactID, SubjectArtifactVersionID: versionID, SubjectArtifactTypeKey: ref.ArtifactTypeKey, SubjectContentHash: ref.ContentHash, OverseerUserID: r.resolveOverseer(ctx, tenantID, executionID, input.PlanStepKey)})
	if err != nil {
		return "", err
	}
	if r.chat != nil {
		if configID, err := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID); err == nil {
			if threadID, err := r.owningThreadID(ctx, tenantID, configID); err == nil {
				execID := executionID
				_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{ThreadID: threadID, ExecutionID: &execID, Role: chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM, Kind: chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_REVIEW_RAISED, Text: "Review required on step " + input.PlanStepKey + ".", PayloadJSON: chat.BuildReviewRaisedPayload(review.ID)})
			}
		}
	}
	r.notifyExecutionConversation(ctx, tenantID, executionID, "REVIEW_RAISED")
	return review.ID.String(), nil
}

func (r *RuntimeRepository) ResolveReviewRequest(ctx context.Context, input workflow.ResolveReviewRequestInput) error {
	tenantID, _, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	reviewID, err := uuid.Parse(input.ReviewRequestID)
	if err != nil {
		return err
	}
	review, err := r.plans.GetPlanReviewRequest(ctx, tenantID, reviewID)
	if err != nil {
		return err
	}
	stepID, err := uuid.Parse(input.StepExecutionID)
	if err != nil {
		return err
	}
	if review.StepExecutionID != stepID {
		return fmt.Errorf("review request does not belong to step execution")
	}
	updated, err := r.plans.MarkReviewDecided(ctx, tenantID, reviewID, input.Decision, input.Feedback, uuid.NullUUID{})
	if err != nil {
		// A replay or duplicate Temporal signal sees the already-terminal row.
		// It is safe to return without a second REVIEW_DECIDED event.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if r.chat != nil {
		if configID, err := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, review.PlanExecutionID); err == nil {
			if threadID, err := r.owningThreadID(ctx, tenantID, configID); err == nil {
				execID := review.PlanExecutionID
				_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{ThreadID: threadID, ExecutionID: &execID, Role: chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM, Kind: chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_REVIEW_DECIDED, Text: "Review decided.", PayloadJSON: chat.BuildReviewDecidedPayload(reviewID, input.Decision)})
			}
		}
	}
	_ = updated // terminal write is intentionally owned by this activity path.
	r.notifyExecutionConversation(ctx, tenantID, review.PlanExecutionID, "REVIEW_DECIDED")
	return nil
}

func nullableRuntimeUUID(raw string) uuid.NullUUID {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}

func (r *RuntimeRepository) FailStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	if err := r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusFailed, "", "", ""); err != nil {
		return err
	}
	if r.chat != nil {
		// Surface the failure in the chat thread so the operator sees which
		// step failed instead of a silent "nothing happened". The step key and
		// execution id come from the step-execution row (the workflow does not
		// pass them on FailStepExecutionActivity). The error summary is kept
		// generic: we never leak credentials, stack traces, or internal paths.
		step, lookupErr := r.plans.GetStepExecution(ctx, tenantID, stepID)
		if lookupErr != nil {
			return fmt.Errorf("load failed step execution for chat message: %w", lookupErr)
		}
		configID, configErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, step.PlanExecutionID)
		if configErr != nil {
			return fmt.Errorf("resolve plan configuration for failed step: %w", configErr)
		}
		threadID, threadErr := r.owningThreadID(ctx, tenantID, configID)
		if threadErr != nil {
			return threadErr
		}
		execID := step.PlanExecutionID
		_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    threadID,
			ExecutionID: &execID,
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_FAILED,
			Text:        "Step " + step.PlanStepKey + " failed.",
			PayloadJSON: chat.BuildStepFailedPayload(step.PlanStepKey, step.ID.String(), ""),
		})
	}
	if step, lookupErr := r.plans.GetStepExecution(ctx, tenantID, stepID); lookupErr == nil {
		r.notifyExecutionConversation(ctx, tenantID, step.PlanExecutionID, "STEP_FAILED")
	}
	return nil
}

func (r *RuntimeRepository) AwaitElicitationStepExecution(ctx context.Context, input workflow.StepStatusUpdateInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	if err := r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusAwaitingElicitation, "", input.ElicitationThreadID, ""); err != nil {
		return err
	}
	if err := r.persistElicitation(ctx, tenantID, stepID, input); err != nil {
		return err
	}
	if executionID, parseErr := uuid.Parse(strings.TrimSpace(input.PlanExecutionID)); parseErr == nil {
		r.notifyExecutionConversation(ctx, tenantID, executionID, "ELICITATION_RAISED")
	}
	return nil
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
		if lookupErr != nil {
			return fmt.Errorf("resolve plan configuration for execution: %w", lookupErr)
		}
		threadID, err := r.owningThreadID(ctx, tenantID, configID)
		if err != nil {
			return err
		}
		_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    threadID,
			ExecutionID: &execID,
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ELICITATION_RAISED,
			Text:        "Elicitation raised on step " + input.PlanStepKey + ".",
			PayloadJSON: chat.BuildElicitationRaisedPayload(created.ID),
		})
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
	subject := input.SubjectArtifactRef
	subjectID, subjectErr := uuid.Parse(strings.TrimSpace(subject.ArtifactID))
	versionID, versionErr := uuid.Parse(strings.TrimSpace(subject.ArtifactVersionID))
	if subjectErr != nil || versionErr != nil || strings.TrimSpace(subject.ArtifactTypeKey) == "" || strings.TrimSpace(subject.ContentHash) == "" {
		return fmt.Errorf("approval request requires a complete version-pinned subject artifact reference")
	}
	if err := r.plans.CreatePlanApprovalRequest(ctx, &PlanApprovalRequest{
		ID:                input.ApprovalRequestID,
		TenantID:          tenantID,
		PlanExecutionID:   executionID,
		StepExecutionID:   stepID,
		PlanStepKey:       input.PlanStepKey,
		InputArtifactID:   input.InputArtifactID,
		SubjectArtifactID: uuid.NullUUID{UUID: subjectID, Valid: true}, SubjectVersionID: uuid.NullUUID{UUID: versionID, Valid: true}, SubjectTypeKey: subject.ArtifactTypeKey, SubjectContentHash: subject.ContentHash,
		Status: ApprovalRequestStatusPending,
	}); err != nil {
		return err
	}
	if r.chat != nil {
		execID := executionID
		configID, lookupErr := r.plans.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr != nil {
			return fmt.Errorf("resolve plan configuration for execution: %w", lookupErr)
		}
		threadID, err := r.owningThreadID(ctx, tenantID, configID)
		if err != nil {
			return err
		}
		_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    threadID,
			ExecutionID: &execID,
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_APPROVAL_RAISED,
			Text:        "Approval required on step " + input.PlanStepKey + ".",
			PayloadJSON: chat.BuildApprovalRaisedPayload(input.ApprovalRequestID, chat.ApprovalRaisedContext{
				PlanConfigurationID: configID.String(),
				PlanExecutionID:     input.PlanExecutionID,
				StepExecutionID:     input.StepExecutionID,
				PlanStepKey:         input.PlanStepKey,
				InputArtifactID:     input.InputArtifactID,
			}),
		})
	}
	if err := r.plans.UpdateStepExecutionStatus(ctx, tenantID, stepID, StepStatusAwaitingApproval, "", "", input.ApprovalRequestID); err != nil {
		return err
	}
	r.notifyExecutionConversation(ctx, tenantID, executionID, "APPROVAL_RAISED")
	return nil
}

func (r *RuntimeRepository) ResolveApprovalRequest(ctx context.Context, input workflow.ResolveApprovalRequestInput) error {
	tenantID, stepID, err := parseRuntimeTenantStep(input.TenantID, input.StepExecutionID)
	if err != nil {
		return err
	}
	request, err := r.plans.GetPlanApprovalRequest(ctx, tenantID, input.ApprovalRequestID)
	if err != nil {
		return err
	}
	if request.StepExecutionID != stepID {
		return fmt.Errorf("approval request does not belong to step execution")
	}
	if input.Approved {
		if r.artifacts == nil || !request.SubjectArtifactID.Valid || !request.SubjectVersionID.Valid || strings.TrimSpace(request.SubjectContentHash) == "" {
			return fmt.Errorf("approval request has no verifiable pinned subject")
		}
		version, err := r.artifacts.GetArtifactVersion(ctx, tenantID, request.SubjectArtifactID.UUID, request.SubjectVersionID.UUID)
		if err != nil {
			return fmt.Errorf("load pinned approval subject: %w", err)
		}
		if version.ContentHash != request.SubjectContentHash {
			return fmt.Errorf("pinned approval subject content hash mismatch")
		}
	}
	if err := r.plans.ResolvePlanApprovalRequest(ctx, tenantID, input.ApprovalRequestID, stepID, input.Approved, input.Reason); err != nil {
		return err
	}
	r.notifyExecutionConversation(ctx, tenantID, request.PlanExecutionID, "APPROVAL_DECIDED")
	return nil
}

func (r *RuntimeRepository) CompletePlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error {
	now := time.Now().UTC()
	if err := r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusCompleted, &now); err != nil {
		return err
	}
	if err := r.appendRunFinishedChatMessage(ctx, tenantID, executionID); err != nil {
		return err
	}
	r.notifyExecutionConversation(ctx, tenantID, executionID, "RUN_COMPLETED")
	return nil
}

func (r *RuntimeRepository) FailPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID, reason string) error {
	now := time.Now().UTC()
	if err := r.plans.UpdateExecutionStatus(ctx, tenantID, executionID, ExecutionStatusFailed, &now); err != nil {
		return err
	}
	if err := r.appendRunFinishedChatMessage(ctx, tenantID, executionID, reason); err != nil {
		return err
	}
	r.notifyExecutionConversation(ctx, tenantID, executionID, "RUN_FAILED")
	return nil
}

func (r *RuntimeRepository) appendRunFinishedChatMessage(ctx context.Context, tenantID, executionID uuid.UUID, failureReasonOverride ...string) error {
	if r.chat == nil {
		return nil
	}
	execID := executionID
	exec, lookupErr := r.plans.GetExecution(ctx, tenantID, executionID)
	if lookupErr != nil {
		return fmt.Errorf("load plan execution for run-finished message: %w", lookupErr)
	}
	configID := exec.PlanConfigurationID
	var kind chatv1.ThreadMessageKind
	var text string
	var payload string
	if exec.Status == ExecutionStatusFailed {
		kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_FAILED
		failureReason := executionFailureReason(exec)
		if len(failureReasonOverride) > 0 {
			if override := strings.TrimSpace(failureReasonOverride[0]); override != "" {
				failureReason = override
			}
		}
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
	threadID, err := r.owningThreadID(ctx, tenantID, configID)
	if err != nil {
		return err
	}
	_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    threadID,
		ExecutionID: &execID,
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
		Kind:        kind,
		Text:        text,
		PayloadJSON: payload,
	})
	return nil
}

func (r *RuntimeRepository) owningThreadID(ctx context.Context, tenantID, configID uuid.UUID) (string, error) {
	config, err := r.plans.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return "", fmt.Errorf("load plan configuration for owning thread: %w", err)
	}
	threadID := configurationThreadID(config)
	if threadID == uuid.Nil {
		return "", fmt.Errorf("plan configuration %s has no owning origin thread", configID)
	}
	return threadID.String(), nil
}

// stepStartedChatMessageExists reports whether STEP_STARTED was already appended
// for the given step execution (Temporal activity retries must not duplicate).
func (r *RuntimeRepository) stepStartedChatMessageExists(
	ctx context.Context,
	tenantID uuid.UUID,
	threadID, stepExecutionID string,
) bool {
	if r == nil || r.chat == nil {
		return false
	}
	msgs, err := r.chat.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		return false
	}
	for _, msg := range msgs {
		if msg.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_STARTED {
			continue
		}
		var payload map[string]string
		if err := json.Unmarshal([]byte(msg.GetPayloadJson()), &payload); err != nil {
			continue
		}
		if payload["step_execution_id"] == stepExecutionID {
			return true
		}
	}
	return false
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
		if attempt.Status == StepStatusSkipped {
			// A skipped upstream step has no artifact, but its branch is also
			// skipped on this run, so no running step consumes it.
			continue
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
