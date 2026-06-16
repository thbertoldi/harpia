package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"

	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

func (a *PlanActivities) RunIntegrationActivity(ctx context.Context, input ExecutorActivityInput) (ExecutorActivityResult, error) {
	if a == nil || a.Integrations == nil {
		return ExecutorActivityResult{}, errors.New("integration runner is not configured")
	}

	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return ExecutorActivityResult{
			Status: ExecutorResultStatusFailed,
			Error:  err.Error(),
		}, nil
	}

	result, err := a.Integrations.Run(ctx, mapIntegrationRequest(tenantID, input))
	if err != nil {
		var retryable *runtime.RetryableError
		if errors.As(err, &retryable) {
			return ExecutorActivityResult{}, temporal.NewApplicationError(
				retryable.Error(),
				retryable.Code,
			)
		}
		return ExecutorActivityResult{}, err
	}

	return mapIntegrationResult(result), nil
}

func mapIntegrationRequest(tenantID uuid.UUID, input ExecutorActivityInput) executors.IntegrationExecutionRequest {
	inputArtifacts := make([]executors.InputArtifactRef, 0, len(input.InputArtifacts))
	for _, ref := range input.InputArtifacts {
		inputArtifacts = append(inputArtifacts, executors.InputArtifactRef{
			ArtifactTypeKey: ref.ArtifactTypeKey,
			ArtifactID:   ref.ArtifactID,
			LiteralJSON:  ref.LiteralJSON,
		})
	}

	var configJSON json.RawMessage
	if trimmed := input.ExecutorInstallationSnapshot.ConfigJSON; trimmed != "" {
		configJSON = json.RawMessage(trimmed)
	}

	return executors.IntegrationExecutionRequest{
		TenantID:             tenantID,
		StepExecutionID:      input.StepExecutionID,
		PlanStepKey:          input.PlanStepKey,
		InputArtifacts:       inputArtifacts,
		OutputArtifactTypeKey: input.OutputArtifactTypeKey,
		Installation: executors.InstallationSnapshot{
			ID:             input.ExecutorInstallationSnapshot.ID,
			ExecutorSKUID:  input.ExecutorInstallationSnapshot.ExecutorSKUID,
			ExecutorSKUKey: input.ExecutorInstallationSnapshot.ExecutorSKUKey,
			ConfigJSON:     configJSON,
		},
	}
}

func mapIntegrationResult(result executors.IntegrationExecutionResult) ExecutorActivityResult {
	switch result.Status {
	case executors.IntegrationStatusCompleted:
		return ExecutorActivityResult{
			Status:           ExecutorResultStatusCompleted,
			OutputArtifactID: result.OutputArtifactID,
		}
	default:
		message := result.Error
		if message == "" {
			message = "integration failed"
		}
		return ExecutorActivityResult{
			Status: ExecutorResultStatusFailed,
			Error:  message,
		}
	}
}
