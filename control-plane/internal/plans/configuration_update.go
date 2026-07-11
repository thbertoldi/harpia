package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/database"
)

// applyConfigurationUpdate is the single mutation path for a plan configuration
// projection. It runs entirely against the provided Querier — typically a tx the
// caller opened via database.WithTenant — so the upcoming selection-turn flow
// can compose it into a larger transaction alongside its own writes.
//
// The helper loads the existing configuration (to preserve identity/lineage
// fields the projection does not own), materializes the declarative projection
// from the parameter values, validates slot + overseer bindings against the
// requested status, persists, and returns the updated configuration. Schedule
// sync (Temporal), chat side-effects, and assistant advancement remain the
// caller's concern — they live in the RPC handler.
//
// All failures are returned as connect-coded errors so the RPC handler can
// propagate them verbatim.
func (h *PlanHandler) applyConfigurationUpdate(
	ctx context.Context,
	q database.Querier,
	tenantID uuid.UUID,
	configID uuid.UUID,
	template *PlanTemplate,
	nextParameterValuesJSON string,
	nextOverseerBindings []*plansv1.OverseerBinding,
	nextStatus plansv1.PlanConfigurationStatus,
	nextKind plansv1.PlanConfigurationKind,
	nextSchedule *plansv1.PlanSchedule,
) (*PlanConfiguration, error) {
	if q == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("plans: querier is required"))
	}
	if template == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("plan template is required"))
	}

	var existing PlanConfiguration
	if err := getConfigurationQ(ctx, q, tenantID, configID, &existing); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("plan configuration not found: %w", err))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load existing plan configuration: %w", err))
	}

	parameterValues, err := parameterValuesForUpdate(nextParameterValuesJSON, existing.ParameterValues)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	seedArtifacts, slotBindings, behaviorPolicies, included, err := h.materializeConfigurationProjection(ctx, tenantID, template, string(parameterValues))
	if err != nil {
		return nil, err
	}

	if err := h.validator.ValidateSlotBindings(ctx, tenantID, template, nextStatus, slotBindings, included); err != nil {
		return nil, connectErrorFromBinding(err)
	}
	if err := h.validator.ValidateOverseerBindings(template, nextStatus, nextOverseerBindings, included); err != nil {
		return nil, connectErrorFromBinding(err)
	}

	// Kind follows the same precedence as the legacy RPC: an explicit caller
	// value wins; UNSPECIFIED falls back to the stored kind so an update that
	// does not touch kind preserves recurring/one-shot.
	kind := nextKind
	if kind == plansv1.PlanConfigurationKind_PLAN_CONFIGURATION_KIND_UNSPECIFIED {
		kind = stringToConfigurationKind(existing.Kind)
	}

	workspaceID := ""
	if existing.WorkspaceID.Valid {
		workspaceID = existing.WorkspaceID.UUID.String()
	}
	config, err := h.buildConfigurationFromRequest(
		tenantID, template, workspaceID, nextStatus, kind,
		seedArtifacts, slotBindings, nextOverseerBindings,
		behaviorPolicies, nextSchedule, string(parameterValues),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Preserve identity/lineage fields the projection does not own.
	config.ID = existing.ID
	config.PlanTemplateID = existing.PlanTemplateID
	config.PlanTemplateVersion = existing.PlanTemplateVersion
	config.WorkspaceID = existing.WorkspaceID
	config.ParameterValues = parameterValues
	config.OriginThreadID = existing.OriginThreadID

	var updated PlanConfiguration
	if err := updateConfigurationQ(ctx, q, config, &updated); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update plan configuration: %w", err))
	}
	return &updated, nil
}

// MergeParameterValue merges a single parameter value into the parameter_values
// JSON blob and returns the compacted result. An empty current blob is treated
// as {}. value may be any JSON-serializable Go value (string, number, bool,
// map, slice). A corrupt current blob is overwritten defensively rather than
// silently dropping the caller's prior values.
func MergeParameterValue(currentJSON, key string, value any) string {
	key = strings.TrimSpace(key)
	values := map[string]any{}
	if trimmed := strings.TrimSpace(currentJSON); trimmed != "" {
		var existing map[string]any
		if err := json.Unmarshal([]byte(trimmed), &existing); err == nil && existing != nil {
			values = existing
		}
	}
	if key != "" {
		values[key] = value
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return currentJSON
	}
	return string(raw)
}

