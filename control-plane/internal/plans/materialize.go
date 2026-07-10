package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
)

type seedTarget struct {
	stepKey   string
	inputName string
}

type seedBuilder struct {
	wholeSet bool
	whole    any
	object   map[string]any
}

// MaterializePlanConfiguration expands declarative template runtime mappings
// into the server-owned configuration projection used at execution time. The
// returned includedOptionalCapabilities is the set of opt-in capabilities the
// user enabled (INCLUDED_CAPABILITY target) — callers set it on the runtime
// PlanConfiguration so the engine can skip steps whose optional capability is
// not in the set (ADR-018 D4).
func MaterializePlanConfiguration(template *plansv1.PlanTemplate, values map[string]any) ([]*plansv1.SeedArtifactBinding, []*plansv1.SlotBinding, *plansv1.PlanBehaviorPolicies, []string, error) {
	if template == nil {
		return nil, nil, nil, nil, fmt.Errorf("plan template is required")
	}
	if values == nil {
		values = map[string]any{}
	}

	seedGroups := map[seedTarget]*seedBuilder{}
	var slots []*plansv1.SlotBinding
	policies := &plansv1.PlanBehaviorPolicies{}
	included := []string{}
	includedSet := map[string]struct{}{}

	addIncluded := func(capability string) {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			return
		}
		if _, ok := includedSet[capability]; ok {
			return
		}
		includedSet[capability] = struct{}{}
		included = append(included, capability)
	}

	for _, parameter := range template.GetInputParameters() {
		if parameter == nil {
			continue
		}
		value, ok := values[parameter.GetKey()]
		if !ok {
			continue
		}
		for _, mapping := range parameter.GetRuntimeMappings() {
			if mapping == nil {
				continue
			}
			switch mapping.GetTarget() {
			case plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT:
				target := seedTarget{
					stepKey:   strings.TrimSpace(mapping.GetStepKey()),
					inputName: strings.TrimSpace(mapping.GetInputName()),
				}
				if target.stepKey == "" || target.inputName == "" {
					continue
				}
				builder := seedGroups[target]
				if builder == nil {
					builder = &seedBuilder{object: map[string]any{}}
					seedGroups[target] = builder
				}
				if err := builder.set(mapping.GetJsonPath(), value); err != nil {
					return nil, nil, nil, nil, fmt.Errorf("materialize parameter %q: %w", parameter.GetKey(), err)
				}
			case plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_SLOT_BINDING:
				installationID := strings.TrimSpace(fmt.Sprint(value))
				stepKey := strings.TrimSpace(mapping.GetStepKey())
				if stepKey == "" || installationID == "" {
					continue
				}
				slots = append(slots, &plansv1.SlotBinding{
					StepKey:                stepKey,
					ExecutorInstallationId: installationID,
				})
			case plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY:
				if err := applyBehaviorPolicy(policies, mapping.GetPolicyKey(), value); err != nil {
					return nil, nil, nil, nil, fmt.Errorf("materialize parameter %q: %w", parameter.GetKey(), err)
				}
			case plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY:
				// A truthy value opts the capability (policy_key) INTO the run.
				// Falsy values are a no-op: the capability stays opted out.
				if isTruthyCapabilityValue(value) {
					addIncluded(mapping.GetPolicyKey())
				}
			}
		}
	}

	seeds, err := seedBindingsFromGroups(seedGroups)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].GetStepKey() < slots[j].GetStepKey()
	})
	sort.Strings(included)
	return seeds, slots, policies, included, nil
}

// isTruthyCapabilityValue reports whether a SELECT yes/no value opts a
// capability in. Accepts "true"/"yes"/"on" (case-insensitive) and native bools.
func isTruthyCapabilityValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	default:
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
		case "true", "yes", "on":
			return true
		}
		return false
	}
}

func (b *seedBuilder) set(path string, value any) error {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" {
		b.wholeSet = true
		b.whole = value
		return nil
	}
	if !strings.HasPrefix(path, "$.") {
		return fmt.Errorf("unsupported jsonPath %q", path)
	}
	if b.object == nil {
		b.object = map[string]any{}
	}
	current := b.object
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return fmt.Errorf("unsupported jsonPath %q", path)
		}
		if i == len(parts)-1 {
			current[part] = value
			return nil
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	return nil
}

func seedBindingsFromGroups(groups map[seedTarget]*seedBuilder) ([]*plansv1.SeedArtifactBinding, error) {
	keys := make([]seedTarget, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].stepKey == keys[j].stepKey {
			return keys[i].inputName < keys[j].inputName
		}
		return keys[i].stepKey < keys[j].stepKey
	})

	seeds := make([]*plansv1.SeedArtifactBinding, 0, len(keys))
	for _, key := range keys {
		builder := groups[key]
		payload := any(builder.object)
		if builder.wholeSet {
			payload = builder.whole
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal seed artifact %s/%s: %w", key.stepKey, key.inputName, err)
		}
		seeds = append(seeds, &plansv1.SeedArtifactBinding{
			StepKey:     key.stepKey,
			InputName:   key.inputName,
			LiteralJson: string(raw),
		})
	}
	return seeds, nil
}

