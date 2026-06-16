package runtime

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

const (
	IntegrationStatusCompleted = "completed"
	IntegrationStatusFailed    = "failed"
)

type InputArtifactRef struct {
	ArtifactTypeKey string
	ArtifactID      string
	LiteralJSON     string
}

type InstallationSnapshot struct {
	ID             string
	ExecutorSKUID  string
	ExecutorSKUKey string
	ConfigJSON     json.RawMessage
}

type IntegrationExecutionRequest struct {
	TenantID              uuid.UUID
	StepExecutionID       string
	PlanStepKey           string
	InputArtifacts        []InputArtifactRef
	OutputArtifactTypeKey string
	Installation          InstallationSnapshot
}

type IntegrationExecutionResult struct {
	Status           string
	OutputArtifactID string
	Error            string
}

// IntegrationRunner routes execution to a registered handler by SKU key.
type IntegrationRunner interface {
	Run(ctx context.Context, req IntegrationExecutionRequest) (IntegrationExecutionResult, error)
}

// IntegrationHandler satisfies one IntegrationExecutor SKU contract at runtime.
type IntegrationHandler interface {
	SKUKey() string
	Execute(ctx context.Context, req IntegrationExecutionRequest) (IntegrationExecutionResult, error)
}
