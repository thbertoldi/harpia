package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/planrules"
)

const PlanScheduledExecutionWorkflowName = "PlanScheduledExecution"
const PlanWorkflowName = "PlanWorkflow"

const (
	LoadPlanExecutionActivityName               = "LoadPlanExecutionActivity"
	StartPlanExecutionActivityName              = "StartPlanExecutionActivity"
	CreateStepExecutionActivityName             = "CreateStepExecutionActivity"
	CreateSkippedStepExecutionActivityName      = "CreateSkippedStepExecutionActivity"
	RunIntegrationActivityName                  = "RunIntegrationActivity"
	RunAgentActivityName                        = "RunAgentActivity"
	CompleteStepExecutionActivityName           = "CompleteStepExecutionActivity"
	FailStepExecutionActivityName               = "FailStepExecutionActivity"
	AwaitElicitationStepExecutionActivityName   = "AwaitElicitationStepExecutionActivity"
	TimeoutElicitationStepExecutionActivityName = "TimeoutElicitationStepExecutionActivity"
	ResumeStepExecutionActivityName             = "ResumeStepExecutionActivity"
	CreateApprovalRequestActivityName           = "CreateApprovalRequestActivity"
	ResolveApprovalRequestActivityName          = "ResolveApprovalRequestActivity"
	CreateReviewRequestActivityName             = "CreateReviewRequestActivity"
	ResolveReviewRequestActivityName            = "ResolveReviewRequestActivity"
	CompletePlanExecutionActivityName           = "CompletePlanExecutionActivity"
	FailPlanExecutionActivityName               = "FailPlanExecutionActivity"
	RecordAuditActivityName                     = "RecordAuditActivity"

	PlanElicitationResponseSignalName = "plan-elicitation-response"
	PlanApprovalDecisionSignalName    = "plan-approval-decision"
	PlanReviewDecisionSignalName      = "plan-review-decision"

	ExecutorResultStatusCompleted            = "completed"
	ExecutorResultStatusFailed               = "failed"
	ExecutorResultStatusElicitationRequested = "elicitation_requested"

	ExecutorKindIntegration = "integration"
	ExecutorKindAgent       = "agent"

	PlanExecutionStatusPending = "pending"
)

const (
	maxElicitationRoundsPerStep    int   = 10
	defaultElicitationTimeoutHours       = 24
	maxElicitationTimeoutHours     int32 = 720
	planActivityMaximumAttempts    int32 = 3
	// planActivityScheduleToStartTimeout bounds how long an activity can sit in
	// a task queue before a worker picks it up. Agent activities are routed to
	// a dedicated queue (AgentTaskQueueName) serviced only by the Python
	// agent-runtime worker; without this timeout an outage there would park the
	// plan at ExecuteActivity().Get() forever — invisible, no retry, no failure.
	// 2 min turns that silent stall into a clean, retryable failure.
	planActivityScheduleToStartTimeout = 2 * time.Minute
)

type PlanExecutionInput struct {
	TenantID            string `json:"tenant_id"`
	PlanConfigurationID string `json:"plan_configuration_id"`
	PlanExecutionID     string `json:"plan_execution_id,omitempty"`
}

type PlanWorkflowInput struct {
	TenantID                string                 `json:"tenant_id"`
	PlanExecutionID         string                 `json:"plan_execution_id"`
	RetryFromStepKey        string                 `json:"retry_from_step_key,omitempty"`
	RetryStepArtifactsByKey map[string]ArtifactRef `json:"retry_step_artifacts_by_key,omitempty"`
}

type PlanFailureInput struct {
	TenantID        string `json:"tenant_id"`
	PlanExecutionID string `json:"plan_execution_id"`
	Error           string `json:"error,omitempty"`
}

type PlanWorkflowResult struct {
	PlanExecutionID string `json:"plan_execution_id"`
	Status          string `json:"status"`
}

func planActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout:    5 * time.Minute,
		ScheduleToStartTimeout: planActivityScheduleToStartTimeout,
		HeartbeatTimeout:       30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    planActivityMaximumAttempts,
		},
	}
}

type PlanExecutionSnapshot struct {
	SchemaVersion         int                                     `json:"schema_version"`
	Configuration         *plansv1.PlanConfiguration              `json:"configuration"`
	Template              *plansv1.PlanTemplate                   `json:"template"`
	ActiveStepKeys        []string                                `json:"active_step_keys"`
	ExecutorInstallations map[string]ExecutorInstallationSnapshot `json:"executor_installations"`
	SnapshotAt            string                                  `json:"snapshot_at,omitempty"`
}

