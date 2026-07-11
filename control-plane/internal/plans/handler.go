package plans

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/database"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/planassistant"
	"github.com/harpia/control-plane/internal/workflow"
	cron "github.com/robfig/cron/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

// AssistantConfigurationStore adapts *Repository to planassistant.ConfigurationStore.
type AssistantConfigurationStore struct {
	Repo      *Repository
	Executors ExecutorLookup
}

func (a *AssistantConfigurationStore) GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*plansv1.PlanConfiguration, error) {
	cfg, err := a.Repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, err
	}
	// The opt-in capability set is a derived runtime projection (ADR-018 D4).
	// Populate it from the template + parameter values so the assistant state
	// machine can tell opted-in optional steps from opted-out ones. A missing
	// template falls back to an empty set (everything optional opted out).
	if template, err := a.Repo.GetTemplateByID(ctx, cfg.PlanTemplateID); err == nil {
		return configurationToProto(cfg, template), nil
	}
	return configurationToProto(cfg), nil
}

func configurationFromProto(proto *plansv1.PlanConfiguration, existing *PlanConfiguration) (*PlanConfiguration, error) {
	if existing == nil {
		return nil, errors.New("existing configuration is required")
	}
	statusStr, err := configurationStatusToString(proto.GetStatus())
	if err != nil {
		return nil, err
	}
	seedJSON, err := json.Marshal(proto.GetSeedArtifacts())
	if err != nil {
		return nil, err
	}
	slotJSON, err := json.Marshal(proto.GetSlotBindings())
	if err != nil {
		return nil, err
	}
	overseerJSON, err := json.Marshal(proto.GetOverseerBindings())
	if err != nil {
		return nil, err
	}
	policiesJSON, err := json.Marshal(proto.GetBehaviorPolicies())
	if err != nil {
		return nil, err
	}
	scheduleJSON, err := json.Marshal(proto.GetSchedule())
	if err != nil {
		return nil, err
	}
	parameterValuesJSON, err := normalizeParameterValuesJSON(proto.GetParameterValuesJson())
	if err != nil {
		return nil, err
	}
	return &PlanConfiguration{
		ID:                  existing.ID,
		TenantID:            existing.TenantID,
		WorkspaceID:         existing.WorkspaceID,
		PlanTemplateID:      existing.PlanTemplateID,
		PlanTemplateVersion: existing.PlanTemplateVersion,
		Status:              statusStr,
		Kind:                existing.Kind,
		SeedArtifacts:       seedJSON,
		SlotBindings:        slotJSON,
		OverseerBindings:    overseerJSON,
		BehaviorPolicies:    policiesJSON,
		Schedule:            scheduleJSON,
		ParameterValues:     parameterValuesJSON,
		OriginThreadID:      existing.OriginThreadID,
		CreatedAt:           existing.CreatedAt,
		UpdatedAt:           existing.UpdatedAt,
	}, nil
}

func dbExecutorKindToProto(kind string) (plansv1.ExecutorKind, error) {
	switch kind {
	case "integration":
		return plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION, nil
	case "agent":
		return plansv1.ExecutorKind_EXECUTOR_KIND_AGENT, nil
	default:
		return plansv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED, fmt.Errorf("unsupported executor kind %q", kind)
	}
}

func tierFromSKUKey(key string) string {
	for _, tier := range []string{"junior", "senior", "specialist"} {
		if strings.Contains(key, tier) {
			return tier
		}
	}
	return ""
}

// AssistantTemplates adapts *Repository to planassistant.TemplateStore.
type AssistantTemplates struct{ Repo *Repository }

func (a *AssistantTemplates) GetTemplateByID(ctx context.Context, id uuid.UUID) (*plansv1.PlanTemplate, error) {
	tpl, err := a.Repo.GetTemplateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return templateToProto(tpl), nil
}

// AssistantCatalog adapts ExecutorLookup to planassistant.ExecutorCatalog.
type AssistantCatalog struct{ Executors ExecutorLookup }

