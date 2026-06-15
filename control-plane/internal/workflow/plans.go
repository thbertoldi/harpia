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
	"go.temporal.io/sdk/workflow"
)

const PlanScheduledExecutionWorkflowName = "PlanScheduledExecution"
const PlanWorkflowName = "PlanWorkflow"

const (
	LoadPlanExecutionActivityName             = "LoadPlanExecutionActivity"
	StartPlanExecutionActivityName            = "StartPlanExecutionActivity"
	CreateStepExecutionActivityName           = "CreateStepExecutionActivity"
	RunIntegrationActivityName                = "RunIntegrationActivity"
	RunAgentActivityName                      = "RunAgentActivity"
	CompleteStepExecutionActivityName         = "CompleteStepExecutionActivity"
	FailStepExecutionActivityName             = "FailStepExecutionActivity"
	AwaitElicitationStepExecutionActivityName = "AwaitElicitationStepExecutionActivity"
	CompletePlanExecutionActivityName         = "CompletePlanExecutionActivity"
	FailPlanExecutionActivityName             = "FailPlanExecutionActivity"

	ExecutorResultStatusCompleted            = "completed"
	ExecutorResultStatusFailed               = "failed"
	ExecutorResultStatusElicitationRequested = "elicitation_requested"

	ExecutorKindIntegration = "integration"
	ExecutorKindAgent       = "agent"

	PlanExecutionStatusPending = "pending"
)

type PlanExecutionInput struct {
	TenantID            string `json:"tenant_id"`
	PlanConfigurationID string `json:"plan_configuration_id"`
	PlanExecutionID     string `json:"plan_execution_id,omitempty"`
}

type PlanWorkflowInput struct {
	TenantID        string `json:"tenant_id"`
	PlanExecutionID string `json:"plan_execution_id"`
}

type PlanWorkflowResult struct {
	PlanExecutionID string `json:"plan_execution_id"`
	Status          string `json:"status"`
}

type PlanExecutionSnapshot struct {
	SchemaVersion         int                                     `json:"schema_version"`
	Configuration         *plansv1.PlanConfiguration              `json:"configuration"`
	Template              *plansv1.PlanTemplate                   `json:"template"`
	ExecutorInstallations map[string]ExecutorInstallationSnapshot `json:"executor_installations"`
	SnapshotAt            string                                  `json:"snapshot_at,omitempty"`
}

type ExecutorInstallationSnapshot struct {
	ID               string `json:"id"`
	TenantID         string `json:"tenant_id"`
	ExecutorSKUID    string `json:"executor_sku_id"`
	Kind             string `json:"kind"`
	DisplayName      string `json:"display_name"`
	Enabled          bool   `json:"enabled"`
	ConnectionStatus string `json:"connection_status,omitempty"`
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
	ExecutorInstallationSnapshot ExecutorInstallationSnapshot `json:"executor_installation_snapshot"`
}

type StepExecutionRecord struct {
	ID              string `json:"id"`
	PlanStepKey     string `json:"plan_step_key"`
	Attempt         int32  `json:"attempt"`
	InputArtifactID string `json:"input_artifact_id,omitempty"`
}

type StepStatusUpdateInput struct {
	TenantID            string `json:"tenant_id"`
	StepExecutionID     string `json:"step_execution_id"`
	OutputArtifactID    string `json:"output_artifact_id,omitempty"`
	ElicitationThreadID string `json:"elicitation_thread_id,omitempty"`
}

type ArtifactRef struct {
	Source       string `json:"source"`
	StepKey      string `json:"step_key,omitempty"`
	InputName    string `json:"input_name,omitempty"`
	ArtifactID   string `json:"artifact_id,omitempty"`
	LiteralJSON  string `json:"literal_json,omitempty"`
	ArtifactType string `json:"artifact_type,omitempty"`
}

type ExecutorActivityInput struct {
	TenantID                     string                       `json:"tenant_id"`
	PlanExecutionID              string                       `json:"plan_execution_id"`
	StepExecutionID              string                       `json:"step_execution_id"`
	PlanStepKey                  string                       `json:"plan_step_key"`
	InputArtifacts               []ArtifactRef                `json:"input_artifacts"`
	OutputArtifactTypeID         string                       `json:"output_artifact_type_id"`
	ExecutorInstallationSnapshot ExecutorInstallationSnapshot `json:"executor_installation_snapshot"`
}

type ExecutorActivityResult struct {
	Status              string `json:"status"`
	OutputArtifactID    string `json:"output_artifact_id,omitempty"`
	ElicitationThreadID string `json:"elicitation_thread_id,omitempty"`
	Error               string `json:"error,omitempty"`
}

type PlanRuntimeStore interface {
	CreateScheduledExecution(ctx context.Context, tenantID, configID, executionID uuid.UUID) (PlanWorkflowInput, error)
	LoadPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) (LoadedPlanExecution, error)
	StartPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error
	CreateStepExecution(ctx context.Context, input CreateStepExecutionInput) (StepExecutionRecord, error)
	CompleteStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	FailStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	AwaitElicitationStepExecution(ctx context.Context, input StepStatusUpdateInput) error
	CompletePlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error
	FailPlanExecution(ctx context.Context, tenantID, executionID uuid.UUID) error
}

type PlanActivities struct {
	Runtime PlanRuntimeStore
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
	return a.Runtime.StartPlanExecution(ctx, tenantID, executionID)
}

func (a *PlanActivities) CreateStepExecutionActivity(ctx context.Context, input CreateStepExecutionInput) (StepExecutionRecord, error) {
	if a == nil || a.Runtime == nil {
		return StepExecutionRecord{}, fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.CreateStepExecution(ctx, input)
}

func (a *PlanActivities) CompleteStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.CompleteStepExecution(ctx, input)
}