type ExecutorInstallationSnapshot struct {
	ID               string `json:"id"`
	TenantID         string `json:"tenant_id"`
	ExecutorSKUID    string `json:"executor_sku_id"`
	ExecutorSKUKey   string `json:"executor_sku_key,omitempty"`
	Kind             string `json:"kind"`
	DisplayName      string `json:"display_name"`
	Enabled          bool   `json:"enabled"`
	ConnectionStatus string `json:"connection_status,omitempty"`
	ConfigJSON       string `json:"config_json,omitempty"`
	ManifestID       string `json:"manifest_id,omitempty"`
	ManifestVersion  string `json:"manifest_version,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type LoadedPlanExecution struct {
	PlanExecutionID string                `json:"plan_execution_id"`
	Status          string                `json:"status"`
	Snapshot        PlanExecutionSnapshot `json:"snapshot"`
}

type CreateStepExecutionInput struct {
	TenantID                     string                       `json:"tenant_id"`
	PlanExecutionID              string                       `json:"plan_execution_id"`
	PlanStepKey                  string                       `json:"plan_step_key"`
	InputArtifactID              string                       `json:"input_artifact_id,omitempty"`
	InputArtifactRef             ArtifactRef                  `json:"input_artifact_ref,omitempty"`
	ExecutorInstallationSnapshot ExecutorInstallationSnapshot `json:"executor_installation_snapshot"`
}

// CreateSkippedStepInput captures the identity of a step the run deliberately
// did not execute (a non-selected output-format branch). Skipped steps record a
// SKIPPED row with no input artifact and emit no chat message.
type CreateSkippedStepInput struct {
	TenantID        string      `json:"tenant_id"`
	PlanExecutionID string      `json:"plan_execution_id"`
	PlanStepKey     string      `json:"plan_step_key"`
	ArtifactRef     ArtifactRef `json:"artifact_ref"`
}

type StepExecutionRecord struct {
	ID              string `json:"id"`
	PlanStepKey     string `json:"plan_step_key"`
	Attempt         int32  `json:"attempt"`
	InputArtifactID string `json:"input_artifact_id,omitempty"`
}

type StepStatusUpdateInput struct {
	TenantID            string      `json:"tenant_id"`
	StepExecutionID     string      `json:"step_execution_id"`
	OutputArtifactID    string      `json:"output_artifact_id,omitempty"`
	OutputArtifactRef   ArtifactRef `json:"output_artifact_ref,omitempty"`
	ElicitationThreadID string      `json:"elicitation_thread_id,omitempty"`
	// Elicitation persistence fields (E5.1), populated when a step pauses for an
	// elicitation so the runtime can durably record the in-app thread.
	PlanExecutionID       string `json:"plan_execution_id,omitempty"`
	PlanStepKey           string `json:"plan_step_key,omitempty"`
	ElicitationPrompt     string `json:"elicitation_prompt,omitempty"`
	ElicitationSchemaJSON string `json:"elicitation_schema_json,omitempty"`
	ElicitationExpiresAt  string `json:"elicitation_expires_at,omitempty"`
	ElicitationTimeout    string `json:"elicitation_timeout_behavior,omitempty"`
}

type ArtifactRef struct {
	Source            string `json:"source"`
	StepKey           string `json:"step_key,omitempty"`
	InputName         string `json:"input_name,omitempty"`
	ArtifactID        string `json:"artifact_id,omitempty"`
	ArtifactVersionID string `json:"artifact_version_id,omitempty"`
	ContentHash       string `json:"content_hash,omitempty"`
	LiteralJSON       string `json:"literal_json,omitempty"`
	ArtifactTypeKey   string `json:"artifact_type_key,omitempty"`
}

type ExecutorActivityInput struct {
	TenantID                     string                       `json:"tenant_id"`
	PlanExecutionID              string                       `json:"plan_execution_id"`
	StepExecutionID              string                       `json:"step_execution_id"`
	PlanStepKey                  string                       `json:"plan_step_key"`
	InputArtifacts               []ArtifactRef                `json:"input_artifacts"`
	OutputArtifactTypeKey        string                       `json:"output_artifact_type_key"`
	ExecutorInstallationSnapshot ExecutorInstallationSnapshot `json:"executor_installation_snapshot"`
	ElicitationResponse          *ElicitationResponseSignal   `json:"elicitation_response,omitempty"`
}

type ExecutorActivityResult struct {
	Status                  string `json:"status"`
	OutputArtifactID        string `json:"output_artifact_id,omitempty"`
	OutputArtifactVersionID string `json:"output_artifact_version_id,omitempty"`
	OutputArtifactTypeKey   string `json:"output_artifact_type_key,omitempty"`
	OutputContentHash       string `json:"output_content_hash,omitempty"`
	ElicitationThreadID     string `json:"elicitation_thread_id,omitempty"`
	ElicitationPrompt       string `json:"elicitation_prompt,omitempty"`
	ElicitationSchemaJSON   string `json:"elicitation_schema_json,omitempty"`
	Error                   string `json:"error,omitempty"`
}

type ElicitationResponseSignal struct {
	StepExecutionID     string `json:"step_execution_id"`
	ElicitationThreadID string `json:"elicitation_thread_id,omitempty"`
	ResponseArtifactID  string `json:"response_artifact_id,omitempty"`
	ResponseText        string `json:"response_text,omitempty"`
}

type ApprovalDecisionSignal struct {
	StepExecutionID   string `json:"step_execution_id"`
	ApprovalRequestID string `json:"approval_request_id"`
	Approved          bool   `json:"approved"`
	Reason            string `json:"reason,omitempty"`
}

type ReviewDecisionSignal struct {
	StepExecutionID string `json:"step_execution_id"`
	ReviewRequestID string `json:"review_request_id"`
	Decision        string `json:"decision"`
	Feedback        string `json:"feedback,omitempty"`
}

type CreateReviewRequestInput struct {
	TenantID           string      `json:"tenant_id"`
	PlanExecutionID    string      `json:"plan_execution_id"`
	StepExecutionID    string      `json:"step_execution_id"`
	PlanStepKey        string      `json:"plan_step_key"`
	SubjectArtifactRef ArtifactRef `json:"subject_artifact_ref"`
}

type ResolveReviewRequestInput struct {
	TenantID        string `json:"tenant_id"`
	StepExecutionID string `json:"step_execution_id"`
	ReviewRequestID string `json:"review_request_id"`
	Decision        string `json:"decision"`
	Feedback        string `json:"feedback,omitempty"`
}

type CreateApprovalRequestInput struct {
	TenantID           string      `json:"tenant_id"`
	PlanExecutionID    string      `json:"plan_execution_id"`
	StepExecutionID    string      `json:"step_execution_id"`
	PlanStepKey        string      `json:"plan_step_key"`
	ApprovalRequestID  string      `json:"approval_request_id"`
	InputArtifactID    string      `json:"input_artifact_id,omitempty"`
	SubjectArtifactRef ArtifactRef `json:"subject_artifact_ref"`
}

type ResolveApprovalRequestInput struct {
	TenantID          string `json:"tenant_id"`
	StepExecutionID   string `json:"step_execution_id"`
	ApprovalRequestID string `json:"approval_request_id"`
	Approved          bool   `json:"approved"`
	Reason            string `json:"reason,omitempty"`
}

// AuditDiff and AuditEvent are the workflow-owned, transport-free contract for
// the audit activity. The API process adapts it to audit.Recorder at wiring
// time, keeping Temporal workflow code free of database and Connect concerns.
type AuditDiff struct {
	Field  string `json:"field"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type AuditEvent struct {
	TenantID       string      `json:"tenant_id"`
	EventType      string      `json:"event_type"`
	BoundedContext string      `json:"bounded_context"`
	SubjectType    string      `json:"subject_type"`
	SubjectID      string      `json:"subject_id"`
	Decision       string      `json:"decision,omitempty"`
	DedupeKey      string      `json:"dedupe_key"`
	Diff           []AuditDiff `json:"diff,omitempty"`
}

// AuditRecorder is implemented by API wiring. Its narrow semantic contract
// keeps the workflow package independent of the audit adapter's pgx/Connect
// dependencies.
type AuditRecorder interface {
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type PlanRuntimeStore interface {
	CreateScheduledExecution(ctx context.Context, tenantID, configID, executionID uuid.UUID) (PlanWorkflowInput, error)
	LoadPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) (LoadedPlanExecution, error)
	StartPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error
	CreateStepExecution(ctx context.Context, input CreateStepExecutionInput) (StepExecutionRecord, error)
	CreateSkippedStepExecution(ctx context.Context, input CreateSkippedStepInput) (StepExecutionRecord, error)
	ResumeStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	CompleteStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	FailStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	AwaitElicitationStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	TimeoutElicitationStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	CreateApprovalRequest(ctx context.Context, input CreateApprovalRequestInput) error
	ResolveApprovalRequest(ctx context.Context, input ResolveApprovalRequestInput) error
	CreateReviewRequest(ctx context.Context, input CreateReviewRequestInput) (string, error)
	ResolveReviewRequest(ctx context.Context, input ResolveReviewRequestInput) error
	CompletePlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error
	FailPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID, reason string) error
}

type PlanActivities struct {
	Runtime      PlanRuntimeStore
	Integrations executors.IntegrationRunner
	Audit        AuditRecorder
}

func (a *PlanActivities) CreateScheduledPlanExecutionActivity(ctx context.Context, input PlanExecutionInput) (PlanWorkflowInput, error) {
	if a == nil || a.Runtime == nil {
		return PlanWorkflowInput{}, fmt.Errorf("plan runtime store is not configured")
	}

	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return PlanWorkflowInput{}, fmt.Errorf("parse tenant id: %w", err)
	}
	configID, err := uuid.Parse(input.PlanConfigurationID)
	if err != nil {
		return PlanWorkflowInput{}, fmt.Errorf("parse plan configuration id: %w", err)
	}
	executionID, err := parseOptionalUUID(input.PlanExecutionID)
	if err != nil {
		return PlanWorkflowInput{}, fmt.Errorf("parse plan execution id: %w", err)
	}

	return a.Runtime.CreateScheduledExecution(ctx, tenantID, configID, executionID)
}

func (a *PlanActivities) LoadPlanExecutionActivity(ctx context.Context, input PlanWorkflowInput) (LoadedPlanExecution, error) {
	if a == nil || a.Runtime == nil {
		return LoadedPlanExecution{}, fmt.Errorf("plan runtime store is not configured")
	}
	tenantID, executionID, err := parsePlanWorkflowIDs(input)
	if err != nil {
		return LoadedPlanExecution{}, err
	}
	return a.Runtime.LoadPlanExecution(ctx, tenantID, executionID)
}

func (a *PlanActivities) StartPlanExecutionActivity(ctx context.Context, input PlanWorkflowInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	tenantID, executionID, err := parsePlanWorkflowIDs(input)
	if err != nil {
		return err
	}
	if err := a.Runtime.StartPlanExecution(ctx, tenantID, executionID); err != nil {
		return err
	}
	a.recordAudit(ctx, planTransitionAudit(input, "plan_execution.started", "pending", "running"))
	return nil
}

func (a *PlanActivities) CreateStepExecutionActivity(ctx context.Context, input CreateStepExecutionInput) (StepExecutionRecord, error) {
	if a == nil || a.Runtime == nil {
		return StepExecutionRecord{}, fmt.Errorf("plan runtime store is not configured")
	}
	record, err := a.Runtime.CreateStepExecution(ctx, input)
	if err != nil {
		return StepExecutionRecord{}, err
	}
	a.recordAudit(ctx, stepTransitionAudit(input.TenantID, record.ID, "step_execution.started", "", "running"))
	return record, nil
}

