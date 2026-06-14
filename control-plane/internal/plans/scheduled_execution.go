package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) CreateScheduledExecution(ctx context.Context, tenantID, configID uuid.UUID) error {
	config, err := r.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("get plan configuration: %w", err)
	}

	if config.Status != ConfigurationStatusScheduled {
		return nil
	}

	snapshot, err := json.Marshal(configurationToProto(config))
	if err != nil {
		return fmt.Errorf("marshal plan configuration snapshot: %w", err)
	}

	now := time.Now().UTC()
	_, err = r.CreateExecution(ctx, &PlanExecution{
		TenantID:                  tenantID,
		PlanConfigurationID:       configID,
		PlanConfigurationSnapshot: snapshot,
		Status:                    ExecutionStatusPending,
		TriggeredAt:               &now,
	})
	if err != nil {
		return fmt.Errorf("create scheduled plan execution: %w", err)
	}

	return nil
}
