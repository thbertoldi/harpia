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
	executorpkg "github.com/harpia/control-plane/internal/executors"
)

var (
	ErrBindingValidation = errors.New("slot binding validation failed")
)

type ExecutorLookup interface {
	GetInstallationByID(ctx context.Context, tenantID, id uuid.UUID) (*executorpkg.ExecutorInstallation, error)
	GetSKUByID(ctx context.Context, id uuid.UUID) (*executorpkg.ExecutorSKU, error)
	ListEntitlements(ctx context.Context, tenantID uuid.UUID, skuID *uuid.UUID, limit, offset int) ([]executorpkg.ExecutorEntitlement, error)
	ListCompatibleInstallationsForStep(ctx context.Context, tenantID uuid.UUID, step *plansv1.PlanStep) ([]executorpkg.ExecutorInstallation, error)
}

type BindingValidationError struct {
	StepKey                string
	ExecutorSKUID          string
	ExecutorSKUKey         string
	ExecutorInstallationID string
	Reason                 string
	Code                   connect.Code
}

func (e *BindingValidationError) Error() string {
	details := make([]string, 0, 4)
	if e.StepKey != "" {
		details = append(details, fmt.Sprintf("step_key=%q", e.StepKey))
	}
	if e.ExecutorSKUKey != "" {
		details = append(details, fmt.Sprintf("sku_key=%q", e.ExecutorSKUKey))
	}
	if e.ExecutorSKUID != "" {
		details = append(details, fmt.Sprintf("sku_id=%s", e.ExecutorSKUID))
	}
	if e.ExecutorInstallationID != "" {
		details = append(details, fmt.Sprintf("installation_id=%s", e.ExecutorInstallationID))
	}
	prefix := "slot binding invalid"
	if len(details) > 0 {
		prefix = fmt.Sprintf("%s (%s)", prefix, strings.Join(details, ", "))
	}
	return fmt.Sprintf("%s: %s", prefix, e.Reason)
}

func (e *BindingValidationError) Unwrap() error {
	return ErrBindingValidation
}

type BindingValidator struct {
	executorLookup   ExecutorLookup
	configValidators *executorpkg.ConfigValidatorRegistry
}

func NewBindingValidator(lookup ExecutorLookup, configValidators ...*executorpkg.ConfigValidatorRegistry) *BindingValidator {
	registry := executorpkg.DefaultConfigValidators()
	if len(configValidators) > 0 && configValidators[0] != nil {
		registry = configValidators[0]
	}
	return &BindingValidator{executorLookup: lookup, configValidators: registry}
}

func (v *BindingValidator) ValidateSlotBindings(
	ctx context.Context,
	tenantID uuid.UUID,
	template *PlanTemplate,
	status plansv1.PlanConfigurationStatus,
	slotBindings []*plansv1.SlotBinding,
	includedOptionalCapabilities []string,
) error {
	if v == nil || v.executorLookup == nil {
		return errors.New("plans: binding validator is required")
	}
	if template == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("plan template is required"))
	}

	statusStr, err := configurationStatusToString(status)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	if statusStr == "" {
		statusStr = ConfigurationStatusDraft
	}

	strict := statusStr == ConfigurationStatusRunnable || statusStr == ConfigurationStatusScheduled
	stepKeys := templateStepKeys(template)
	bindingsByStep := make(map[string]*plansv1.SlotBinding, len(slotBindings))

	for _, binding := range slotBindings {
		if binding == nil {
			return bindingError("", "", "", connect.CodeInvalidArgument, "slot binding entry is required")
		}
		stepKey := strings.TrimSpace(binding.StepKey)
		if stepKey == "" {
			return bindingError("", binding.ExecutorSkuId, binding.ExecutorInstallationId, connect.CodeInvalidArgument, "step_key is required")
		}
		if _, ok := stepKeys[stepKey]; !ok {
			return bindingError(stepKey, binding.ExecutorSkuId, binding.ExecutorInstallationId, connect.CodeInvalidArgument, "step_key does not exist in plan template")
		}
		if existing, ok := bindingsByStep[stepKey]; ok {
			return bindingError(stepKey, existing.ExecutorSkuId, existing.ExecutorInstallationId, connect.CodeInvalidArgument, "duplicate slot binding for step")
		}
		bindingsByStep[stepKey] = binding
	}

	if strict {
		for _, step := range template.Steps {
			stepKey := strings.TrimSpace(step.Key)
			if stepKey == "" {
				continue
			}
			// Opt-out capability steps run no executor and need no binding
			// (ADR-018 D4), so skip them when requiring runnable readiness.
			if stepOptedOut(stepToProto(&step), includedOptionalCapabilities) {
				continue
			}
			binding, ok := bindingsByStep[stepKey]
			if !ok {
				return bindingError(stepKey, "", "", connect.CodeFailedPrecondition, "slot binding is required for runnable or scheduled configuration")
			}
			if err := v.validateBinding(ctx, tenantID, binding, true); err != nil {
				return err
			}
		}
		return nil
	}

	for _, binding := range bindingsByStep {
		if err := v.validateBinding(ctx, tenantID, binding, false); err != nil {
			return err
		}
	}
	return nil
}

