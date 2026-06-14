package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type PlanHandler struct {
	repo      *Repository
	validator *BindingValidator
	schedule  *ScheduleManager
}

func NewPlanHandler(repo *Repository, executors ExecutorLookup, schedule *ScheduleManager) (*PlanHandler, error) {
	if repo == nil {
		return nil, errors.New("plans: repository is required")
	}
	if executors == nil {
		return nil, errors.New("plans: executor lookup is required")
	}
	return &PlanHandler{
		repo:      repo,
		validator: NewBindingValidator(executors),
		schedule:  schedule,
	}, nil
}

func (h *PlanHandler) GetPlanTemplate(ctx context.Context, req *connect.Request[plansv1.GetPlanTemplateRequest]) (*connect.Response[plansv1.GetPlanTemplateResponse], error) {
	templateID, err := uuid.Parse(req.Msg.PlanTemplateId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	template, err := h.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&plansv1.GetPlanTemplateResponse{
		PlanTemplate: templateToProto(template),
	}), nil
}

func (h *PlanHandler) GetPlanTemplateByKey(ctx context.Context, req *connect.Request[plansv1.GetPlanTemplateByKeyRequest]) (*connect.Response[plansv1.GetPlanTemplateByKeyResponse], error) {
	if req.Msg.Key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key is required"))
	}

	template, err := h.repo.GetTemplateByKey(ctx, req.Msg.Key)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&plansv1.GetPlanTemplateByKeyResponse{
		PlanTemplate: templateToProto(template),
	}), nil
}