func (a *PlanActivities) CreateSkippedStepExecutionActivity(ctx context.Context, input CreateSkippedStepInput) (StepExecutionRecord, error) {
	if a == nil || a.Runtime == nil {
		return StepExecutionRecord{}, fmt.Errorf("plan runtime store is not configured")
	}
	record, err := a.Runtime.CreateSkippedStepExecution(ctx, input)
	if err != nil {
		return StepExecutionRecord{}, err
	}
	a.recordAudit(ctx, stepTransitionAudit(input.TenantID, record.ID, "step_execution.skipped", "", "skipped"))
	return record, nil
}

func (a *PlanActivities) ResumeStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.ResumeStepExecution(ctx, input)
}

func (a *PlanActivities) CompleteStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	if err := a.Runtime.CompleteStepExecution(ctx, input); err != nil {
		return err
	}
	a.recordAudit(ctx, stepTransitionAudit(input.TenantID, input.StepExecutionID, "step_execution.completed", "running", "completed"))
	return nil
}

func (a *PlanActivities) FailStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	if err := a.Runtime.FailStepExecution(ctx, input); err != nil {
		return err
	}
	a.recordAudit(ctx, stepTransitionAudit(input.TenantID, input.StepExecutionID, "step_execution.failed", "running", "failed"))
	return nil
}

func (a *PlanActivities) AwaitElicitationStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.AwaitElicitationStepExecution(ctx, input)
}

func (a *PlanActivities) TimeoutElicitationStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.TimeoutElicitationStepExecution(ctx, input)
}

func (a *PlanActivities) CreateApprovalRequestActivity(ctx context.Context, input CreateApprovalRequestInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	if err := a.Runtime.CreateApprovalRequest(ctx, input); err != nil {
		return err
	}
	a.recordAudit(ctx, approvalCreatedAudit(input))
	return nil
}

func (a *PlanActivities) ResolveApprovalRequestActivity(ctx context.Context, input ResolveApprovalRequestInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	if err := a.Runtime.ResolveApprovalRequest(ctx, input); err != nil {
		return err
	}
	a.recordAudit(ctx, approvalDecisionAudit(input))
	return nil
}

func (a *PlanActivities) CreateReviewRequestActivity(ctx context.Context, input CreateReviewRequestInput) (string, error) {
	if a == nil || a.Runtime == nil {
		return "", fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.CreateReviewRequest(ctx, input)
}

func (a *PlanActivities) ResolveReviewRequestActivity(ctx context.Context, input ResolveReviewRequestInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.ResolveReviewRequest(ctx, input)
}

func (a *PlanActivities) CompletePlanExecutionActivity(ctx context.Context, input PlanWorkflowInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	tenantID, executionID, err := parsePlanWorkflowIDs(input)
	if err != nil {
		return err
	}
	if err := a.Runtime.CompletePlanExecution(ctx, tenantID, executionID); err != nil {
		return err
	}
	a.recordAudit(ctx, planTransitionAudit(input, "plan_execution.completed", "running", "completed"))
	return nil
}

func (a *PlanActivities) FailPlanExecutionActivity(ctx context.Context, input PlanFailureInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	tenantID, executionID, err := parsePlanFailureIDs(input)
	if err != nil {
		return err
	}
	if err := a.Runtime.FailPlanExecution(ctx, tenantID, executionID, strings.TrimSpace(input.Error)); err != nil {
		return err
	}
	a.recordAudit(ctx, AuditEvent{
		TenantID: input.TenantID, EventType: "plan_execution.failed", BoundedContext: "workflow_engine",
		SubjectType: "plan_execution", SubjectID: input.PlanExecutionID,
		DedupeKey: input.PlanExecutionID + ":plan_execution.failed",
		Diff:      []AuditDiff{{Field: "status", Before: "running", After: "failed"}},
	})
	return nil
}

// RecordAuditActivity is scheduled by workflow code for signal receipt. Other
// lifecycle activities call the same recorder after their successful state
// transition. This is intentionally best effort: audit queue pressure must not
// fail a governed operation after its state was committed.
func (a *PlanActivities) RecordAuditActivity(ctx context.Context, event AuditEvent) error {
	a.recordAudit(ctx, event)
	return nil
}

func (a *PlanActivities) recordAudit(ctx context.Context, event AuditEvent) {
	if a == nil || a.Audit == nil {
		return
	}
	_ = a.Audit.RecordAudit(ctx, event)
}

// Workflow audit dedupe keys are deterministic because Temporal activities are
// at-least-once: execution transitions use execution_id + transition;
// StepExecution transitions use the persisted step_execution_id + transition;
// approval creation/decisions use request_id (+ decision); signal receipts use
// their stable execution/request/step identifiers. The audit table suppresses
// duplicate keys within a tenant while preserving distinct retries/attempts.
func planTransitionAudit(input PlanWorkflowInput, eventType, before, after string) AuditEvent {
	return AuditEvent{TenantID: input.TenantID, EventType: eventType, BoundedContext: "workflow_engine", SubjectType: "plan_execution", SubjectID: input.PlanExecutionID, DedupeKey: input.PlanExecutionID + ":" + eventType, Diff: []AuditDiff{{Field: "status", Before: before, After: after}}}
}

func stepTransitionAudit(tenantID, stepID, eventType, before, after string) AuditEvent {
	return AuditEvent{TenantID: tenantID, EventType: eventType, BoundedContext: "workflow_engine", SubjectType: "step_execution", SubjectID: stepID, DedupeKey: stepID + ":" + eventType, Diff: []AuditDiff{{Field: "status", Before: before, After: after}}}
}

func approvalCreatedAudit(input CreateApprovalRequestInput) AuditEvent {
	return AuditEvent{TenantID: input.TenantID, EventType: "approval.created", BoundedContext: "human_interaction", SubjectType: "approval_request", SubjectID: input.ApprovalRequestID, DedupeKey: input.PlanExecutionID + ":approval:" + input.ApprovalRequestID + ":created", Diff: []AuditDiff{{Field: "status", After: "pending"}}}
}

func approvalDecisionAudit(input ResolveApprovalRequestInput) AuditEvent {
	decision := "reject"
	status := "rejected"
	if input.Approved {
		decision, status = "approve", "approved"
	}
	return AuditEvent{TenantID: input.TenantID, EventType: "approval.decided", BoundedContext: "human_interaction", SubjectType: "approval_request", SubjectID: input.ApprovalRequestID, Decision: decision, DedupeKey: input.ApprovalRequestID + ":approval.decided:" + decision, Diff: []AuditDiff{{Field: "status", Before: "pending", After: status}, {Field: "decision", After: decision}}}
}

func (a *PlanActivities) RunAgentActivity(ctx context.Context, input ExecutorActivityInput) (ExecutorActivityResult, error) {
	return ExecutorActivityResult{
		Status: ExecutorResultStatusFailed,
		Error:  fmt.Sprintf("RunAgent is not implemented for step %q", input.PlanStepKey),
	}, nil
}

func parsePlanWorkflowIDs(input PlanWorkflowInput) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse tenant id: %w", err)
	}
	executionID, err := uuid.Parse(input.PlanExecutionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse plan execution id: %w", err)
	}
	return tenantID, executionID, nil
}

func parsePlanFailureIDs(input PlanFailureInput) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse tenant id: %w", err)
	}
	executionID, err := uuid.Parse(input.PlanExecutionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse plan execution id: %w", err)
	}
	return tenantID, executionID, nil
}

func parseOptionalUUID(raw string) (uuid.UUID, error) {
	if strings.TrimSpace(raw) == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(raw)
}

