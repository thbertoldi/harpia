package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/workflow"
	"google.golang.org/protobuf/encoding/protojson"
)

const executionSnapshotSchemaVersion = 1

func buildPlanExecutionSnapshot(
	ctx context.Context,
	tenantID uuid.UUID,
	config *PlanConfiguration,
	template *PlanTemplate,
	executorLookup ExecutorLookup,
	now time.Time,
) (workflow.PlanExecutionSnapshot, error) {
	if config == nil {
		return workflow.PlanExecutionSnapshot{}, fmt.Errorf("plan configuration is required")
	}
	if template == nil {
		return workflow.PlanExecutionSnapshot{}, fmt.Errorf("plan template is required")
	}
	if executorLookup == nil {
		return workflow.PlanExecutionSnapshot{}, fmt.Errorf("executor lookup is required")
	}

	var slotBindings []*plansv1.SlotBinding
	if len(config.SlotBindings) > 0 {
		if err := json.Unmarshal(config.SlotBindings, &slotBindings); err != nil {
			return workflow.PlanExecutionSnapshot{}, fmt.Errorf("parse slot bindings: %w", err)
		}
	}

	bindingsByStep := make(map[string]*plansv1.SlotBinding, len(slotBindings))
	for _, binding := range slotBindings {
		if binding == nil {
			continue
		}
		stepKey := strings.TrimSpace(binding.StepKey)
		if stepKey == "" {
			continue
		}
		bindingsByStep[stepKey] = binding
	}

	installationSnapshots := make(map[string]workflow.ExecutorInstallationSnapshot, len(template.Steps))
	for _, step := range template.Steps {
		binding := bindingsByStep[step.Key]
		if binding == nil {
			return workflow.PlanExecutionSnapshot{}, fmt.Errorf("missing slot binding for step %q", step.Key)
		}
		installationID, err := uuid.Parse(binding.ExecutorInstallationId)
		if err != nil {
			return workflow.PlanExecutionSnapshot{}, fmt.Errorf("parse executor installation for step %q: %w", step.Key, err)
		}
		installation, err := executorLookup.GetInstallationByID(ctx, tenantID, installationID)
		if err != nil {
			return workflow.PlanExecutionSnapshot{}, fmt.Errorf("load executor installation for step %q: %w", step.Key, err)
		}
		sku, err := executorLookup.GetSKUByID(ctx, installation.ExecutorSKUID)
		if err != nil {
			return workflow.PlanExecutionSnapshot{}, fmt.Errorf("load executor sku for step %q: %w", step.Key, err)
		}
		installationSnapshots[step.Key] = executorInstallationToSnapshot(installation, sku)
	}

	configuration := configurationToProto(config)
	resolveDateRangePresetSeeds(configuration, configuration.GetSchedule(), now)
	// Opt-in capabilities are derived from the persisted parameter values rather
	// than stored on the row, so the runtime snapshot is the single place that
	// populates PlanConfiguration.included_optional_capabilities for the engine.
	populateIncludedOptionalCapabilities(configuration, templateToProto(template))

	return workflow.PlanExecutionSnapshot{
		SchemaVersion:         executionSnapshotSchemaVersion,
		Configuration:         configuration,
		Template:              templateToProto(template),
		ExecutorInstallations: installationSnapshots,
		SnapshotAt:            now.UTC().Format(time.RFC3339),
	}, nil
}

func resolveDateRangePresetSeeds(config *plansv1.PlanConfiguration, schedule *plansv1.PlanSchedule, now time.Time) {
	if config == nil {
		return
	}
	for _, seed := range config.GetSeedArtifacts() {
		if seed == nil {
			continue
		}
		if strings.TrimSpace(seed.GetLiteralJson()) == "" {
			continue
		}
		var payload struct {
			Preset string `json:"preset"`
		}
		if err := json.Unmarshal([]byte(seed.GetLiteralJson()), &payload); err != nil {
			continue
		}
		if strings.TrimSpace(payload.Preset) == "" {
			continue
		}
		dateRange, ok := ResolveDateRangePresetForSchedule(payload.Preset, schedule, now)
		if !ok {
			continue
		}
		raw, err := protojson.MarshalOptions{UseProtoNames: false}.Marshal(dateRange)
		if err != nil {
			continue
		}
		seed.LiteralJson = string(raw)
	}
}

// populateIncludedOptionalCapabilities derives the opt-in capability set from
// the configuration's parameter values (e.g. include_carousel/include_images
// SELECTs) and sets it on the runtime PlanConfiguration. The engine skips
// steps whose optional capability is not in this set (ADR-018 D4). Parse or
// materialize failures are non-fatal — an empty set means "opt out everything
// optional", which is the safe default for an unconfigured run.
func populateIncludedOptionalCapabilities(config *plansv1.PlanConfiguration, template *plansv1.PlanTemplate) {
	if config == nil || template == nil {
		return
	}
	values := map[string]any{}
	if trimmed := strings.TrimSpace(string(config.GetParameterValuesJson())); trimmed != "" {
		_ = json.Unmarshal([]byte(trimmed), &values)
	}
	if _, _, _, included, err := MaterializePlanConfiguration(template, values); err == nil {
		config.IncludedOptionalCapabilities = included
	}
}

func marshalPlanExecutionSnapshot(snapshot workflow.PlanExecutionSnapshot) (json.RawMessage, error) {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal plan execution snapshot: %w", err)
	}
	return raw, nil
}

func unmarshalPlanExecutionSnapshot(raw json.RawMessage) (workflow.PlanExecutionSnapshot, error) {
	var snapshot workflow.PlanExecutionSnapshot
	if len(raw) == 0 {
		return snapshot, fmt.Errorf("plan execution snapshot is empty")
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, fmt.Errorf("parse plan execution snapshot: %w", err)
	}
	if snapshot.SchemaVersion != executionSnapshotSchemaVersion {
		return snapshot, fmt.Errorf("unsupported plan execution snapshot schema_version %d", snapshot.SchemaVersion)
	}
	if snapshot.Configuration == nil || snapshot.Template == nil {
		return snapshot, fmt.Errorf("plan execution snapshot is missing runnable plan graph")
	}
	return snapshot, nil
}

func executorInstallationToSnapshot(installation *executors.ExecutorInstallation, sku *executors.ExecutorSKU) workflow.ExecutorInstallationSnapshot {
	if installation == nil {
		return workflow.ExecutorInstallationSnapshot{}
	}

	snapshot := workflow.ExecutorInstallationSnapshot{
		ID:            installation.ID.String(),
		TenantID:      installation.TenantID.String(),
		ExecutorSKUID: installation.ExecutorSKUID.String(),
		Kind:          installation.Kind,
		DisplayName:   installation.DisplayName,
		Enabled:       installation.Enabled,
		CreatedAt:     installation.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     installation.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if sku != nil {
		snapshot.ExecutorSKUKey = sku.Key
	}
	if installation.ConnectionStatus != nil {
		snapshot.ConnectionStatus = *installation.ConnectionStatus
	}
	if len(installation.ConfigJSON) > 0 {
		snapshot.ConfigJSON = string(installation.ConfigJSON)
	}
	if installation.ManifestID != nil {
		snapshot.ManifestID = *installation.ManifestID
	}
	if installation.ManifestVersion != nil {
		snapshot.ManifestVersion = *installation.ManifestVersion
	}
	return snapshot
}