func (a *AssistantCatalog) CandidatesForStep(ctx context.Context, tenantID uuid.UUID, template *plansv1.PlanTemplate, stepKey string) ([]planassistant.ExecutorOption, error) {
	var out []planassistant.ExecutorOption
	for _, step := range template.GetSteps() {
		if step.GetKey() != stepKey {
			continue
		}
		installations, err := a.Executors.ListCompatibleInstallationsForStep(ctx, tenantID, step)
		if err != nil {
			return nil, err
		}
		for _, inst := range installations {
			sku, err := a.Executors.GetSKUByID(ctx, inst.ExecutorSKUID)
			if err != nil {
				continue
			}
			var price *float64
			if sku.PriceCents > 0 {
				p := float64(sku.PriceCents) / 100.0
				price = &p
			}
			out = append(out, planassistant.ExecutorOption{
				StepKey:        stepKey,
				InstallationID: inst.ID.String(),
				DisplayName:    inst.DisplayName,
				SkuKey:         sku.Key,
				Tier:           tierFromSKUKey(sku.Key),
				PriceBrl:       price,
			})
		}
	}
	return out, nil
}

type PlanHandler struct {
	chat             chat.Store
	assistant        *planassistant.Controller
	repo             *Repository
	executors        ExecutorLookup
	validator        *BindingValidator
	schedule         *ScheduleManager
	runtime          PlanExecutionRetryer
	workflowStarter  PlanWorkflowStarter
	elicitations     ElicitationStore
	signaler         PlanElicitationSignaler
	approvals        ApprovalStore
	approvalSignaler PlanApprovalSignaler
}

type PlanWorkflowStarter interface {
	StartPlanWorkflow(ctx context.Context, input workflow.PlanWorkflowInput) (client.WorkflowRun, error)
}

type PlanExecutionRetryer interface {
	PrepareRetryFromStep(ctx context.Context, tenantID uuid.UUID, planExecutionID uuid.UUID, failedStepExecutionID uuid.UUID) (*PlanExecution, workflow.PlanWorkflowInput, error)
}