func PlanScheduledExecution(ctx workflow.Context, input PlanExecutionInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting scheduled plan execution",
		"tenant_id", input.TenantID,
		"plan_configuration_id", input.PlanConfigurationID,
	)
	if input.PlanExecutionID == "" {
		workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID
		input.PlanExecutionID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(workflowID)).String()
	}

	ctx = workflow.WithActivityOptions(ctx, planActivityOptions())

	var planInput PlanWorkflowInput
	if err := workflow.ExecuteActivity(ctx, CreateScheduledPlanExecutionActivityName, input).Get(ctx, &planInput); err != nil {
		return err
	}
	if planInput.PlanExecutionID == "" {
		logger.Info("scheduled plan did not create an execution",
			"tenant_id", input.TenantID,
			"plan_configuration_id", input.PlanConfigurationID,
		)
		return nil
	}

	_, err := runPlanWorkflow(ctx, planInput)
	return err
}

func PlanWorkflow(ctx workflow.Context, input PlanWorkflowInput) (PlanWorkflowResult, error) {
	return runPlanWorkflow(ctx, input)
}

func runPlanWorkflow(ctx workflow.Context, input PlanWorkflowInput) (PlanWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("starting plan workflow",
		"tenant_id", input.TenantID,
		"plan_execution_id", input.PlanExecutionID,
	)

	ctx = workflow.WithActivityOptions(ctx, planActivityOptions())

	result := PlanWorkflowResult{PlanExecutionID: input.PlanExecutionID, Status: "failed"}

	var loaded LoadedPlanExecution
	if err := workflow.ExecuteActivity(ctx, LoadPlanExecutionActivityName, input).Get(ctx, &loaded); err != nil {
		return result, err
	}
	if err := validatePlanSnapshot(loaded.Snapshot); err != nil {
		_ = workflow.ExecuteActivity(ctx, FailPlanExecutionActivityName, planFailureInput(input, err)).Get(ctx, nil)
		return result, err
	}
	if loaded.Status != PlanExecutionStatusPending {
		return result, failPlan(ctx, input, fmt.Errorf("plan execution status %q is not pending", loaded.Status))
	}
	if err := workflow.ExecuteActivity(ctx, StartPlanExecutionActivityName, input).Get(ctx, nil); err != nil {
		return result, err
	}

	order, err := topologicalPlanSteps(loaded.Snapshot.Template)
	if err != nil {
		_ = workflow.ExecuteActivity(ctx, FailPlanExecutionActivityName, planFailureInput(input, err)).Get(ctx, nil)
		return result, err
	}
	dependencies := dependencyIndex(loaded.Snapshot.Template)
	seedArtifacts := seedArtifactsByStep(loaded.Snapshot.Configuration)
	outputs := make(map[string]ArtifactRef, len(order))
	for stepKey, output := range input.RetryStepArtifactsByKey {
		if strings.TrimSpace(output.ArtifactID) == "" {
			continue
		}
		outputs[stepKey] = output
	}
	retryIndex := indexForRetryStep(order, input.RetryFromStepKey)
	if strings.TrimSpace(input.RetryFromStepKey) != "" && retryIndex < 0 {
		return result, failPlan(ctx, input, fmt.Errorf("retry step %q is not part of the plan template", input.RetryFromStepKey))
	}

	policies := loaded.Snapshot.Configuration.GetBehaviorPolicies()

	for i, step := range order {
		if shouldSkipStepForFormat(step, policies) {
			if err := workflow.ExecuteActivity(ctx, CreateSkippedStepExecutionActivityName, CreateSkippedStepInput{
				TenantID:        input.TenantID,
				PlanExecutionID: input.PlanExecutionID,
				PlanStepKey:     step.Key,
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
			continue
		}
		if shouldSkipStepForOptOut(step, loaded.Snapshot.Configuration) {
			alias := ArtifactRef{}
			if step.GetInputArtifactTypeId() == step.GetOutputArtifactTypeId() {
				var aliasErr error
				alias, aliasErr = aliasUpstreamArtifact(step, dependencies[step.Key], outputs)
				if aliasErr != nil {
					return result, failPlan(ctx, input, aliasErr)
				}
			}
			if err := workflow.ExecuteActivity(ctx, CreateSkippedStepExecutionActivityName, CreateSkippedStepInput{
				TenantID:        input.TenantID,
				PlanExecutionID: input.PlanExecutionID,
				PlanStepKey:     step.Key,
				ArtifactRef:     alias,
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
			if alias.ArtifactID != "" {
				outputs[step.Key] = alias
			}
			continue
		}
		if retryIndex >= 0 && i < retryIndex {
			reused, ok := outputs[step.Key]
			if !ok || strings.TrimSpace(reused.ArtifactID) == "" {
				return result, failPlan(ctx, input, fmt.Errorf("retry step %q missing reusable artifact for upstream step %q", input.RetryFromStepKey, step.Key))
			}
			continue
		}
		inputArtifacts, err := stepInputArtifacts(step, dependencies[step.Key], seedArtifacts, outputs)
		if err != nil {
			installation := loaded.Snapshot.ExecutorInstallations[step.Key]
			return result, failStepAndPlan(ctx, input, step, inputArtifacts, installation, err)
		}
		installation, ok := loaded.Snapshot.ExecutorInstallations[step.Key]
		if !ok {
			return result, failStepAndPlan(ctx, input, step, inputArtifacts, ExecutorInstallationSnapshot{}, fmt.Errorf("missing executor installation snapshot for step %q", step.Key))
		}

		stepRecord, err := createRunningStep(ctx, input, step, inputArtifacts, installation)
		if err != nil {
			return result, failPlan(ctx, input, err)
		}

		if requiresPublishApproval(step, policies) {
			approvalRequestID := planApprovalRequestID(input.PlanExecutionID, stepRecord.ID)
			if err := workflow.ExecuteActivity(ctx, CreateApprovalRequestActivityName, CreateApprovalRequestInput{
				TenantID:           input.TenantID,
				PlanExecutionID:    input.PlanExecutionID,
				ApprovalRequestID:  approvalRequestID,
				StepExecutionID:    stepRecord.ID,
				PlanStepKey:        step.Key,
				InputArtifactID:    stepRecord.InputArtifactID,
				SubjectArtifactRef: firstArtifactRef(inputArtifacts),
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
			decision, err := waitForApprovalDecision(ctx, input, stepRecord.ID, approvalRequestID)
			if err != nil {
				_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:        input.TenantID,
					StepExecutionID: stepRecord.ID,
				}).Get(ctx, nil)
				return result, failPlan(ctx, input, fmt.Errorf("step %q approval gate: %w", step.Key, err))
			}
			if err := workflow.ExecuteActivity(ctx, ResolveApprovalRequestActivityName, ResolveApprovalRequestInput{
				TenantID:          input.TenantID,
				StepExecutionID:   stepRecord.ID,
				ApprovalRequestID: approvalRequestID,
				Approved:          decision.Approved,
				Reason:            decision.Reason,
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
			if !decision.Approved {
				_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:        input.TenantID,
					StepExecutionID: stepRecord.ID,
				}).Get(ctx, nil)
				return result, failPlan(ctx, input, fmt.Errorf("step %q approval rejected: %s", step.Key, strings.TrimSpace(decision.Reason)))
			}
			if err := workflow.ExecuteActivity(ctx, ResumeStepExecutionActivityName, StepStatusUpdateInput{
				TenantID:        input.TenantID,
				StepExecutionID: stepRecord.ID,
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
		}

		executorInput := ExecutorActivityInput{
			TenantID:                     input.TenantID,
			PlanExecutionID:              input.PlanExecutionID,
			StepExecutionID:              stepRecord.ID,
			PlanStepKey:                  step.Key,
			InputArtifacts:               inputArtifacts,
			OutputArtifactTypeKey:        step.OutputArtifactTypeId,
			ExecutorInstallationSnapshot: installation,
		}

		elicitationRounds := 0
		for {
			executorResult, err := runExecutorActivity(ctx, installation.Kind, executorInput)
			if err != nil {
				_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:        input.TenantID,
					StepExecutionID: stepRecord.ID,
				}).Get(ctx, nil)
				return result, failPlan(ctx, input, err)
			}

			switch executorResult.Status {
			case ExecutorResultStatusCompleted:
				if strings.TrimSpace(executorResult.OutputArtifactID) == "" {
					_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
						TenantID:        input.TenantID,
						StepExecutionID: stepRecord.ID,
					}).Get(ctx, nil)
					return result, failPlan(ctx, input, fmt.Errorf("step %q completed without output artifact", step.Key))
				}
				outputRef := ArtifactRef{Source: "step_output", StepKey: step.Key, ArtifactID: executorResult.OutputArtifactID, ArtifactVersionID: executorResult.OutputArtifactVersionID, ArtifactTypeKey: executorResult.OutputArtifactTypeKey, ContentHash: executorResult.OutputContentHash}
				if outputRef.ArtifactTypeKey == "" {
					outputRef.ArtifactTypeKey = step.OutputArtifactTypeId
				}
				if requiresReview(step) {
					reviewID, reviewErr := createAndWaitForReview(ctx, input, stepRecord, step, outputRef)
					if reviewErr != nil {
						return result, failPlan(ctx, input, reviewErr)
					}
					decision, decisionErr := waitForReviewDecision(ctx, input, stepRecord.ID, reviewID)
					if decisionErr != nil {
						return result, failPlan(ctx, input, decisionErr)
					}
					if err := workflow.ExecuteActivity(ctx, ResolveReviewRequestActivityName, ResolveReviewRequestInput{TenantID: input.TenantID, StepExecutionID: stepRecord.ID, ReviewRequestID: reviewID, Decision: decision.Decision, Feedback: decision.Feedback}).Get(ctx, nil); err != nil {
						return result, failPlan(ctx, input, err)
					}
					if decision.Decision == "revise" {
						executorInput.ElicitationResponse = &ElicitationResponseSignal{StepExecutionID: stepRecord.ID, ResponseText: `{"review_feedback":` + fmt.Sprintf("%q", decision.Feedback) + `}`}
						continue
					}
				}
				if err := workflow.ExecuteActivity(ctx, CompleteStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:          input.TenantID,
					StepExecutionID:   stepRecord.ID,
					PlanExecutionID:   input.PlanExecutionID,
					PlanStepKey:       step.Key,
					OutputArtifactID:  executorResult.OutputArtifactID,
					OutputArtifactRef: outputRef,
				}).Get(ctx, nil); err != nil {
					return result, failPlan(ctx, input, err)
				}
				outputs[step.Key] = outputRef

			case ExecutorResultStatusElicitationRequested:
				elicitationRounds++
				if elicitationRounds > maxElicitationRoundsPerStep {
					_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
						TenantID:        input.TenantID,
						StepExecutionID: stepRecord.ID,
					}).Get(ctx, nil)
					return result, failPlan(ctx, input, fmt.Errorf("step %q exceeded %d elicitation rounds", step.Key, maxElicitationRoundsPerStep))
				}
				threadID := strings.TrimSpace(executorResult.ElicitationThreadID)
				if threadID == "" {
					_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
						TenantID:        input.TenantID,
						StepExecutionID: stepRecord.ID,
					}).Get(ctx, nil)
					return result, failPlan(ctx, input, fmt.Errorf("step %q requested elicitation without thread id", step.Key))
				}
				awaitInput := StepStatusUpdateInput{
					TenantID:              input.TenantID,
					StepExecutionID:       stepRecord.ID,
					ElicitationThreadID:   threadID,
					PlanExecutionID:       input.PlanExecutionID,
					PlanStepKey:           step.Key,
					ElicitationPrompt:     executorResult.ElicitationPrompt,
					ElicitationSchemaJSON: executorResult.ElicitationSchemaJSON,
					ElicitationExpiresAt:  elicitationExpiresAt(ctx, policies),
					ElicitationTimeout:    elicitationTimeoutBehavior(policies).String(),
				}
				if err := workflow.ExecuteActivity(ctx, AwaitElicitationStepExecutionActivityName, awaitInput).Get(ctx, nil); err != nil {
					return result, failPlan(ctx, input, err)
				}
				response, timedOut, err := waitForElicitationResponse(ctx, input, stepRecord.ID, threadID, policies)
				if err != nil {
					_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
						TenantID:        input.TenantID,
						StepExecutionID: stepRecord.ID,
					}).Get(ctx, nil)
					return result, failPlan(ctx, input, fmt.Errorf("step %q elicitation response: %w", step.Key, err))
				}
				if timedOut {
					_ = workflow.ExecuteActivity(ctx, TimeoutElicitationStepExecutionActivityName, StepStatusUpdateInput{
						TenantID:            input.TenantID,
						StepExecutionID:     stepRecord.ID,
						ElicitationThreadID: threadID,
					}).Get(ctx, nil)
					return result, failTimedOutElicitation(ctx, input, step, stepRecord, policies)
				}
				if err := workflow.ExecuteActivity(ctx, ResumeStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:        input.TenantID,
					StepExecutionID: stepRecord.ID,
				}).Get(ctx, nil); err != nil {
					return result, failPlan(ctx, input, err)
				}
				executorInput.ElicitationResponse = &response
				continue

			case ExecutorResultStatusFailed:
				_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:        input.TenantID,
					StepExecutionID: stepRecord.ID,
				}).Get(ctx, nil)
				if executorResult.Error == "" {
					executorResult.Error = "executor failed"
				}
				return result, failPlan(ctx, input, fmt.Errorf("step %q: %s", step.Key, executorResult.Error))

			default:
				_ = workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
					TenantID:        input.TenantID,
					StepExecutionID: stepRecord.ID,
				}).Get(ctx, nil)
				return result, failPlan(ctx, input, fmt.Errorf("step %q returned unsupported executor status %q", step.Key, executorResult.Status))
			}
			break
		}
	}

	if err := workflow.ExecuteActivity(ctx, CompletePlanExecutionActivityName, input).Get(ctx, nil); err != nil {
		return result, failPlan(ctx, input, fmt.Errorf("complete plan execution: %w", err))
	}
	result.Status = "completed"
	logger.Info("plan workflow completed",
		"tenant_id", input.TenantID,
		"plan_execution_id", input.PlanExecutionID,
	)
	return result, nil
}