func (v *BindingValidator) ValidateOverseerBindings(
	template *PlanTemplate,
	status plansv1.PlanConfigurationStatus,
	overseerBindings []*plansv1.OverseerBinding,
	includedOptionalCapabilities []string,
) error {
	if template == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("plan template is required"))
	}

	statusStr, err := configurationStatusToString(status)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	if statusStr == "" {
		statusStr = ConfigurationStatusDraft
	}

	strict := statusStr == ConfigurationStatusRunnable || statusStr == ConfigurationStatusScheduled
	stepKeys := templateStepKeys(template)
	bindingsByStep := make(map[string]*plansv1.OverseerBinding, len(overseerBindings))

	for _, binding := range overseerBindings {
		if binding == nil {
			return bindingError("", "", "", connect.CodeInvalidArgument, "overseer binding entry is required")
		}
		stepKey := strings.TrimSpace(binding.StepKey)
		if stepKey == "" {
			return bindingError("", "", "", connect.CodeInvalidArgument, "step_key is required")
		}
		if _, ok := stepKeys[stepKey]; !ok {
			return bindingError(stepKey, "", "", connect.CodeInvalidArgument, "step_key does not exist in plan template")
		}
		if existing, ok := bindingsByStep[stepKey]; ok {
			return bindingError(existing.StepKey, "", "", connect.CodeInvalidArgument, "duplicate overseer binding for step")
		}
		if strings.TrimSpace(binding.OverseerUserId) == "" {
			return bindingError(stepKey, "", "", connect.CodeInvalidArgument, "overseer_user_id is required")
		}
		bindingsByStep[stepKey] = binding
	}

	if !strict {
		return nil
	}

	for _, step := range template.Steps {
		stepKey := strings.TrimSpace(step.Key)
		if stepKey == "" || planStepExecutorKind(step) != plansv1.ExecutorKind_EXECUTOR_KIND_AGENT {
			continue
		}
		// Opt-out capability steps run no executor and need no overseer
		// (ADR-018 D4), so skip them when requiring runnable readiness.
		if stepOptedOut(stepToProto(&step), includedOptionalCapabilities) {
			continue
		}
		if _, ok := bindingsByStep[stepKey]; !ok {
			return bindingError(stepKey, "", "", connect.CodeFailedPrecondition, "overseer binding is required for runnable or scheduled configuration")
		}
	}
	return nil
}

func (v *BindingValidator) ValidateConfigurationForExecution(
	ctx context.Context,
	tenantID uuid.UUID,
	template *PlanTemplate,
	config *PlanConfiguration,
) error {
	if config == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("plan configuration is required"))
	}

	var slotBindings []*plansv1.SlotBinding
	if len(config.SlotBindings) > 0 {
		if err := json.Unmarshal(config.SlotBindings, &slotBindings); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid stored slot bindings: %w", err))
		}
	}
	var seedArtifacts []*plansv1.SeedArtifactBinding
	if len(config.SeedArtifacts) > 0 {
		if err := json.Unmarshal(config.SeedArtifacts, &seedArtifacts); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid stored seed artifacts: %w", err))
		}
	}
	// Derive the opt-in capability set BEFORE the root-seed readiness check so
	// an opted-out root step (no upstream edges, has an input artifact type)
	// is not required to carry a seed artifact it will never consume. This
	// mirrors the opt-out skip already applied in ValidateSlotBindings and
	// ValidateOverseerBindings (ADR-018 D4).
	included := includedOptionalCapabilitiesForConfig(template, config)
	if err := validateRootSeedReadiness(template, seedArtifacts, included); err != nil {
		return err
	}

	return v.ValidateSlotBindings(ctx, tenantID, template, plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, slotBindings, included)
}

// includedOptionalCapabilitiesForConfig derives the opt-in capability set from
// a stored configuration's parameter values + template (ADR-018 D4). The set
// drives which optional steps need a binding/overseer: a capability not in the
// set means the step is opted out and skipped. Parse or materialize failures
// yield an empty set — the safe "opt out everything optional" default for an
// unconfigured run.
func includedOptionalCapabilitiesForConfig(template *PlanTemplate, config *PlanConfiguration) []string {
	if template == nil || config == nil {
		return nil
	}
	values := map[string]any{}
	if trimmed := strings.TrimSpace(string(config.ParameterValues)); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil
		}
	}
	_, _, _, included, err := MaterializePlanConfiguration(templateToProto(template), values)
	if err != nil {
		return nil
	}
	return included
}

