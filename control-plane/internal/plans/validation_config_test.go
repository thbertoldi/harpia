package plans

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/executors"
)

func TestValidateSlotBindingsRunnableRejectsInvalidRSSConfig(t *testing.T) {
	tenantID := uuid.New()
	skuID := uuid.New()
	installationID := uuid.New()
	connected := "connected"
	manifestID := "linkedin-voice-senior"
	manifestVersion := "1.0.0"
	agentInstallationID := uuid.New()
	agentSKUID := uuid.New()

	validator := NewBindingValidator(&mockExecutorLookup{
		installations: map[uuid.UUID]*executors.ExecutorInstallation{
			installationID: {
				ID:               installationID,
				TenantID:         tenantID,
				ExecutorSKUID:    skuID,
				Kind:             executors.KindIntegration,
				Enabled:          true,
				ConnectionStatus: &connected,
				ConfigJSON:       json.RawMessage(`{"feeds":["not-a-url"]}`),
			},
			agentInstallationID: {
				ID:              agentInstallationID,
				TenantID:        tenantID,
				ExecutorSKUID:   agentSKUID,
				Kind:            executors.KindAgent,
				Enabled:         true,
				ManifestID:      &manifestID,
				ManifestVersion: &manifestVersion,
			},
		},
		skus: map[uuid.UUID]*executors.ExecutorSKU{
			skuID:      {ID: skuID, Key: executors.SKURSSNewsFeed, Kind: executors.KindIntegration},
			agentSKUID: {ID: agentSKUID, Key: "linkedin-voice-senior", Kind: executors.KindAgent},
		},
		entitledSKUs: map[uuid.UUID]bool{skuID: true, agentSKUID: true},
	})

	bindings := []*plansv1.SlotBinding{
		{
			StepKey:                "fetch-news",
			ExecutorSkuId:          skuID.String(),
			ExecutorInstallationId: installationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
		},
		{
			StepKey:                "adapt-for-linkedin",
			ExecutorSkuId:          agentSKUID.String(),
			ExecutorInstallationId: agentInstallationID.String(),
			ExecutorKind:           plansv1.ExecutorKind_EXECUTOR_KIND_AGENT,
		},
	}

	err := validator.ValidateSlotBindings(context.Background(), tenantID, testTemplate(), plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, bindings, nil)
	assertBindingError(t, err, connect.CodeFailedPrecondition, "absolute URL")
}