const CreateScheduledPlanExecutionActivityName = "CreateScheduledPlanExecutionActivity"

func validatePlanSnapshot(snapshot PlanExecutionSnapshot) error {
	if snapshot.Configuration == nil {
		return errors.New("plan execution snapshot missing configuration")
	}
	if snapshot.Template == nil {
		return errors.New("plan execution snapshot missing template")
	}
	if len(snapshot.Template.Steps) == 0 {
		return errors.New("plan execution snapshot template has no steps")
	}
	if snapshot.ExecutorInstallations == nil {
		return errors.New("plan execution snapshot missing executor installation snapshots")
	}
	return nil
}

func failPlan(ctx workflow.Context, input PlanWorkflowInput, cause error) error {
	if failErr := workflow.ExecuteActivity(ctx, FailPlanExecutionActivityName, planFailureInput(input, cause)).Get(ctx, nil); failErr != nil {
		return fmt.Errorf("%w; additionally failed to mark plan failed: %v", cause, failErr)
	}
	return cause
}

func planFailureInput(input PlanWorkflowInput, cause error) PlanFailureInput {
	failure := PlanFailureInput{
		TenantID:        input.TenantID,
		PlanExecutionID: input.PlanExecutionID,
	}
	if cause != nil {
		failure.Error = cause.Error()
	}
	return failure
}

func failStepAndPlan(
	ctx workflow.Context,
	input PlanWorkflowInput,
	step *plansv1.PlanStep,
	inputArtifacts []ArtifactRef,
	installation ExecutorInstallationSnapshot,
	cause error,
) error {
	stepRecord, err := createRunningStep(ctx, input, step, inputArtifacts, installation)
	if err != nil {
		return failPlan(ctx, input, fmt.Errorf("%w; additionally failed to create failed step execution: %v", cause, err))
	}
	if err := workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
		TenantID:        input.TenantID,
		StepExecutionID: stepRecord.ID,
	}).Get(ctx, nil); err != nil {
		return failPlan(ctx, input, fmt.Errorf("%w; additionally failed to mark step failed: %v", cause, err))
	}
	return failPlan(ctx, input, cause)
}