func validateRootSeedReadiness(template *PlanTemplate, seedArtifacts []*plansv1.SeedArtifactBinding, includedOptionalCapabilities []string) error {
	if template == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("plan template is required"))
	}
	hasUpstream := make(map[string]bool, len(template.Steps))
	for _, edge := range template.Edges {
		to := strings.TrimSpace(edge.ToStepKey)
		if to != "" {
			hasUpstream[to] = true
		}
	}
	seedsByStep := make(map[string][]*plansv1.SeedArtifactBinding, len(seedArtifacts))
	for _, seed := range seedArtifacts {
		if seed == nil {
			continue
		}
		stepKey := strings.TrimSpace(seed.GetStepKey())
		if stepKey == "" {
			continue
		}
		seedsByStep[stepKey] = append(seedsByStep[stepKey], seed)
	}
	for _, step := range template.Steps {
		stepKey := strings.TrimSpace(step.Key)
		inputType := strings.TrimSpace(step.InputArtifactTypeID)
		if stepKey == "" || inputType == "" || hasUpstream[stepKey] {
			continue
		}
		// Opt-out capability steps run no executor and consume no seed artifact
		// (ADR-018 D4), so skip them when requiring root-seed readiness.
		if stepOptedOut(stepToProto(&step), includedOptionalCapabilities) {
			continue
		}
		if !hasSeedForRootInput(seedsByStep[stepKey], inputType) {
			return bindingError(stepKey, "", "", connect.CodeFailedPrecondition, fmt.Sprintf("seed artifact is required for input artifact type %q", inputType))
		}
	}
	return nil
}

func hasSeedForRootInput(seeds []*plansv1.SeedArtifactBinding, inputType string) bool {
	for _, seed := range seeds {
		if seed == nil {
			continue
		}
		if strings.TrimSpace(seed.GetArtifactId()) == "" && strings.TrimSpace(seed.GetLiteralJson()) == "" {
			continue
		}
		inputName := strings.TrimSpace(seed.GetInputName())
		if strings.HasPrefix(inputName, "harpia.internal.") {
			continue
		}
		if inputName == "" || inputName == inputType {
			return true
		}
		return true
	}
	return false
}

func planStepExecutorKind(step PlanStep) plansv1.ExecutorKind {
	if len(step.ExecutorRequirement) == 0 {
		return plansv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED
	}
	var raw struct {
		ExecutorKind any `json:"executor_kind"`
	}
	if err := json.Unmarshal(step.ExecutorRequirement, &raw); err != nil {
		return plansv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED
	}
	switch v := raw.ExecutorKind.(type) {
	case float64:
		return plansv1.ExecutorKind(int32(v))
	case string:
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "AGENT", "EXECUTOR_KIND_AGENT":
			return plansv1.ExecutorKind_EXECUTOR_KIND_AGENT
		case "INTEGRATION", "EXECUTOR_KIND_INTEGRATION":
			return plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION
		}
	}
	return plansv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED
}