func NewPlanHandler(repo *Repository, executors ExecutorLookup, schedule *ScheduleManager, chatStore chat.Store, assistant *planassistant.Controller, starters ...PlanWorkflowStarter) (*PlanHandler, error) {
	if repo == nil {
		return nil, errors.New("plans: repository is required")
	}
	if executors == nil {
		return nil, errors.New("plans: executor lookup is required")
	}
	var starter PlanWorkflowStarter
	if len(starters) > 0 {
		starter = starters[0]
	}
	handler := &PlanHandler{
		chat:            chatStore,
		assistant:       assistant,
		repo:            repo,
		executors:       executors,
		validator:       NewBindingValidator(executors),
		schedule:        schedule,
		runtime:         NewRuntimeRepository(repo, executors, chatStore),
		workflowStarter: starter,
		elicitations:    repo,
		approvals:       repo,
	}
	if signaler, ok := starter.(PlanElicitationSignaler); ok {
		handler.signaler = signaler
	}
	if approvalSignaler, ok := starter.(PlanApprovalSignaler); ok {
		handler.approvalSignaler = approvalSignaler
	}
	return handler, nil
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

	threadID, err := uuid.Parse(strings.TrimSpace(req.Msg.GetThreadId()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("thread_id is required"))
	}

	templateID, err := uuid.Parse(req.Msg.PlanTemplateId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	template, err := h.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("plan template not found: %w", err))
	}

	seedArtifacts, slotBindings, behaviorPolicies, included, err := h.materializeConfigurationProjection(ctx, tenantID, template, req.Msg.ParameterValuesJson)
	if err != nil {
		return nil, err
	}

	if err := h.validator.ValidateSlotBindings(ctx, tenantID, template, req.Msg.Status, slotBindings, included); err != nil {
		return nil, connectErrorFromBinding(err)
	}
	if err := h.validator.ValidateOverseerBindings(template, req.Msg.Status, req.Msg.OverseerBindings, included); err != nil {
		return nil, connectErrorFromBinding(err)
	}

	// Auto-assign the current user as overseer for every agent-backed step the
	// caller did not explicitly bind. Validation has already run on the
	// caller-supplied bindings, so this only fills gaps for well-formed input.
	// With every agent step overseer-bound, DeriveState skips OVERSEER_STEP and
	// the conversational flow advances BINDING_STEP → POLICIES_STEP →
	// BINDING_MATRIX without the pointless "who oversees this?" prompt.
	overseerBindings := autoBindOverseers(ctx, template, req.Msg.OverseerBindings, included)

	config, err := h.buildConfigurationFromRequest(tenantID, template, req.Msg.WorkspaceId, req.Msg.Status, req.Msg.Kind,
		seedArtifacts, slotBindings, overseerBindings,
		behaviorPolicies, req.Msg.Schedule, req.Msg.ParameterValuesJson)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	config.OriginThreadID = threadID

	created, err := h.repo.CreateConfiguration(ctx, config)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := h.syncSchedule(ctx, created); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Path B: announce that a plan was attached to the owning thread, before the
	// assistant seeds the binding-matrix prompt.
	if h.chat != nil && created.OriginThreadID != uuid.Nil {
		_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    created.OriginThreadID.String(),
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_ATTACHED,
			Text:        "Plan attached.",
			PayloadJSON: chat.BuildPlanAttachedPayload(created.ID.String(), created.PlanTemplateID.String()),
		})
	}

	if h.assistant != nil {
		if err := h.assistant.SeedThread(ctx, tenantID, created.ID); err != nil {
			_ = err
		}
	}

	return connect.NewResponse(&plansv1.CreatePlanConfigurationResponse{
		PlanConfiguration: configurationToProto(created, template),
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

	// Populate the derived opt-in capability projection (ADR-018 D4) when the
	// template is available, mirroring AssistantConfigurationStore.GetConfiguration.
	// A missing template falls back to an empty set (everything optional opted out).
	if template, err := h.repo.GetTemplateByID(ctx, config.PlanTemplateID); err == nil {
		return connect.NewResponse(&plansv1.GetPlanConfigurationResponse{
			PlanConfiguration: configurationToProto(config, template),
		}), nil
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

	// Pre-load to resolve the template and to drive the post-mutation
	// schedule/status diffs. applyConfigurationUpdate re-reads the row inside
	// its own transaction so the persisted projection stays consistent.
	existing, err := h.repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	template, err := h.repo.GetTemplateByID(ctx, existing.PlanTemplateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var (
		lockedExisting *PlanConfiguration
		updated        *PlanConfiguration
	)
	err = database.WithTenant(ctx, h.repo.pool, tenantID, func(q database.Querier) error {
		var current PlanConfiguration
		if err := getConfigurationQ(ctx, q, tenantID, configID, &current); err != nil {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("plan configuration not found: %w", err))
		}
		lockedExisting = &current
		if threadID := configurationThreadID(&current); threadID != uuid.Nil {
			if err := chat.LockThreadForUpdate(ctx, q, tenantID, threadID); err != nil {
				return connect.NewError(connect.CodeNotFound, err)
			}
		}

		var updateErr error
		updated, updateErr = h.applyConfigurationUpdate(
			ctx, q, tenantID, configID, template,
			req.Msg.ParameterValuesJson,
			req.Msg.OverseerBindings,
			req.Msg.Status,
			req.Msg.Kind,
			req.Msg.Schedule,
		)
		return updateErr
	})
	if err != nil {
		return nil, err
	}

	if err := h.syncSchedule(ctx, updated); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if lockedExisting != nil {
		existing = lockedExisting
	}

	// Schedule reconciliation: if the cron expression changed, validate it,
	// emit SCHEDULE_SET, and reconcile RUNNABLE ↔ SCHEDULED. See M5 design
	// spec §2.7 and §2.11.
	prevCron := ""
	prevTz := ""
	if len(existing.Schedule) > 0 && string(existing.Schedule) != "{}" && string(existing.Schedule) != "null" {
		var prevSchedule plansv1.PlanSchedule
		if err := json.Unmarshal(existing.Schedule, &prevSchedule); err == nil {
			prevCron = prevSchedule.CronExpression
			prevTz = prevSchedule.Timezone
		}
	}
	newCron := ""
	newTz := ""
	if req.Msg.Schedule != nil {
		newCron = req.Msg.Schedule.CronExpression
		newTz = req.Msg.Schedule.Timezone
	}
	if newCron != prevCron || newTz != prevTz {
		if newCron != "" {
			if _, parseErr := cron.ParseStandard(newCron); parseErr != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("invalid schedule cron expression: %w", parseErr))
			}
		}
		switch updated.Status {
		case ConfigurationStatusRunnable:
			if newCron != "" {
				updated.Status = ConfigurationStatusScheduled
				_ = h.repo.UpdateConfigurationStatus(ctx, tenantID, updated.ID, plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED)
			}
		case ConfigurationStatusScheduled:
			if newCron == "" {
				updated.Status = ConfigurationStatusRunnable
				_ = h.repo.UpdateConfigurationStatus(ctx, tenantID, updated.ID, plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE)
			}
		}
		if h.chat != nil {
			threadID := configurationThreadID(updated)
			if threadID != uuid.Nil {
				_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
					ThreadID:    threadID.String(),
					Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
					Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_SCHEDULE_SET,
					Text:        "Schedule updated.",
					PayloadJSON: chat.BuildScheduleSetPayload(newCron, newTz),
				})
			}
		}
	}

	// Only announce on explicit user saves. Incremental edits (per-executor
	// binding picks, Suggest) set announce_saved=false to avoid flooding the
	// thread with a saved message on every change.
	if h.chat != nil && req.Msg.AnnounceSaved {
		threadID := configurationThreadID(updated)
		if threadID != uuid.Nil {
			_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    threadID.String(),
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_SAVED,
				Text:        "Configuration updated.",
				PayloadJSON: chat.BuildConfigurationSavedPayload(),
			})
		}
	}

	// M6 (spec §2.1): when the matrix card's Save promotes status out of
	// DRAFT, fire the assistant turn that emits the LandingCard. Best-effort
	// — the status change is already persisted; NextTurn is idempotent, so a
	// duplicate trigger (e.g. via the Save USER_SELECTION path) is harmless.
	if h.assistant != nil &&
		existing.Status == ConfigurationStatusDraft &&
		updated.Status != ConfigurationStatusDraft {
		_ = h.assistant.NextTurn(ctx, tenantID, updated.ID)
	}

	return connect.NewResponse(&plansv1.UpdatePlanConfigurationResponse{
		PlanConfiguration: configurationToProto(updated, template),
	}), nil
}