func failTimedOutElicitation(
	ctx workflow.Context,
	input PlanWorkflowInput,
	step *plansv1.PlanStep,
	stepRecord StepExecutionRecord,
	policies *plansv1.PlanBehaviorPolicies,
) error {
	behavior := elicitationTimeoutBehavior(policies)
	failTimedOutStep := func(cause error) error {
		if err := workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
			TenantID:        input.TenantID,
			StepExecutionID: stepRecord.ID,
		}).Get(ctx, nil); err != nil {
			return failPlan(ctx, input, fmt.Errorf("%w; additionally failed to mark timed-out step failed: %v", cause, err))
		}
		// The runtime does not yet have branch continuation semantics; a failed mandatory step terminates the plan.
		return failPlan(ctx, input, cause)
	}

	switch behavior {
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_STEP:
		return failTimedOutStep(fmt.Errorf("step %q elicitation timed out; policy failed the step", step.Key))
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_SKIP_WITH_DEFAULT:
		return failTimedOutStep(fmt.Errorf("step %q elicitation timed out; deprecated skip-with-default policy failed the step", step.Key))
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_PLAN:
		return failTimedOutStep(fmt.Errorf("step %q elicitation timed out; policy failed the plan", step.Key))
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED:
		return failPlan(ctx, input, fmt.Errorf("step %q elicitation timed out despite pause-until-answered policy", step.Key))
	default:
		return failTimedOutStep(fmt.Errorf("step %q elicitation timed out using unsupported policy %s", step.Key, behavior))
	}
}

func createRunningStep(
	ctx workflow.Context,
	input PlanWorkflowInput,
	step *plansv1.PlanStep,
	inputArtifacts []ArtifactRef,
	installation ExecutorInstallationSnapshot,
) (StepExecutionRecord, error) {
	inputArtifactID := ""
	for _, artifact := range inputArtifacts {
		if strings.TrimSpace(artifact.ArtifactID) != "" {
			inputArtifactID = artifact.ArtifactID
			break
		}
	}

	var record StepExecutionRecord
	inputRef := firstArtifactRef(inputArtifacts)
	err := workflow.ExecuteActivity(ctx, CreateStepExecutionActivityName, CreateStepExecutionInput{
		TenantID:                     input.TenantID,
		PlanExecutionID:              input.PlanExecutionID,
		PlanStepKey:                  step.Key,
		InputArtifactID:              inputArtifactID,
		InputArtifactRef:             inputRef,
		ExecutorInstallationSnapshot: installation,
	}).Get(ctx, &record)
	return record, err
}

func runExecutorActivity(ctx workflow.Context, kind string, input ExecutorActivityInput) (ExecutorActivityResult, error) {
	var result ExecutorActivityResult
	var activityName string
	switch kind {
	case ExecutorKindIntegration:
		activityName = RunIntegrationActivityName
	case ExecutorKindAgent:
		activityName = RunAgentActivityName
		// Agent activities are served by the Python agent-runtime worker on a
		// dedicated task queue; routing here keeps them off the Go worker
		// (which would NotFound) and vice versa.
		ctx = workflow.WithTaskQueue(ctx, AgentTaskQueueName)
	default:
		return result, fmt.Errorf("unsupported executor kind %q for step %q", kind, input.PlanStepKey)
	}
	if err := workflow.ExecuteActivity(ctx, activityName, input).Get(ctx, &result); err != nil {
		return ExecutorActivityResult{}, err
	}
	return result, nil
}

func waitForElicitationResponse(
	ctx workflow.Context,
	input PlanWorkflowInput,
	stepExecutionID string,
	elicitationThreadID string,
	policies *plansv1.PlanBehaviorPolicies,
) (ElicitationResponseSignal, bool, error) {
	var response ElicitationResponseSignal
	var received bool
	var timedOut bool

	selector := workflow.NewSelector(ctx)
	signalChan := workflow.GetSignalChannel(ctx, PlanElicitationResponseSignalName)
	selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
		var candidate ElicitationResponseSignal
		c.Receive(ctx, &candidate)
		if validateElicitationResponseSignal(candidate, stepExecutionID, elicitationThreadID) == nil {
			response = candidate
			received = true
		}
	})

	if timeout := elicitationTimeoutDuration(policies); timeout > 0 {
		selector.AddFuture(workflow.NewTimer(ctx, timeout), func(f workflow.Future) {
			timedOut = true
		})
	}

	for !received && !timedOut {
		selector.Select(ctx)
	}
	if timedOut {
		return ElicitationResponseSignal{}, true, nil
	}
	if !received {
		return ElicitationResponseSignal{}, false, errors.New("no elicitation response received")
	}
	// A signal becomes auditable only after its identity has been validated for
	// the awaiting step/thread. Temporal replays schedule this activity again,
	// so the deterministic key includes the stable execution, step, and thread.
	scheduleAudit(ctx, AuditEvent{
		TenantID: input.TenantID, EventType: "workflow.signal_received", BoundedContext: "workflow_engine",
		SubjectType: "step_execution", SubjectID: stepExecutionID,
		DedupeKey: input.PlanExecutionID + ":signal:elicitation:" + stepExecutionID + ":" + elicitationThreadID,
		Diff:      []AuditDiff{{Field: "signal", After: PlanElicitationResponseSignalName}},
	})
	return response, false, nil
}

func waitForApprovalDecision(ctx workflow.Context, input PlanWorkflowInput, stepExecutionID string, approvalRequestID string) (ApprovalDecisionSignal, error) {
	signalChan := workflow.GetSignalChannel(ctx, PlanApprovalDecisionSignalName)
	for {
		var decision ApprovalDecisionSignal
		signalChan.Receive(ctx, &decision)
		if validateApprovalDecisionSignal(decision, stepExecutionID, approvalRequestID) == nil {
			decisionName := "reject"
			if decision.Approved {
				decisionName = "approve"
			}
			scheduleAudit(ctx, AuditEvent{
				TenantID: input.TenantID, EventType: "workflow.signal_received", BoundedContext: "workflow_engine",
				SubjectType: "approval_request", SubjectID: approvalRequestID,
				DedupeKey: input.PlanExecutionID + ":signal:approval:" + approvalRequestID + ":" + decisionName,
				Diff:      []AuditDiff{{Field: "signal", After: PlanApprovalDecisionSignalName}},
			})
			return decision, nil
		}
	}
}

func scheduleAudit(ctx workflow.Context, event AuditEvent) {
	// Best effort is intentional: state transitions have already been recorded
	// by their owning activities, and recorder queue pressure must not make a
	// deterministic workflow fail. The activity, never workflow code, writes.
	_ = workflow.ExecuteActivity(ctx, RecordAuditActivityName, event).Get(ctx, nil)
}

func validateElicitationResponseSignal(signal ElicitationResponseSignal, stepExecutionID string, elicitationThreadID string) error {
	if strings.TrimSpace(signal.StepExecutionID) != strings.TrimSpace(stepExecutionID) {
		return fmt.Errorf("step execution id %q does not match awaiting step %q", signal.StepExecutionID, stepExecutionID)
	}
	if strings.TrimSpace(signal.ElicitationThreadID) != strings.TrimSpace(elicitationThreadID) {
		return fmt.Errorf("elicitation thread id %q does not match awaiting thread %q", signal.ElicitationThreadID, elicitationThreadID)
	}
	if strings.TrimSpace(signal.ResponseArtifactID) == "" && strings.TrimSpace(signal.ResponseText) == "" {
		return errors.New("response_artifact_id or response_text is required")
	}
	return nil
}

