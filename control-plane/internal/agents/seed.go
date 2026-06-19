package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/executors"
)

// EnsureAgentTypesForTenant upserts agent_types rows for entitled agent executor SKUs.
func EnsureAgentTypesForTenant(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	executorRepo := executors.NewRepository(pool)
	agentRepo := NewRepository(pool)

	entitlements, err := executorRepo.ListEntitlements(ctx, tenantID, nil, 100, 0)
	if err != nil {
		return fmt.Errorf("list entitlements: %w", err)
	}

	for _, entitlement := range entitlements {
		sku, err := executorRepo.GetSKUByID(ctx, entitlement.ExecutorSKUID)
		if err != nil {
			return fmt.Errorf("load sku for entitlement: %w", err)
		}
		if sku.Kind != executors.KindAgent {
			continue
		}

		manifestID := sku.Compatibility.ManifestID
		if manifestID == "" {
			return fmt.Errorf("agent sku %q is missing manifest id", sku.Key)
		}

		if _, err := agentRepo.UpsertByName(ctx, tenantID, &AgentType{
			Name:         manifestID,
			Description:  sku.Description,
			InputSchema:  json.RawMessage("{}"),
			OutputSchema: json.RawMessage("{}"),
			MCPServers:   json.RawMessage("[]"),
			Enabled:      true,
		}); err != nil {
			return fmt.Errorf("upsert agent type %q: %w", manifestID, err)
		}
	}

	return nil
}
