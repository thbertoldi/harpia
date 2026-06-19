package executors

import (
	"github.com/harpia/control-plane/internal/executors/runtime"
)

const (
	IntegrationStatusCompleted = runtime.IntegrationStatusCompleted
	IntegrationStatusFailed    = runtime.IntegrationStatusFailed
)

type (
	InputArtifactRef             = runtime.InputArtifactRef
	InstallationSnapshot         = runtime.InstallationSnapshot
	IntegrationExecutionRequest  = runtime.IntegrationExecutionRequest
	IntegrationExecutionResult   = runtime.IntegrationExecutionResult
	IntegrationHandler           = runtime.IntegrationHandler
	IntegrationRunner            = runtime.IntegrationRunner
	IntegrationRegistry          = runtime.IntegrationRegistry
	RetryableError               = runtime.RetryableError
	ExecutorArtifactStore        = runtime.ExecutorArtifactStore
	ExecutorArtifactStoreAdapter = runtime.ExecutorArtifactStoreAdapter
	CreateArtifactRequest        = runtime.CreateArtifactRequest
	ConfigValidatorRegistry      = runtime.ConfigValidatorRegistry
	ConfigValidator              = runtime.ConfigValidator
)

var (
	NewIntegrationRegistry   = runtime.NewIntegrationRegistry
	NewRetryableError        = runtime.NewRetryableError
	NewExecutorArtifactStore = runtime.NewExecutorArtifactStore
)