func validateApprovalDecisionSignal(signal ApprovalDecisionSignal, stepExecutionID string, approvalRequestID string) error {
	if strings.TrimSpace(signal.StepExecutionID) != strings.TrimSpace(stepExecutionID) {
		return fmt.Errorf("step execution id %q does not match awaiting step %q", signal.StepExecutionID, stepExecutionID)
	}
	if strings.TrimSpace(signal.ApprovalRequestID) != strings.TrimSpace(approvalRequestID) {
		return fmt.Errorf("approval request id %q does not match awaiting request %q", signal.ApprovalRequestID, approvalRequestID)
	}
	return nil
}

func requiresReview(step *plansv1.PlanStep) bool {
	return step != nil && step.GetHumanInteractionPolicy().GetReviewMode() == plansv1.ReviewMode_REVIEW_MODE_REQUIRED
}

func createAndWaitForReview(ctx workflow.Context, input PlanWorkflowInput, record StepExecutionRecord, step *plansv1.PlanStep, subject ArtifactRef) (string, error) {
	if strings.TrimSpace(subject.ArtifactID) == "" || strings.TrimSpace(subject.ArtifactVersionID) == "" || strings.TrimSpace(subject.ContentHash) == "" {
		return "", fmt.Errorf("step %q review candidate is not version-pinned", step.Key)
	}
	var reviewID string
	err := workflow.ExecuteActivity(ctx, CreateReviewRequestActivityName, CreateReviewRequestInput{TenantID: input.TenantID, PlanExecutionID: input.PlanExecutionID, StepExecutionID: record.ID, PlanStepKey: step.Key, SubjectArtifactRef: subject}).Get(ctx, &reviewID)
	return reviewID, err
}

func waitForReviewDecision(ctx workflow.Context, input PlanWorkflowInput, stepExecutionID, reviewRequestID string) (ReviewDecisionSignal, error) {
	channel := workflow.GetSignalChannel(ctx, PlanReviewDecisionSignalName)
	for {
		var signal ReviewDecisionSignal
		channel.Receive(ctx, &signal)
		if strings.TrimSpace(signal.StepExecutionID) != strings.TrimSpace(stepExecutionID) || strings.TrimSpace(signal.ReviewRequestID) != strings.TrimSpace(reviewRequestID) {
			continue
		}
		if signal.Decision != "accept" && signal.Decision != "revise" {
			continue
		}
		if signal.Decision == "revise" && strings.TrimSpace(signal.Feedback) == "" {
			continue
		}
		return signal, nil
	}
}

func aliasUpstreamArtifact(step *plansv1.PlanStep, upstream []string, outputs map[string]ArtifactRef) (ArtifactRef, error) {
	if len(upstream) != 1 {
		return ArtifactRef{}, fmt.Errorf("optional step %q requires exactly one upstream artifact to alias", step.Key)
	}
	ref, ok := outputs[upstream[0]]
	if !ok || strings.TrimSpace(ref.ArtifactID) == "" {
		return ArtifactRef{}, fmt.Errorf("optional step %q cannot alias missing upstream output %q", step.Key, upstream[0])
	}
	if ref.ArtifactTypeKey != step.GetInputArtifactTypeId() || step.GetInputArtifactTypeId() != step.GetOutputArtifactTypeId() {
		return ArtifactRef{}, fmt.Errorf("optional step %q is not identity-shaped", step.Key)
	}
	ref.StepKey = step.Key
	return ref, nil
}

func firstArtifactRef(refs []ArtifactRef) ArtifactRef {
	for _, ref := range refs {
		if strings.TrimSpace(ref.ArtifactID) != "" {
			return ref
		}
	}
	return ArtifactRef{}
}

func elicitationTimeoutDuration(policies *plansv1.PlanBehaviorPolicies) time.Duration {
	switch elicitationTimeoutBehavior(policies) {
	case plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED:
		return 0
	default:
		hours := policies.GetElicitationTimeoutHours()
		if hours <= 0 {
			hours = defaultElicitationTimeoutHours
		}
		if hours > maxElicitationTimeoutHours {
			hours = maxElicitationTimeoutHours
		}
		return time.Duration(hours) * time.Hour
	}
}

// elicitationExpiresAt returns the RFC3339 deadline for an elicitation given the
// policy, or "" when the policy pauses indefinitely (no deadline).
func elicitationExpiresAt(ctx workflow.Context, policies *plansv1.PlanBehaviorPolicies) string {
	timeout := elicitationTimeoutDuration(policies)
	if timeout <= 0 {
		return ""
	}
	return workflow.Now(ctx).UTC().Add(timeout).Format(time.RFC3339)
}

func elicitationTimeoutBehavior(policies *plansv1.PlanBehaviorPolicies) plansv1.ElicitationTimeoutBehavior {
	if policies == nil || policies.GetElicitationTimeoutBehavior() == plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED {
		return plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED
	}
	return policies.GetElicitationTimeoutBehavior()
}

func requiresPublishApproval(step *plansv1.PlanStep, policies *plansv1.PlanBehaviorPolicies) bool {
	if !isPublishStep(step) {
		return false
	}
	return publishApprovalMode(policies) != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH
}

// contentOutputFormat returns the run's selected output format, normalizing
// APPROVAL_ONLY to TEXT_POST for step-selection purposes (approval-only runs
// the text-post branch under a require-approval policy).
func contentOutputFormat(policies *plansv1.PlanBehaviorPolicies) plansv1.ContentOutputFormat {
	if policies == nil {
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED
	}
	f := policies.GetContentOutputFormat()
	if f == plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_APPROVAL_ONLY {
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_TEXT_POST
	}
	return f
}

// formatForStep infers a step's output format from its artifact types. A step
// belongs to a branch when its input or output is a format-bearing type;
// DateRange/NewsList/TextDraft/PublishConfirmation steps are format-agnostic.
func formatForStep(step *plansv1.PlanStep) plansv1.ContentOutputFormat {
	if step == nil {
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED
	}
	for _, typeKey := range []string{step.GetInputArtifactTypeId(), step.GetOutputArtifactTypeId()} {
		switch strings.TrimSpace(typeKey) {
		case "harpia.artifacts.v1.LinkedInPostDraft":
			return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_TEXT_POST
		case "harpia.artifacts.v1.CarouselDraft":
			return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_CAROUSEL
		case "harpia.artifacts.v1.ImageAsset":
			return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_IMAGE_BACKED_POST
		}
	}
	return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED
}

// shouldSkipStepForFormat is true when the step belongs to a branch that is not
// the run's selected format. Format-agnostic steps (UNSPECIFIED) always run, and
// an unset (UNSPECIFIED) format policy runs everything (backward-compatible).
func shouldSkipStepForFormat(step *plansv1.PlanStep, policies *plansv1.PlanBehaviorPolicies) bool {
	selected := contentOutputFormat(policies)
	if selected == plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED {
		return false // no format policy → run everything (backward-compatible)
	}
	stepFormat := formatForStep(step)
	if stepFormat == plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED {
		return false // format-agnostic step
	}
	return stepFormat != selected
}

// shouldSkipStepForOptOut is true when the step declares an optional capability
// the run did not include. Such steps run no executor and need no binding
// (ADR-018 D4) — they are excluded from the run and recorded as SKIPPED.
func shouldSkipStepForOptOut(step *plansv1.PlanStep, config *plansv1.PlanConfiguration) bool {
	return !planrules.StepWillRun(step, config.GetIncludedOptionalCapabilities())
}

func publishApprovalMode(policies *plansv1.PlanBehaviorPolicies) plansv1.PublishApprovalMode {
	if policies == nil {
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL
	}
	switch policies.GetPublishApprovalMode() {
	case plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH:
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH
	case plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL:
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL
	default:
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL
	}
}