func (v *BindingValidator) validateBinding(
	ctx context.Context,
	tenantID uuid.UUID,
	binding *plansv1.SlotBinding,
	requireReadiness bool,
) error {
	stepKey := strings.TrimSpace(binding.StepKey)
	installationID := strings.TrimSpace(binding.ExecutorInstallationId)
	skuID := strings.TrimSpace(binding.ExecutorSkuId)

	if installationID == "" {
		if requireReadiness {
			return bindingError(stepKey, skuID, "", connect.CodeFailedPrecondition, "executor_installation_id is required")
		}
		if skuID != "" {
			if _, err := uuid.Parse(skuID); err != nil {
				return bindingError(stepKey, skuID, "", connect.CodeInvalidArgument, "executor_sku_id must be a valid UUID")
			}
		}
		return nil
	}

	parsedInstallationID, err := uuid.Parse(installationID)
	if err != nil {
		return bindingError(stepKey, skuID, installationID, connect.CodeInvalidArgument, "executor_installation_id must be a valid UUID")
	}

	installation, err := v.executorLookup.GetInstallationByID(ctx, tenantID, parsedInstallationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bindingError(stepKey, skuID, installationID, connect.CodeFailedPrecondition, "executor installation not found for tenant")
		}
		return connect.NewError(connect.CodeInternal, fmt.Errorf("lookup executor installation: %w", err))
	}
	if installation.TenantID != tenantID {
		return bindingError(stepKey, skuID, installationID, connect.CodeFailedPrecondition, "executor installation does not belong to tenant")
	}

	sku, err := v.executorLookup.GetSKUByID(ctx, installation.ExecutorSKUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bindingError(stepKey, installation.ExecutorSKUID.String(), installationID, connect.CodeFailedPrecondition, "executor sku not found for installation")
		}
		return connect.NewError(connect.CodeInternal, fmt.Errorf("lookup executor sku: %w", err))
	}

	if skuID != "" {
		parsedSKUID, parseErr := uuid.Parse(skuID)
		if parseErr != nil {
			return bindingError(stepKey, skuID, installationID, connect.CodeInvalidArgument, "executor_sku_id must be a valid UUID")
		}
		if parsedSKUID != installation.ExecutorSKUID {
			return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeFailedPrecondition, "executor_sku_id does not match installation")
		}
	}

	if binding.ExecutorKind != plansv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED {
		expectedKind, kindErr := plansExecutorKindToDB(binding.ExecutorKind)
		if kindErr != nil {
			return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeInvalidArgument, kindErr.Error())
		}
		if installation.Kind != expectedKind {
			return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeFailedPrecondition, fmt.Sprintf("executor kind mismatch: installation is %q", installation.Kind))
		}
	}

	entitlements, err := v.executorLookup.ListEntitlements(ctx, tenantID, &installation.ExecutorSKUID, 1, 0)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("lookup executor entitlement: %w", err))
	}
	if len(entitlements) == 0 {
		return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeFailedPrecondition, "tenant is not entitled to executor sku")
	}

	if requireReadiness {
		if !installation.Enabled {
			return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeFailedPrecondition, "executor installation is disabled")
		}
		switch installation.Kind {
		case executorpkg.KindIntegration:
			if err := v.validateIntegrationReadiness(installation, sku.Key); err != nil {
				return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeFailedPrecondition, err.Error())
			}
		case executorpkg.KindAgent:
			if installation.ManifestID == nil || strings.TrimSpace(*installation.ManifestID) == "" ||
				installation.ManifestVersion == nil || strings.TrimSpace(*installation.ManifestVersion) == "" {
				return bindingErrorWithSKU(stepKey, sku, installationID, connect.CodeFailedPrecondition, "agent installation requires manifest_id and manifest_version")
			}
		}
	}

	return nil
}

func (v *BindingValidator) validateIntegrationReadiness(installation *executorpkg.ExecutorInstallation, skuKey string) error {
	status := "disconnected"
	if installation.ConnectionStatus != nil {
		status = strings.TrimSpace(*installation.ConnectionStatus)
	}
	if status != "connected" {
		return fmt.Errorf("integration installation is not connected (status=%q)", status)
	}

	config := strings.TrimSpace(string(installation.ConfigJSON))
	if config == "" || config == "{}" || config == "null" {
		return errors.New("integration installation is not configured")
	}
	if !json.Valid(installation.ConfigJSON) {
		return errors.New("integration installation config_json is invalid")
	}
	return v.configValidators.Validate(skuKey, installation.ConfigJSON)
}

func templateStepKeys(template *PlanTemplate) map[string]struct{} {
	keys := make(map[string]struct{}, len(template.Steps))
	for i := range template.Steps {
		keys[template.Steps[i].Key] = struct{}{}
	}
	return keys
}

func plansExecutorKindToDB(kind plansv1.ExecutorKind) (string, error) {
	switch kind {
	case plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION:
		return executorpkg.KindIntegration, nil
	case plansv1.ExecutorKind_EXECUTOR_KIND_AGENT:
		return executorpkg.KindAgent, nil
	default:
		return "", fmt.Errorf("unsupported executor kind %s", kind.String())
	}
}

func bindingError(stepKey, skuID, installationID string, code connect.Code, reason string) error {
	return &BindingValidationError{
		StepKey:                stepKey,
		ExecutorSKUID:          skuID,
		ExecutorInstallationID: installationID,
		Reason:                 reason,
		Code:                   code,
	}
}

func bindingErrorWithSKU(stepKey string, sku *executorpkg.ExecutorSKU, installationID string, code connect.Code, reason string) error {
	return &BindingValidationError{
		StepKey:                stepKey,
		ExecutorSKUID:          sku.ID.String(),
		ExecutorSKUKey:         sku.Key,
		ExecutorInstallationID: installationID,
		Reason:                 reason,
		Code:                   code,
	}
}

func connectErrorFromBinding(err error) error {
	if err == nil {
		return nil
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connectErr
	}
	var bindingErr *BindingValidationError
	if errors.As(err, &bindingErr) {
		code := bindingErr.Code
		if code == 0 {
			code = connect.CodeInvalidArgument
		}
		return connect.NewError(code, bindingErr)
	}
	return connect.NewError(connect.CodeInternal, err)
}
