package executors

import (
	"context"
	"strings"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func (r *Repository) ListCompatibleInstallationsForStep(
	ctx context.Context,
	tenantID uuid.UUID,
	step *plansv1.PlanStep,
) ([]ExecutorInstallation, error) {
	if r == nil {
		return nil, nil
	}
	requirement := step.GetExecutorRequirement()
	if requirement == nil {
		return nil, nil
	}

	entitlements, err := r.ListEntitlements(ctx, tenantID, nil, 500, 0)
	if err != nil {
		return nil, err
	}
	entitledSKUs := make(map[uuid.UUID]struct{}, len(entitlements))
	for _, e := range entitlements {
		entitledSKUs[e.ExecutorSKUID] = struct{}{}
	}

	installations, err := r.ListInstallations(ctx, tenantID, "", nil, 500, 0)
	if err != nil {
		return nil, err
	}

	defaultSKUKey := strings.TrimSpace(step.GetDefaultExecutorSkuKey())
	var out []ExecutorInstallation
	for i := range installations {
		inst := installations[i]
		if !inst.Enabled {
			continue
		}
		if _, ok := entitledSKUs[inst.ExecutorSKUID]; !ok {
			continue
		}
		sku, err := r.GetSKUByID(ctx, inst.ExecutorSKUID)
		if err != nil {
			continue
		}
		if defaultSKUKey != "" && sku.Key != defaultSKUKey {
			continue
		}
		if !executorKindMatches(requirement.GetExecutorKind(), inst.Kind) {
			continue
		}
		connectionType := strings.TrimSpace(requirement.GetConnectionType())
		if connectionType != "" &&
			requirement.GetExecutorKind() == plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION &&
			sku.Kind != KindIntegration {
			continue
		}
		out = append(out, inst)
	}
	return out, nil
}

func executorKindMatches(stepKind plansv1.ExecutorKind, installationKind string) bool {
	switch stepKind {
	case plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION:
		return installationKind == KindIntegration
	case plansv1.ExecutorKind_EXECUTOR_KIND_AGENT:
		return installationKind == KindAgent
	default:
		return false
	}
}