func isPublishStep(step *plansv1.PlanStep) bool {
	if step == nil {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(step.Key))
	title := strings.ToLower(strings.TrimSpace(step.Title))
	description := strings.ToLower(strings.TrimSpace(step.Description))
	defaultSKU := strings.ToLower(strings.TrimSpace(step.DefaultExecutorSkuKey))
	outputType := strings.ToLower(strings.TrimSpace(step.OutputArtifactTypeId))
	if strings.HasPrefix(key, "publish-") ||
		strings.Contains(title, "publish") ||
		strings.Contains(description, "publish") ||
		strings.Contains(defaultSKU, "publish") ||
		strings.Contains(outputType, "publishconfirmation") {
		return true
	}
	if requirement := step.ExecutorRequirement; requirement != nil {
		if strings.Contains(strings.ToLower(requirement.ConnectionType), "publish") {
			return true
		}
		for _, capability := range requirement.RequiredCapabilities {
			if strings.Contains(strings.ToLower(capability), "publish") {
				return true
			}
		}
	}
	return false
}

func planApprovalRequestID(planExecutionID string, stepExecutionID string) string {
	return fmt.Sprintf("approval-%s-%s", strings.TrimSpace(planExecutionID), strings.TrimSpace(stepExecutionID))
}

func topologicalPlanSteps(template *plansv1.PlanTemplate) ([]*plansv1.PlanStep, error) {
	if template == nil {
		return nil, errors.New("plan template is required")
	}
	steps := template.Steps
	indexByKey := make(map[string]int, len(steps))
	stepByKey := make(map[string]*plansv1.PlanStep, len(steps))
	for i, step := range steps {
		if step == nil {
			return nil, fmt.Errorf("plan template step at index %d is nil", i)
		}
		key := strings.TrimSpace(step.Key)
		if key == "" {
			return nil, fmt.Errorf("plan template step at index %d has empty key", i)
		}
		if _, exists := stepByKey[key]; exists {
			return nil, fmt.Errorf("duplicate plan step key %q", key)
		}
		indexByKey[key] = i
		stepByKey[key] = step
	}

	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps))
	for key := range stepByKey {
		inDegree[key] = 0
	}
	for _, edge := range template.Edges {
		if edge == nil {
			continue
		}
		from := strings.TrimSpace(edge.FromStepKey)
		to := strings.TrimSpace(edge.ToStepKey)
		if _, ok := stepByKey[from]; !ok {
			return nil, fmt.Errorf("dependency references unknown from_step_key %q", from)
		}
		if _, ok := stepByKey[to]; !ok {
			return nil, fmt.Errorf("dependency references unknown to_step_key %q", to)
		}
		dependents[from] = append(dependents[from], to)
		inDegree[to]++
	}

	ready := make([]string, 0, len(steps))
	for key, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, key)
		}
	}
	sortStepKeys(ready, indexByKey)

	ordered := make([]*plansv1.PlanStep, 0, len(steps))
	for len(ready) > 0 {
		key := ready[0]
		ready = ready[1:]
		ordered = append(ordered, stepByKey[key])

		sortStepKeys(dependents[key], indexByKey)
		for _, dependent := range dependents[key] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
				sortStepKeys(ready, indexByKey)
			}
		}
	}
	if len(ordered) != len(steps) {
		return nil, errors.New("plan template dependency graph contains a cycle")
	}
	return ordered, nil
}

func dependencyIndex(template *plansv1.PlanTemplate) map[string][]string {
	if template == nil {
		return nil
	}
	indexByKey := make(map[string]int, len(template.Steps))
	for i, step := range template.Steps {
		if step != nil {
			indexByKey[step.Key] = i
		}
	}
	deps := make(map[string][]string, len(template.Steps))
	for _, step := range template.Steps {
		if step != nil {
			deps[step.Key] = nil
		}
	}
	for _, edge := range template.Edges {
		if edge == nil {
			continue
		}
		from := strings.TrimSpace(edge.FromStepKey)
		to := strings.TrimSpace(edge.ToStepKey)
		deps[to] = append(deps[to], from)
	}
	for key := range deps {
		sortStepKeys(deps[key], indexByKey)
	}
	return deps
}

func seedArtifactsByStep(config *plansv1.PlanConfiguration) map[string][]ArtifactRef {
	seeds := make(map[string][]ArtifactRef)
	if config == nil {
		return seeds
	}
	for _, seed := range config.SeedArtifacts {
		if seed == nil {
			continue
		}
		stepKey := strings.TrimSpace(seed.StepKey)
		if stepKey == "" {
			continue
		}
		artifactTypeKey := internalArtifactTypeKey(seed.InputName)
		seeds[stepKey] = append(seeds[stepKey], ArtifactRef{
			Source:          "seed",
			StepKey:         stepKey,
			InputName:       seed.InputName,
			ArtifactID:      seed.ArtifactId,
			LiteralJSON:     seed.LiteralJson,
			ArtifactTypeKey: artifactTypeKey,
		})
	}
	for key := range seeds {
		sort.SliceStable(seeds[key], func(i, j int) bool {
			if seeds[key][i].InputName == seeds[key][j].InputName {
				return seeds[key][i].ArtifactID < seeds[key][j].ArtifactID
			}
			return seeds[key][i].InputName < seeds[key][j].InputName
		})
	}
	return seeds
}

func stepInputArtifacts(
	step *plansv1.PlanStep,
	upstreamStepKeys []string,
	seedArtifacts map[string][]ArtifactRef,
	outputs map[string]ArtifactRef,
) ([]ArtifactRef, error) {
	if step == nil {
		return nil, errors.New("plan step is required")
	}
	inputs := append([]ArtifactRef(nil), seedArtifacts[step.Key]...)
	for i := range inputs {
		if strings.TrimSpace(inputs[i].ArtifactTypeKey) == "" {
			if internal := internalArtifactTypeKey(inputs[i].InputName); internal != "" {
				inputs[i].ArtifactTypeKey = internal
			} else {
				inputs[i].ArtifactTypeKey = step.InputArtifactTypeId
			}
		}
	}
	if len(upstreamStepKeys) == 0 {
		if strings.TrimSpace(step.InputArtifactTypeId) != "" && len(inputs) == 0 {
			return nil, fmt.Errorf("step %q requires input artifact type %q but no seed artifact was configured", step.Key, step.InputArtifactTypeId)
		}
		return inputs, validateStepInputArtifactTypes(step, inputs)
	}

	for _, upstreamKey := range upstreamStepKeys {
		output, ok := outputs[upstreamKey]
		if !ok || strings.TrimSpace(output.ArtifactID) == "" {
			return nil, fmt.Errorf("step %q is missing validated upstream artifact from %q", step.Key, upstreamKey)
		}
		inputs = append(inputs, output)
	}
	if err := validateStepInputArtifactTypes(step, inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

func validateStepInputArtifactTypes(step *plansv1.PlanStep, inputs []ArtifactRef) error {
	if step == nil {
		return errors.New("plan step is required")
	}
	expected := strings.TrimSpace(step.InputArtifactTypeId)
	if expected == "" {
		return nil
	}
	for _, input := range inputs {
		actual := strings.TrimSpace(input.ArtifactTypeKey)
		if strings.HasPrefix(actual, "harpia.internal.") {
			continue
		}
		if actual != "" && actual != expected {
			return fmt.Errorf("step %q expected input artifact type %q but received %q from %q", step.Key, expected, actual, input.StepKey)
		}
	}
	return nil
}

func internalArtifactTypeKey(inputName string) string {
	trimmed := strings.TrimSpace(inputName)
	if strings.HasPrefix(trimmed, "harpia.internal.") {
		return trimmed
	}
	return ""
}

func sortStepKeys(keys []string, indexByKey map[string]int) {
	sort.SliceStable(keys, func(i, j int) bool {
		left, leftOK := indexByKey[keys[i]]
		right, rightOK := indexByKey[keys[j]]
		switch {
		case leftOK && rightOK && left != right:
			return left < right
		case leftOK != rightOK:
			return leftOK
		default:
			return keys[i] < keys[j]
		}
	})
}

func indexForRetryStep(order []*plansv1.PlanStep, retryStepKey string) int {
	key := strings.TrimSpace(retryStepKey)
	if key == "" {
		return -1
	}
	for idx, step := range order {
		if step != nil && strings.TrimSpace(step.Key) == key {
			return idx
		}
	}
	return -1
}