// FindSlotParameterForStep returns the key of the input parameter whose runtime
// mapping is target=SLOT_BINDING for the given stepKey. It returns ("", false)
// when no such parameter exists. The lookup walks the template's declared
// InputParameters, so it works against any PlanTemplate loaded from the catalog.
func FindSlotParameterForStep(template *PlanTemplate, stepKey string) (string, bool) {
	if template == nil {
		return "", false
	}
	stepKey = strings.TrimSpace(stepKey)
	if stepKey == "" {
		return "", false
	}
	for _, parameter := range templateToProto(template).GetInputParameters() {
		if parameter == nil {
			continue
		}
		for _, mapping := range parameter.GetRuntimeMappings() {
			if mapping == nil {
				continue
			}
			if mapping.GetTarget() == plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_SLOT_BINDING &&
				strings.TrimSpace(mapping.GetStepKey()) == stepKey {
				return parameter.GetKey(), true
			}
		}
	}
	return "", false
}

// FindPolicyParameter returns the key of the input parameter whose runtime
// mapping is target=BEHAVIOR_POLICY for the given policyKey (e.g.
// "publish_approval_mode", "elicitation_timeout_behavior"). It returns
// ("", false) when no such parameter exists.
func FindPolicyParameter(template *PlanTemplate, policyKey string) (string, bool) {
	if template == nil {
		return "", false
	}
	policyKey = strings.TrimSpace(policyKey)
	if policyKey == "" {
		return "", false
	}
	for _, parameter := range templateToProto(template).GetInputParameters() {
		if parameter == nil {
			continue
		}
		for _, mapping := range parameter.GetRuntimeMappings() {
			if mapping == nil {
				continue
			}
			if mapping.GetTarget() == plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY &&
				strings.TrimSpace(mapping.GetPolicyKey()) == policyKey {
				return parameter.GetKey(), true
			}
		}
	}
	return "", false
}

// MergeOverseerBinding upserts a single overseer binding for stepKey into the
// current slice. If an entry for stepKey already exists it is replaced;
// otherwise a new entry is appended. The returned slice never aliases the
// input — the caller can retain and keep mutating the original. Updated and
// inserted entries are freshly allocated (proto messages embed a mutex, so we
// never copy them by value).
func MergeOverseerBinding(current []*plansv1.OverseerBinding, stepKey, overseerUserID string) []*plansv1.OverseerBinding {
	stepKey = strings.TrimSpace(stepKey)
	out := make([]*plansv1.OverseerBinding, 0, len(current)+1)
	merged := false
	for _, binding := range current {
		if binding != nil && strings.TrimSpace(binding.GetStepKey()) == stepKey {
			// Preserve any future fields by copying known values explicitly.
			out = append(out, &plansv1.OverseerBinding{
				StepKey:        binding.GetStepKey(),
				OverseerUserId: overseerUserID,
			})
			merged = true
			continue
		}
		out = append(out, binding)
	}
	if !merged && stepKey != "" {
		out = append(out, &plansv1.OverseerBinding{
			StepKey:        stepKey,
			OverseerUserId: overseerUserID,
		})
	}
	return out
}

// PromoteStatus advances a configuration status out of DRAFT as part of a
// selection turn / matrix save. A non-empty cron schedule yields SCHEDULED;
// otherwise RUNNABLE. Non-DRAFT statuses are returned unchanged — this helper
// never demotes (e.g. RUNNABLE→DRAFT) nor collapses SCHEDULED back to RUNNABLE.
func PromoteStatus(current string, schedule *plansv1.PlanSchedule) string {
	if current != ConfigurationStatusDraft {
		return current
	}
	if schedule != nil && strings.TrimSpace(schedule.GetCronExpression()) != "" {
		return ConfigurationStatusScheduled
	}
	return ConfigurationStatusRunnable
}