func (a *PlanActivities) FailStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.FailStepExecution(ctx, input)
}

func (a *PlanActivities) AwaitElicitationStepExecutionActivity(ctx context.Context, input StepStatusUpdateInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	return a.Runtime.AwaitElicitationStepExecution(ctx, input)
}

func (a *PlanActivities) CompletePlanExecutionActivity(ctx context.Context, input PlanWorkflowInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	tenantID, executionID, err := parsePlanWorkflowIDs(input)
	if err != nil {
		return err
	}
	return a.Runtime.CompletePlanExecution(ctx, tenantID, executionID)
}

func (a *PlanActivities) FailPlanExecutionActivity(ctx context.Context, input PlanWorkflowInput) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("plan runtime store is not configured")
	}
	tenantID, executionID, err := parsePlanWorkflowIDs(input)
	if err != nil {
		return err
	}
	return a.Runtime.FailPlanExecution(ctx, tenantID, executionID)
}

func (a *PlanActivities) RunIntegrationActivity(ctx context.Context, input ExecutorActivityInput) (ExecutorActivityResult, error) {
	return ExecutorActivityResult{
		Status: ExecutorResultStatusFailed,
		Error:  fmt.Sprintf("RunIntegration is not implemented for step %q", input.PlanStepKey),
	}, nil
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

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

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

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	result := PlanWorkflowResult{PlanExecutionID: input.PlanExecutionID, Status: "failed"}

	var loaded LoadedPlanExecution
	if err := workflow.ExecuteActivity(ctx, LoadPlanExecutionActivityName, input).Get(ctx, &loaded); err != nil {
		return result, err
	}
	if err := validatePlanSnapshot(loaded.Snapshot); err != nil {
		_ = workflow.ExecuteActivity(ctx, FailPlanExecutionActivityName, input).Get(ctx, nil)
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
		_ = workflow.ExecuteActivity(ctx, FailPlanExecutionActivityName, input).Get(ctx, nil)
		return result, err
	}
	dependencies := dependencyIndex(loaded.Snapshot.Template)
	seedArtifacts := seedArtifactsByStep(loaded.Snapshot.Configuration)
	outputs := make(map[string]ArtifactRef, len(order))

	for _, step := range order {
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

		executorInput := ExecutorActivityInput{
			TenantID:                     input.TenantID,
			PlanExecutionID:              input.PlanExecutionID,
			StepExecutionID:              stepRecord.ID,
			PlanStepKey:                  step.Key,
			InputArtifacts:               inputArtifacts,
			OutputArtifactTypeID:         step.OutputArtifactTypeId,
			ExecutorInstallationSnapshot: installation,
		}
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
			if err := workflow.ExecuteActivity(ctx, CompleteStepExecutionActivityName, StepStatusUpdateInput{
				TenantID:         input.TenantID,
				StepExecutionID:  stepRecord.ID,
				OutputArtifactID: executorResult.OutputArtifactID,
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
			outputs[step.Key] = ArtifactRef{
				Source:       "step_output",
				StepKey:      step.Key,
				ArtifactID:   executorResult.OutputArtifactID,
				ArtifactType: step.OutputArtifactTypeId,
			}

		case ExecutorResultStatusElicitationRequested:
			if err := workflow.ExecuteActivity(ctx, FailStepExecutionActivityName, StepStatusUpdateInput{
				TenantID:        input.TenantID,
				StepExecutionID: stepRecord.ID,
			}).Get(ctx, nil); err != nil {
				return result, failPlan(ctx, input, err)
			}
			return result, failPlan(ctx, input, fmt.Errorf("step %q requested elicitation; policy enforcement is not implemented", step.Key))

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
	if failErr := workflow.ExecuteActivity(ctx, FailPlanExecutionActivityName, input).Get(ctx, nil); failErr != nil {
		return fmt.Errorf("%w; additionally failed to mark plan failed: %v", cause, failErr)
	}
	return cause
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
	err := workflow.ExecuteActivity(ctx, CreateStepExecutionActivityName, CreateStepExecutionInput{
		TenantID:                     input.TenantID,
		PlanExecutionID:              input.PlanExecutionID,
		PlanStepKey:                  step.Key,
		InputArtifactID:              inputArtifactID,
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
	default:
		return result, fmt.Errorf("unsupported executor kind %q for step %q", kind, input.PlanStepKey)
	}
	if err := workflow.ExecuteActivity(ctx, activityName, input).Get(ctx, &result); err != nil {
		return ExecutorActivityResult{}, err
	}
	return result, nil
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
		seeds[stepKey] = append(seeds[stepKey], ArtifactRef{
			Source:      "seed",
			StepKey:     stepKey,
			InputName:   seed.InputName,
			ArtifactID:  seed.ArtifactId,
			LiteralJSON: seed.LiteralJson,
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
	if len(upstreamStepKeys) == 0 {
		if strings.TrimSpace(step.InputArtifactTypeId) != "" && len(inputs) == 0 {
			return nil, fmt.Errorf("step %q requires input artifact type %q but no seed artifact was configured", step.Key, step.InputArtifactTypeId)
		}
		return inputs, nil
	}

	for _, upstreamKey := range upstreamStepKeys {
		output, ok := outputs[upstreamKey]
		if !ok || strings.TrimSpace(output.ArtifactID) == "" {
			return nil, fmt.Errorf("step %q is missing validated upstream artifact from %q", step.Key, upstreamKey)
		}
		inputs = append(inputs, output)
	}
	return inputs, nil
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