func (h *PlanHandler) ListPlanTemplates(ctx context.Context, req *connect.Request[plansv1.ListPlanTemplatesRequest], stream *connect.ServerStream[plansv1.ListPlanTemplatesResponse]) error {
	vertical := ""
	if req.Msg.Vertical != nil {
		vertical = *req.Msg.Vertical
	}
	limit := int(req.Msg.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if req.Msg.PageToken != "" {
		parsed, parseErr := strconv.Atoi(req.Msg.PageToken)
		if parseErr != nil || parsed < 0 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid page token %q", req.Msg.PageToken))
		}
		offset = parsed
	}

	templates, err := h.repo.ListTemplates(ctx, vertical, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(templates) > limit
	if hasNext {
		templates = templates[:limit]
	}

	response := &plansv1.ListPlanTemplatesResponse{
		PlanTemplates: make([]*plansv1.PlanTemplate, 0, len(templates)),
	}
	for i := range templates {
		response.PlanTemplates = append(response.PlanTemplates, templateToProto(&templates[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *PlanHandler) CreatePlanConfiguration(ctx context.Context, req *connect.Request[plansv1.CreatePlanConfigurationRequest]) (*connect.Response[plansv1.CreatePlanConfigurationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	templateID, err := uuid.Parse(req.Msg.PlanTemplateId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	template, err := h.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("plan template not found: %w", err))
	}

	if err := h.validator.ValidateSlotBindings(ctx, tenantID, template, req.Msg.Status, req.Msg.SlotBindings); err != nil {
		return nil, connectErrorFromBinding(err)
	}

	config, err := h.buildConfigurationFromRequest(tenantID, template, req.Msg.WorkspaceId, req.Msg.Status,
		req.Msg.SeedArtifacts, req.Msg.SlotBindings, req.Msg.OverseerBindings,
		req.Msg.BehaviorPolicies, req.Msg.Schedule)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	created, err := h.repo.CreateConfiguration(ctx, config)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := h.syncSchedule(ctx, created); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&plansv1.CreatePlanConfigurationResponse{
		PlanConfiguration: configurationToProto(created),
	}), nil
}

func (h *PlanHandler) GetPlanConfiguration(ctx context.Context, req *connect.Request[plansv1.GetPlanConfigurationRequest]) (*connect.Response[plansv1.GetPlanConfigurationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	configID, err := uuid.Parse(req.Msg.PlanConfigurationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	config, err := h.repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&plansv1.GetPlanConfigurationResponse{
		PlanConfiguration: configurationToProto(config),
	}), nil
}

func (h *PlanHandler) UpdatePlanConfiguration(ctx context.Context, req *connect.Request[plansv1.UpdatePlanConfigurationRequest]) (*connect.Response[plansv1.UpdatePlanConfigurationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	configID, err := uuid.Parse(req.Msg.PlanConfigurationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	existing, err := h.repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	template, err := h.repo.GetTemplateByID(ctx, existing.PlanTemplateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	updatedInput := &plansv1.CreatePlanConfigurationRequest{
		WorkspaceId:      existing.WorkspaceID.UUID.String(),
		Status:           req.Msg.Status,
		SeedArtifacts:    req.Msg.SeedArtifacts,
		SlotBindings:     req.Msg.SlotBindings,
		OverseerBindings: req.Msg.OverseerBindings,
		BehaviorPolicies: req.Msg.BehaviorPolicies,
		Schedule:         req.Msg.Schedule,
	}
	if !existing.WorkspaceID.Valid {
		updatedInput.WorkspaceId = ""
	}

	if err := h.validator.ValidateSlotBindings(ctx, tenantID, template, updatedInput.Status, updatedInput.SlotBindings); err != nil {
		return nil, connectErrorFromBinding(err)
	}

	config, err := h.buildConfigurationFromRequest(tenantID, template, updatedInput.WorkspaceId, updatedInput.Status,
		updatedInput.SeedArtifacts, updatedInput.SlotBindings, updatedInput.OverseerBindings,
		updatedInput.BehaviorPolicies, updatedInput.Schedule)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	config.ID = existing.ID
	config.PlanTemplateID = existing.PlanTemplateID
	config.PlanTemplateVersion = existing.PlanTemplateVersion
	config.WorkspaceID = existing.WorkspaceID

	updated, err := h.repo.UpdateConfiguration(ctx, config)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := h.syncSchedule(ctx, updated); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&plansv1.UpdatePlanConfigurationResponse{
		PlanConfiguration: configurationToProto(updated),
	}), nil
}

func (h *PlanHandler) ListPlanConfigurations(ctx context.Context, req *connect.Request[plansv1.ListPlanConfigurationsRequest], stream *connect.ServerStream[plansv1.ListPlanConfigurationsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}

	status := ""
	if req.Msg.Status != nil && *req.Msg.Status != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED {
		var statusErr error
		status, statusErr = configurationStatusToString(*req.Msg.Status)
		if statusErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, statusErr)
		}
	}

	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != nil && *req.Msg.WorkspaceId != "" {
		parsed, parseErr := uuid.Parse(*req.Msg.WorkspaceId)
		if parseErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		workspaceID = &parsed
	}

	limit := int(req.Msg.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if req.Msg.PageToken != "" {
		parsed, parseErr := strconv.Atoi(req.Msg.PageToken)
		if parseErr != nil || parsed < 0 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid page token %q", req.Msg.PageToken))
		}
		offset = parsed
	}

	configs, err := h.repo.ListConfigurations(ctx, tenantID, workspaceID, status, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(configs) > limit
	if hasNext {
		configs = configs[:limit]
	}

	response := &plansv1.ListPlanConfigurationsResponse{
		PlanConfigurations: make([]*plansv1.PlanConfiguration, 0, len(configs)),
	}
	for i := range configs {
		response.PlanConfigurations = append(response.PlanConfigurations, configurationToProto(&configs[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *PlanHandler) CreatePlanExecution(ctx context.Context, req *connect.Request[plansv1.CreatePlanExecutionRequest]) (*connect.Response[plansv1.CreatePlanExecutionResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	configID, err := uuid.Parse(req.Msg.PlanConfigurationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	config, err := h.repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	template, err := h.repo.GetTemplateByID(ctx, config.PlanTemplateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := h.validator.ValidateConfigurationForExecution(ctx, tenantID, template, config); err != nil {
		return nil, connectErrorFromBinding(err)
	}

	snapshot, err := json.Marshal(configurationToProto(config))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now().UTC()
	execution, err := h.repo.CreateExecution(ctx, &PlanExecution{
		TenantID:                  tenantID,
		PlanConfigurationID:       configID,
		PlanConfigurationSnapshot: snapshot,
		Status:                    ExecutionStatusPending,
		TriggeredAt:               &now,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&plansv1.CreatePlanExecutionResponse{
		PlanExecution: executionToProto(execution),
	}), nil
}

func (h *PlanHandler) GetPlanExecution(ctx context.Context, req *connect.Request[plansv1.GetPlanExecutionRequest]) (*connect.Response[plansv1.GetPlanExecutionResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	executionID, err := uuid.Parse(req.Msg.PlanExecutionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	execution, err := h.repo.GetExecution(ctx, tenantID, executionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&plansv1.GetPlanExecutionResponse{
		PlanExecution: executionToProto(execution),
	}), nil
}

func (h *PlanHandler) ListPlanExecutions(ctx context.Context, req *connect.Request[plansv1.ListPlanExecutionsRequest], stream *connect.ServerStream[plansv1.ListPlanExecutionsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}

	var configID *uuid.UUID
	if req.Msg.PlanConfigurationId != nil && *req.Msg.PlanConfigurationId != "" {
		parsed, parseErr := uuid.Parse(*req.Msg.PlanConfigurationId)
		if parseErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		configID = &parsed
	}

	limit := int(req.Msg.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if req.Msg.PageToken != "" {
		parsed, parseErr := strconv.Atoi(req.Msg.PageToken)
		if parseErr != nil || parsed < 0 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid page token %q", req.Msg.PageToken))
		}
		offset = parsed
	}

	executions, err := h.repo.ListExecutions(ctx, tenantID, configID, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(executions) > limit
	if hasNext {
		executions = executions[:limit]
	}

	response := &plansv1.ListPlanExecutionsResponse{
		PlanExecutions: make([]*plansv1.PlanExecution, 0, len(executions)),
	}
	for i := range executions {
		response.PlanExecutions = append(response.PlanExecutions, executionToProto(&executions[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *PlanHandler) GetStepExecution(ctx context.Context, req *connect.Request[plansv1.GetStepExecutionRequest]) (*connect.Response[plansv1.GetStepExecutionResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	stepID, err := uuid.Parse(req.Msg.StepExecutionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	step, err := h.repo.GetStepExecution(ctx, tenantID, stepID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&plansv1.GetStepExecutionResponse{
		StepExecution: stepExecutionToProto(step),
	}), nil
}

func (h *PlanHandler) ListStepExecutions(ctx context.Context, req *connect.Request[plansv1.ListStepExecutionsRequest], stream *connect.ServerStream[plansv1.ListStepExecutionsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}

	executionID, err := uuid.Parse(req.Msg.PlanExecutionId)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	limit := int(req.Msg.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if req.Msg.PageToken != "" {
		parsed, parseErr := strconv.Atoi(req.Msg.PageToken)
		if parseErr != nil || parsed < 0 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid page token %q", req.Msg.PageToken))
		}
		offset = parsed
	}

	steps, err := h.repo.ListStepExecutions(ctx, tenantID, executionID, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(steps) > limit
	if hasNext {
		steps = steps[:limit]
	}

	response := &plansv1.ListStepExecutionsResponse{
		StepExecutions: make([]*plansv1.StepExecution, 0, len(steps)),
	}
	for i := range steps {
		response.StepExecutions = append(response.StepExecutions, stepExecutionToProto(&steps[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *PlanHandler) buildConfigurationFromRequest(
	tenantID uuid.UUID,
	template *PlanTemplate,
	workspaceID string,
	status plansv1.PlanConfigurationStatus,
	seedArtifacts []*plansv1.SeedArtifactBinding,
	slotBindings []*plansv1.SlotBinding,
	overseerBindings []*plansv1.OverseerBinding,
	behaviorPolicies *plansv1.PlanBehaviorPolicies,
	schedule *plansv1.PlanSchedule,
) (*PlanConfiguration, error) {
	statusStr, err := configurationStatusToString(status)
	if err != nil {
		return nil, err
	}
	if statusStr == "" {
		statusStr = ConfigurationStatusDraft
	}

	var workspace uuid.NullUUID
	if workspaceID != "" {
		parsed, parseErr := uuid.Parse(workspaceID)
		if parseErr != nil {
			return nil, parseErr
		}
		workspace = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	seedJSON, err := json.Marshal(seedArtifacts)
	if err != nil {
		return nil, err
	}
	slotJSON, err := json.Marshal(slotBindings)
	if err != nil {
		return nil, err
	}
	overseerJSON, err := json.Marshal(overseerBindings)
	if err != nil {
		return nil, err
	}
	policiesJSON, err := json.Marshal(behaviorPolicies)
	if err != nil {
		return nil, err
	}
	scheduleJSON, err := json.Marshal(schedule)
	if err != nil {
		return nil, err
	}
	if err := validateScheduleForStatus(statusStr, scheduleJSON); err != nil {
		return nil, err
	}

	return &PlanConfiguration{
		TenantID:            tenantID,
		WorkspaceID:         workspace,
		PlanTemplateID:      template.ID,
		PlanTemplateVersion: template.Version,
		Status:              statusStr,
		SeedArtifacts:       seedJSON,
		SlotBindings:        slotJSON,
		OverseerBindings:    overseerJSON,
		BehaviorPolicies:    policiesJSON,
		Schedule:            scheduleJSON,
	}, nil
}

func templateToProto(t *PlanTemplate) *plansv1.PlanTemplate {
	steps := make([]*plansv1.PlanStep, 0, len(t.Steps))
	for i := range t.Steps {
		steps = append(steps, stepToProto(&t.Steps[i]))
	}
	edges := make([]*plansv1.PlanStepDependency, 0, len(t.Edges))
	for i := range t.Edges {
		edges = append(edges, &plansv1.PlanStepDependency{
			FromStepKey: t.Edges[i].FromStepKey,
			ToStepKey:   t.Edges[i].ToStepKey,
		})
	}
	return &plansv1.PlanTemplate{
		Id:          t.ID.String(),
		Key:         t.Key,
		Name:        t.Name,
		Description: t.Description,
		Vertical:    t.Vertical,
		Version:     t.Version,
		Steps:       steps,
		Edges:       edges,
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
	}
}

func stepToProto(s *PlanStep) *plansv1.PlanStep {
	step := &plansv1.PlanStep{
		Id:                   s.ID.String(),
		Key:                  s.Key,
		Title:                s.Title,
		Description:          s.Description,
		InputArtifactTypeId:  s.InputArtifactTypeID,
		OutputArtifactTypeId: s.OutputArtifactTypeID,
		DefaultExecutorSkuKey: s.DefaultExecutorSKUKey,
	}
	if len(s.ExecutorRequirement) > 0 && string(s.ExecutorRequirement) != "{}" {
		var requirement plansv1.ExecutorRequirement
		if err := json.Unmarshal(s.ExecutorRequirement, &requirement); err == nil {
			step.ExecutorRequirement = &requirement
		}
	}
	return step
}

func configurationToProto(c *PlanConfiguration) *plansv1.PlanConfiguration {
	config := &plansv1.PlanConfiguration{
		Id:                  c.ID.String(),
		TenantId:            c.TenantID.String(),
		PlanTemplateId:      c.PlanTemplateID.String(),
		PlanTemplateVersion: c.PlanTemplateVersion,
		Status:              stringToConfigurationStatus(c.Status),
		CreatedAt:           c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           c.UpdatedAt.Format(time.RFC3339),
	}
	if c.WorkspaceID.Valid {
		config.WorkspaceId = c.WorkspaceID.UUID.String()
	}

	var seedArtifacts []*plansv1.SeedArtifactBinding
	if len(c.SeedArtifacts) > 0 {
		_ = json.Unmarshal(c.SeedArtifacts, &seedArtifacts)
	}
	config.SeedArtifacts = seedArtifacts

	var slotBindings []*plansv1.SlotBinding
	if len(c.SlotBindings) > 0 {
		_ = json.Unmarshal(c.SlotBindings, &slotBindings)
	}
	config.SlotBindings = slotBindings

	var overseerBindings []*plansv1.OverseerBinding
	if len(c.OverseerBindings) > 0 {
		_ = json.Unmarshal(c.OverseerBindings, &overseerBindings)
	}
	config.OverseerBindings = overseerBindings

	var behaviorPolicies plansv1.PlanBehaviorPolicies
	if len(c.BehaviorPolicies) > 0 && string(c.BehaviorPolicies) != "{}" && string(c.BehaviorPolicies) != "null" {
		if err := json.Unmarshal(c.BehaviorPolicies, &behaviorPolicies); err == nil {
			config.BehaviorPolicies = &behaviorPolicies
		}
	}

	var schedule plansv1.PlanSchedule
	if len(c.Schedule) > 0 && string(c.Schedule) != "{}" && string(c.Schedule) != "null" {
		if err := json.Unmarshal(c.Schedule, &schedule); err == nil {
			config.Schedule = &schedule
		}
	}

	return config
}

func executionToProto(e *PlanExecution) *plansv1.PlanExecution {
	execution := &plansv1.PlanExecution{
		Id:                  e.ID.String(),
		TenantId:            e.TenantID.String(),
		PlanConfigurationId: e.PlanConfigurationID.String(),
		Status:              stringToExecutionStatus(e.Status),
		CreatedAt:           e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           e.UpdatedAt.Format(time.RFC3339),
	}
	if e.TriggeredAt != nil {
		execution.TriggeredAt = e.TriggeredAt.Format(time.RFC3339)
	}
	if e.CompletedAt != nil {
		execution.CompletedAt = e.CompletedAt.Format(time.RFC3339)
	}

	if len(e.PlanConfigurationSnapshot) > 0 {
		var snapshot plansv1.PlanConfiguration
		if err := json.Unmarshal(e.PlanConfigurationSnapshot, &snapshot); err == nil {
			execution.PlanConfigurationSnapshot = &snapshot
		}
	}

	if len(e.StepExecutions) > 0 {
		execution.StepExecutions = make([]*plansv1.StepExecution, 0, len(e.StepExecutions))
		for i := range e.StepExecutions {
			execution.StepExecutions = append(execution.StepExecutions, stepExecutionToProto(&e.StepExecutions[i]))
		}
	}

	return execution
}

func stepExecutionToProto(s *StepExecution) *plansv1.StepExecution {
	return &plansv1.StepExecution{
		Id:                           s.ID.String(),
		PlanExecutionId:              s.PlanExecutionID.String(),
		PlanStepKey:                  s.PlanStepKey,
		Status:                       stringToStepExecutionStatus(s.Status),
		InputArtifactId:              s.InputArtifactID,
		OutputArtifactId:             s.OutputArtifactID,
		ExecutorInstallationSnapshot: string(s.ExecutorInstallationSnapshot),
		Attempt:                      s.Attempt,
		ElicitationThreadId:          s.ElicitationThreadID,
		ApprovalRequestId:            s.ApprovalRequestID,
		CreatedAt:                    s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                    s.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *PlanHandler) syncSchedule(ctx context.Context, config *PlanConfiguration) error {
	if h.schedule == nil {
		return nil
	}
	return h.schedule.Sync(ctx, config)
}

func configurationStatusToString(status plansv1.PlanConfigurationStatus) (string, error) {
	switch status {
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED:
		return ConfigurationStatusDraft, nil
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT:
		return ConfigurationStatusDraft, nil
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE:
		return ConfigurationStatusRunnable, nil
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED:
		return ConfigurationStatusScheduled, nil
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DISABLED:
		return ConfigurationStatusDisabled, nil
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_ARCHIVED:
		return ConfigurationStatusArchived, nil
	default:
		return "", fmt.Errorf("unsupported plan configuration status %s", status.String())
	}
}

func stringToConfigurationStatus(s string) plansv1.PlanConfigurationStatus {
	switch s {
	case ConfigurationStatusDraft:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT
	case ConfigurationStatusRunnable:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE
	case ConfigurationStatusScheduled:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED
	case ConfigurationStatusDisabled:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DISABLED
	case ConfigurationStatusArchived:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_ARCHIVED
	default:
		return plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED
	}
}

func stringToExecutionStatus(s string) plansv1.PlanExecutionStatus {
	switch s {
	case ExecutionStatusPending:
		return plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_PENDING
	case ExecutionStatusRunning:
		return plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_RUNNING
	case ExecutionStatusCompleted:
		return plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_COMPLETED
	case ExecutionStatusFailed:
		return plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_FAILED
	case ExecutionStatusCancelled:
		return plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_CANCELLED
	default:
		return plansv1.PlanExecutionStatus_PLAN_EXECUTION_STATUS_UNSPECIFIED
	}
}

func stringToStepExecutionStatus(s string) plansv1.StepExecutionStatus {
	switch s {
	case StepStatusPending:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_PENDING
	case StepStatusRunning:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_RUNNING
	case StepStatusAwaitingElicitation:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_AWAITING_ELICITATION
	case StepStatusAwaitingApproval:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_AWAITING_APPROVAL
	case StepStatusCompleted:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_COMPLETED
	case StepStatusFailed:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_FAILED
	default:
		return plansv1.StepExecutionStatus_STEP_EXECUTION_STATUS_UNSPECIFIED
	}
}