func applyBehaviorPolicy(policies *plansv1.PlanBehaviorPolicies, key string, value any) error {
	switch strings.TrimSpace(key) {
	case "publish_approval_mode":
		mode, ok := publishApprovalModeFromValue(value)
		if !ok {
			return fmt.Errorf("unsupported publish_approval_mode %q", fmt.Sprint(value))
		}
		policies.PublishApprovalMode = mode
	case "elicitation_timeout_behavior":
		behavior, ok := elicitationTimeoutBehaviorFromValue(value)
		if !ok {
			return fmt.Errorf("unsupported elicitation_timeout_behavior %q", fmt.Sprint(value))
		}
		policies.ElicitationTimeoutBehavior = behavior
	case "content_output_format":
		format, ok := contentOutputFormatFromValue(value)
		if !ok {
			return fmt.Errorf("unsupported content_output_format %q", fmt.Sprint(value))
		}
		policies.ContentOutputFormat = format
	default:
		return fmt.Errorf("unsupported behavior policy %q", key)
	}
	return nil
}

func publishApprovalModeFromValue(value any) (plansv1.PublishApprovalMode, bool) {
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "require_approval", "require_approval_required", "publish_approval_mode_require_approval":
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL, true
	case "auto_publish", "publish_approval_mode_auto_publish":
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH, true
	default:
		return plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED, false
	}
}

func contentOutputFormatFromValue(value any) (plansv1.ContentOutputFormat, bool) {
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "text_post", "content_output_format_text_post":
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_TEXT_POST, true
	case "carousel", "content_output_format_carousel":
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_CAROUSEL, true
	case "image_backed_post", "image_backed", "content_output_format_image_backed_post":
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_IMAGE_BACKED_POST, true
	case "approval_only", "content_output_format_approval_only":
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_APPROVAL_ONLY, true
	default:
		return plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED, false
	}
}

func elicitationTimeoutBehaviorFromValue(value any) (plansv1.ElicitationTimeoutBehavior, bool) {
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "pause_until_answered", "elicitation_timeout_behavior_pause_until_answered":
		return plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED, true
	case "fail_step", "elicitation_timeout_behavior_fail_step":
		return plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_STEP, true
	case "fail_plan", "elicitation_timeout_behavior_fail_plan":
		return plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_PLAN, true
	default:
		return plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED, false
	}
}

func ResolveDefaultSlotBindings(
	ctx context.Context,
	tenantID uuid.UUID,
	template *plansv1.PlanTemplate,
	existing []*plansv1.SlotBinding,
	executorRepo ExecutorLookup,
) ([]*plansv1.SlotBinding, error) {
	if template == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("plan template is required"))
	}
	if executorRepo == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("executor lookup is required"))
	}

	bound := make(map[string]struct{}, len(existing))
	for _, binding := range existing {
		if binding == nil {
			continue
		}
		if stepKey := strings.TrimSpace(binding.GetStepKey()); stepKey != "" {
			bound[stepKey] = struct{}{}
		}
	}

	var defaults []*plansv1.SlotBinding
	for _, step := range template.GetSteps() {
		if step == nil {
			continue
		}
		stepKey := strings.TrimSpace(step.GetKey())
		if stepKey == "" {
			continue
		}
		if _, ok := bound[stepKey]; ok {
			continue
		}
		defaultSKUKey := strings.TrimSpace(step.GetDefaultExecutorSkuKey())
		if defaultSKUKey == "" {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("step %q has no slot binding and no default executor sku", stepKey))
		}

		installations, err := executorRepo.ListCompatibleInstallationsForStep(ctx, tenantID, step)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("lookup default executor installation for step %q: %w", stepKey, err))
		}
		binding, err := defaultBindingForStep(ctx, executorRepo, tenantID, step, installations)
		if err != nil {
			return nil, err
		}
		defaults = append(defaults, binding)
		bound[stepKey] = struct{}{}
	}
	return defaults, nil
}

func defaultBindingForStep(ctx context.Context, executorRepo ExecutorLookup, tenantID uuid.UUID, step *plansv1.PlanStep, installations []executors.ExecutorInstallation) (*plansv1.SlotBinding, error) {
	stepKey := strings.TrimSpace(step.GetKey())
	defaultSKUKey := strings.TrimSpace(step.GetDefaultExecutorSkuKey())
	for i := range installations {
		installation := installations[i]
		if !installation.Enabled || installation.TenantID != tenantID {
			continue
		}
		sku, err := executorRepo.GetSKUByID(ctx, installation.ExecutorSKUID)
		if err != nil {
			continue
		}
		if sku.Key != defaultSKUKey {
			continue
		}
		kind, err := dbExecutorKindToProto(installation.Kind)
		if err != nil {
			continue
		}
		return &plansv1.SlotBinding{
			StepKey:                stepKey,
			ExecutorKind:           kind,
			ExecutorSkuId:          sku.ID.String(),
			ExecutorInstallationId: installation.ID.String(),
		}, nil
	}
	return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("step %q requires enabled executor installation for default sku %q", stepKey, defaultSKUKey))
}