// NextTurn derives and appends the configuration assistant's next prompt
// (binding/overseer/policies/matrix) for a configuration. This is the
// client-callable trigger that keeps the conversational configuration flow
// advancing after each step — see planassistant.Controller.NextTurn.
func (h *PlanHandler) NextTurn(ctx context.Context, req *connect.Request[plansv1.NextTurnRequest]) (*connect.Response[plansv1.NextTurnResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	configID, err := uuid.Parse(req.Msg.PlanConfigurationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if h.assistant == nil {
		// No assistant wired (e.g. dev without planassistant); nothing to advance.
		return connect.NewResponse(&plansv1.NextTurnResponse{}), nil
	}
	if err := h.assistant.NextTurn(ctx, tenantID, configID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&plansv1.NextTurnResponse{}), nil
}

func (h *PlanHandler) SubmitConfigurationSelection(ctx context.Context, req *connect.Request[plansv1.SubmitConfigurationSelectionRequest]) (*connect.Response[plansv1.SubmitConfigurationSelectionResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	configID, err := uuid.Parse(req.Msg.PlanConfigurationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	promptID := strings.TrimSpace(req.Msg.AssistantPromptMessageId)
	if promptID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("assistant_prompt_message_id is required"))
	}
	selection := req.Msg.GetSelection()
	if selection == nil || strings.TrimSpace(selection.GetValue()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("selection.value is required"))
	}

	existing, err := h.repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	template, err := h.repo.GetTemplateByID(ctx, existing.PlanTemplateID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var (
		updated        *PlanConfiguration
		appended       []*chatv1.ThreadMessage
		alreadyApplied bool
	)
	err = database.WithTenant(ctx, h.repo.pool, tenantID, func(q database.Querier) error {
		var current PlanConfiguration
		if err := getConfigurationQ(ctx, q, tenantID, configID, &current); err != nil {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("plan configuration not found: %w", err))
		}
		threadID := configurationThreadID(&current)
		if threadID == uuid.Nil {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("plan configuration has no owning thread"))
		}
		if err := chat.LockThreadForUpdate(ctx, q, tenantID, threadID); err != nil {
			return connect.NewError(connect.CodeNotFound, err)
		}
		messages, err := chat.ListMessagesTx(ctx, q, tenantID, threadID.String(), 0, 0)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		prompt, promptPayload, err := selectionPrompt(messages, promptID)
		if err != nil {
			return err
		}
		if promptPayload.ConfigurationID != current.ID.String() {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("assistant prompt does not belong to this plan configuration"))
		}
		if promptAlreadyAnswered(messages, promptID) {
			updated = &current
			alreadyApplied = true
			return nil
		}
		if latest := latestUnansweredPromptID(messages, current.ID.String()); latest != promptID {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("assistant prompt is no longer current"))
		}
		label, ok := selectionLabel(promptPayload, selection)
		if !ok && promptPayload.State != string(planassistant.StateBindingMatrix) {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("selection does not match prompt options"))
		}
		if label == "" {
			label = selectionTextForValue(selection.GetValue())
		}

		userMsg, err := chat.AppendMessageTx(ctx, q, tenantID, chat.AppendInput{
			ThreadID:    threadID.String(),
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION,
			Text:        label,
			PayloadJSON: chat.BuildUserSelectionPayload(prompt.GetId(), selection.GetOptionId(), selection.GetValue()),
		})
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		appended = append(appended, userMsg)
		messages = append(messages, userMsg)

		var rebound *chat.AppendInput
		updated, rebound, err = h.applyConfigurationSelection(ctx, q, tenantID, &current, template, promptPayload, selection, label)
		if err != nil {
			return err
		}
		if rebound != nil {
			rebound.ThreadID = threadID.String()
			reboundMsg, err := chat.AppendMessageTx(ctx, q, tenantID, *rebound)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			appended = append(appended, reboundMsg)
			messages = append(messages, reboundMsg)
		}

		nextPrompt, err := h.appendNextAssistantPromptTx(ctx, q, tenantID, template, updated, messages)
		if err != nil {
			return err
		}
		if nextPrompt != nil {
			appended = append(appended, nextPrompt)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if updated != nil {
		if err := h.syncSchedule(ctx, updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&plansv1.SubmitConfigurationSelectionResponse{
		PlanConfiguration: configurationToProto(updated, template),
		Messages:          appended,
		AlreadyApplied:    alreadyApplied,
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
	kind := ""
	if req.Msg.Kind != nil && *req.Msg.Kind != plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED {
		var kindErr error
		kind, kindErr = configurationKindToString(*req.Msg.Kind, nil)
		if kindErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, kindErr)
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
	var originThreadID *uuid.UUID
	if req.Msg.OriginThreadId != nil && *req.Msg.OriginThreadId != "" {
		parsed, parseErr := uuid.Parse(*req.Msg.OriginThreadId)
		if parseErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		originThreadID = &parsed
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

	configs, err := h.repo.ListConfigurations(ctx, tenantID, workspaceID, originThreadID, status, kind, limit+1, offset)
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
	if h.workflowStarter == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workflow engine is unavailable"))
	}

	snapshot, err := buildPlanExecutionSnapshot(ctx, tenantID, config, template, h.executors, time.Now().UTC())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rawSnapshot, err := marshalPlanExecutionSnapshot(snapshot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now().UTC()
	execution, err := h.repo.CreateExecution(ctx, &PlanExecution{
		TenantID:                  tenantID,
		PlanConfigurationID:       configID,
		PlanConfigurationSnapshot: rawSnapshot,
		Status:                    ExecutionStatusPending,
		TriggeredAt:               &now,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := h.workflowStarter.StartPlanWorkflow(ctx, workflow.PlanWorkflowInput{
		TenantID:        tenantID.String(),
		PlanExecutionID: execution.ID.String(),
	}); err != nil {
		completedAt := time.Now().UTC()
		if markErr := h.repo.UpdateExecutionStatus(ctx, tenantID, execution.ID, ExecutionStatusFailed, &completedAt); markErr != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("start plan workflow: %w; additionally failed to mark execution failed: %v", err, markErr))
		}
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("start plan workflow: %w", err))
	}

	return connect.NewResponse(&plansv1.CreatePlanExecutionResponse{
		PlanExecution: executionToProto(execution),
	}), nil
}

func (h *PlanHandler) RetryPlanExecution(ctx context.Context, req *connect.Request[plansv1.RetryPlanExecutionRequest]) (*connect.Response[plansv1.RetryPlanExecutionResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if h.runtime == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("plan runtime is unavailable"))
	}
	if h.workflowStarter == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("workflow engine is unavailable"))
	}

	executionID, err := uuid.Parse(req.Msg.PlanExecutionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parse plan execution id: %w", err))
	}
	stepExecutionID, err := uuid.Parse(req.Msg.StepExecutionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parse step execution id: %w", err))
	}

	retryExecution, workflowInput, err := h.runtime.PrepareRetryFromStep(ctx, tenantID, executionID, stepExecutionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	if _, err := h.workflowStarter.StartPlanWorkflow(ctx, workflowInput); err != nil {
		completedAt := time.Now().UTC()
		if markErr := h.repo.UpdateExecutionStatus(ctx, tenantID, retryExecution.ID, ExecutionStatusFailed, &completedAt); markErr != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("start retry plan workflow: %w; additionally failed to mark execution failed: %v", err, markErr))
		}
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("start retry plan workflow: %w", err))
	}

	return connect.NewResponse(&plansv1.RetryPlanExecutionResponse{
		PlanExecution: executionToProto(retryExecution),
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

// autoBindOverseers returns the caller's overseer bindings plus an
// auto-assigned entry for every agent-backed PlanStep the caller left unbound:
// the current user becomes the overseer. This removes the OVERSEER_STEP prompt
// from the conversational configuration flow — in practice the only sensible
// answer to "who oversees {agent-step}?" is the configuring user themselves.
//
// Caller-supplied bindings always win and are never overwritten. When the
// context carries no authenticated user id, the caller's bindings are returned
// unchanged and the OVERSEER_STEP prompt remains as the fallback path (and for
// future multi-user scenarios). Agent-step detection reuses
// planStepExecutorKind so it stays consistent with the binding validator.
func autoBindOverseers(ctx context.Context, template *PlanTemplate, callerBindings []*plansv1.OverseerBinding, includedOptionalCapabilities []string) []*plansv1.OverseerBinding {
	if template == nil {
		return callerBindings
	}
	userID := ""
	if rc, ok := identity.RequestContextFrom(ctx); ok {
		userID = strings.TrimSpace(rc.UserID)
	}
	if userID == "" {
		return callerBindings
	}

	bound := make(map[string]bool, len(callerBindings))
	for _, ob := range callerBindings {
		if ob == nil {
			continue
		}
		if ob.GetStepKey() != "" && ob.GetOverseerUserId() != "" {
			bound[ob.GetStepKey()] = true
		}
	}

	out := make([]*plansv1.OverseerBinding, 0, len(callerBindings)+len(template.Steps))
	out = append(out, callerBindings...)
	for _, step := range template.Steps {
		stepKey := strings.TrimSpace(step.Key)
		if stepKey == "" || bound[stepKey] {
			continue
		}
		if planStepExecutorKind(step) != plansv1.ExecutorKind_EXECUTOR_KIND_AGENT {
			continue
		}
		// Opt-out capability steps run no executor and need no overseer
		// (ADR-018 D4), so skip them when auto-filling agent overseers.
		if stepOptedOut(stepToProto(&step), includedOptionalCapabilities) {
			continue
		}
		out = append(out, &plansv1.OverseerBinding{
			StepKey:        stepKey,
			OverseerUserId: userID,
		})
		bound[stepKey] = true
	}
	return out
}

func (h *PlanHandler) buildConfigurationFromRequest(
	tenantID uuid.UUID,
	template *PlanTemplate,
	workspaceID string,
	status plansv1.PlanConfigurationStatus,
	kind plansv1.PlanConfigurationKind,
	seedArtifacts []*plansv1.SeedArtifactBinding,
	slotBindings []*plansv1.SlotBinding,
	overseerBindings []*plansv1.OverseerBinding,
	behaviorPolicies *plansv1.PlanBehaviorPolicies,
	schedule *plansv1.PlanSchedule,
	parameterValuesJSON string,
) (*PlanConfiguration, error) {
	statusStr, err := configurationStatusToString(status)
	if err != nil {
		return nil, err
	}
	if statusStr == "" {
		statusStr = ConfigurationStatusDraft
	}
	kindStr, err := configurationKindToString(kind, schedule)
	if err != nil {
		return nil, err
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
	parameterValues, err := normalizeParameterValuesJSON(parameterValuesJSON)
	if err != nil {
		return nil, err
	}

	return &PlanConfiguration{
		TenantID:            tenantID,
		WorkspaceID:         workspace,
		PlanTemplateID:      template.ID,
		PlanTemplateVersion: template.Version,
		Status:              statusStr,
		Kind:                kindStr,
		SeedArtifacts:       seedJSON,
		SlotBindings:        slotJSON,
		OverseerBindings:    overseerJSON,
		BehaviorPolicies:    policiesJSON,
		Schedule:            scheduleJSON,
		ParameterValues:     parameterValues,
	}, nil
}

func (h *PlanHandler) materializeConfigurationProjection(
	ctx context.Context,
	tenantID uuid.UUID,
	template *PlanTemplate,
	parameterValuesJSON string,
) ([]*plansv1.SeedArtifactBinding, []*plansv1.SlotBinding, *plansv1.PlanBehaviorPolicies, []string, error) {
	normalized, err := normalizeParameterValuesJSON(parameterValuesJSON)
	if err != nil {
		return nil, nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var values map[string]any
	if err := json.Unmarshal(normalized, &values); err != nil {
		return nil, nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parameter values must be a JSON object: %w", err))
	}

	templateProto := templateToProto(template)
	// The included_optional_capabilities projection is derived where it is read
	// at runtime (buildPlanExecutionSnapshot), not persisted on the repository
	// row. It is threaded to ResolveDefaultSlotBindings here so opt-out steps
	// (e.g. generate-image when images are not included) are not required to
	// have a default binding, and to the binding/overseer validators so they
	// do not require a binding for opted-out steps (ADR-018 D4).
	seeds, parameterSlots, policies, included, err := MaterializePlanConfiguration(templateProto, values)
	if err != nil {
		return nil, nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	defaultSlots, err := ResolveDefaultSlotBindings(ctx, tenantID, templateProto, parameterSlots, included, h.executors)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	slotBindings := append([]*plansv1.SlotBinding{}, parameterSlots...)
	slotBindings = append(slotBindings, defaultSlots...)
	sortSlotBindings(slotBindings)
	return seeds, slotBindings, policies, included, nil
}

func sortSlotBindings(bindings []*plansv1.SlotBinding) {
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].GetStepKey() < bindings[j].GetStepKey()
	})
}

func normalizeParameterValuesJSON(parameterValuesJSON string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(parameterValuesJSON)
	if trimmed == "" {
		return json.RawMessage(`{}`), nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("parameter values must be a JSON object: %w", err)
	}
	if values == nil {
		return nil, fmt.Errorf("parameter values must be a JSON object")
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, []byte(trimmed)); err != nil {
		return nil, fmt.Errorf("compact parameter values: %w", err)
	}
	return json.RawMessage(compacted.Bytes()), nil
}

func parameterValuesForUpdate(incoming string, existing json.RawMessage) (json.RawMessage, error) {
	if strings.TrimSpace(incoming) == "" {
		return existing, nil
	}
	return normalizeParameterValuesJSON(incoming)
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
	template := &plansv1.PlanTemplate{
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
	if len(t.InputParameters) > 0 && string(t.InputParameters) != "null" {
		raw := []byte(`{"inputParameters":` + string(t.InputParameters) + `}`)
		var templateInputs plansv1.PlanTemplate
		if err := protojson.Unmarshal(raw, &templateInputs); err == nil {
			template.InputParameters = templateInputs.InputParameters
		}
	}
	return template
}

func stepToProto(s *PlanStep) *plansv1.PlanStep {
	step := &plansv1.PlanStep{
		Id:                    s.ID.String(),
		Key:                   s.Key,
		Title:                 s.Title,
		Description:           s.Description,
		InputArtifactTypeId:   s.InputArtifactTypeID,
		OutputArtifactTypeId:  s.OutputArtifactTypeID,
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

func configurationToProto(c *PlanConfiguration, templateOpt ...*PlanTemplate) *plansv1.PlanConfiguration {
	config := &plansv1.PlanConfiguration{
		Id:                  c.ID.String(),
		TenantId:            c.TenantID.String(),
		PlanTemplateId:      c.PlanTemplateID.String(),
		PlanTemplateVersion: c.PlanTemplateVersion,
		Status:              stringToConfigurationStatus(c.Status),
		Kind:                stringToConfigurationKind(c.Kind),
		CreatedAt:           c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           c.UpdatedAt.Format(time.RFC3339),
	}
	if c.WorkspaceID.Valid {
		config.WorkspaceId = c.WorkspaceID.UUID.String()
	}
	if c.OriginThreadID != uuid.Nil {
		config.OriginThreadId = c.OriginThreadID.String()
	}
	parameterValues, err := normalizeParameterValuesJSON(string(c.ParameterValues))
	if err != nil {
		parameterValues = json.RawMessage(`{}`)
	}
	config.ParameterValuesJson = string(parameterValues)

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

	// The opt-in capability set is a derived runtime projection (ADR-018 D4):
	// it is not persisted on the row but read by the configuration assistant
	// and binding validators to tell opted-in optional steps from opted-out
	// ones. Populate it when the caller has the template in hand; callers
	// without a template get an empty set (the safe "opt out everything
	// optional" default).
	applyIncludedOptionalCapabilities(config, templateOpt)

	return config
}

// applyIncludedOptionalCapabilities populates the opt-in capability projection
// on a proto configuration when the caller supplied the owning template.
func applyIncludedOptionalCapabilities(config *plansv1.PlanConfiguration, templateOpt []*PlanTemplate) {
	if len(templateOpt) == 0 || templateOpt[0] == nil {
		return
	}
	populateIncludedOptionalCapabilities(config, templateToProto(templateOpt[0]))
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
		var runtimeSnapshot workflow.PlanExecutionSnapshot
		if err := json.Unmarshal(e.PlanConfigurationSnapshot, &runtimeSnapshot); err == nil && runtimeSnapshot.Configuration != nil {
			execution.PlanConfigurationSnapshot = runtimeSnapshot.Configuration
		} else {
			var snapshot plansv1.PlanConfiguration
			if err := json.Unmarshal(e.PlanConfigurationSnapshot, &snapshot); err == nil {
				execution.PlanConfigurationSnapshot = &snapshot
			}
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

func configurationThreadID(config *PlanConfiguration) uuid.UUID {
	if config == nil {
		return uuid.Nil
	}
	if config.OriginThreadID != uuid.Nil {
		return config.OriginThreadID
	}
	return uuid.Nil
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

func configurationKindToString(kind plansv1.PlanConfigurationKind, schedule *plansv1.PlanSchedule) (string, error) {
	switch kind {
	case plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED:
		if schedule != nil && strings.TrimSpace(schedule.CronExpression) != "" {
			return ConfigurationKindRecurring, nil
		}
		return ConfigurationKindOneShot, nil
	case plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_ONE_SHOT:
		return ConfigurationKindOneShot, nil
	case plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_RECURRING:
		return ConfigurationKindRecurring, nil
	default:
		return "", fmt.Errorf("unsupported plan configuration kind %s", kind.String())
	}
}

func stringToConfigurationKind(s string) plansv1.PlanConfigurationKind {
	switch s {
	case ConfigurationKindOneShot:
		return plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_ONE_SHOT
	case ConfigurationKindRecurring:
		return plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_RECURRING
	default:
		return plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED
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
