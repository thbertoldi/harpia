package executors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	IntegrationStatusCompleted = "completed"
	IntegrationStatusFailed    = "failed"
)

type InputArtifactRef struct {
	ArtifactType string
	ArtifactID   string
	LiteralJSON  string
}

type InstallationSnapshot struct {
	ID             string
	ExecutorSKUID  string
	ExecutorSKUKey string
	ConfigJSON     json.RawMessage
}

type IntegrationExecutionRequest struct {
	TenantID             uuid.UUID
	StepExecutionID      string
	PlanStepKey          string
	InputArtifacts       []InputArtifactRef
	OutputArtifactTypeKey string
	Installation         InstallationSnapshot
}

type IntegrationExecutionResult struct {
	Status           string
	OutputArtifactID string
	Error            string
}

// IntegrationHandler satisfies one IntegrationExecutor SKU contract.
type IntegrationHandler interface {
	SKUKey() string
	Execute(ctx context.Context, req IntegrationExecutionRequest) (IntegrationExecutionResult, error)
}

// IntegrationRunner routes execution to a registered handler by SKU key.
type IntegrationRunner interface {
	Run(ctx context.Context, req IntegrationExecutionRequest) (IntegrationExecutionResult, error)
}

type IntegrationRegistry struct {
	handlers map[string]IntegrationHandler
}

func NewIntegrationRegistry(handlers ...IntegrationHandler) *IntegrationRegistry {
	registry := &IntegrationRegistry{handlers: make(map[string]IntegrationHandler, len(handlers))}
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		key := strings.TrimSpace(handler.SKUKey())
		if key == "" {
			continue
		}
		registry.handlers[key] = handler
	}
	return registry
}

func (r *IntegrationRegistry) Run(ctx context.Context, req IntegrationExecutionRequest) (IntegrationExecutionResult, error) {
	if r == nil {
		return IntegrationExecutionResult{}, errors.New("integration registry is not configured")
	}

	skuKey := strings.TrimSpace(req.Installation.ExecutorSKUKey)
	if skuKey == "" {
		return failedIntegrationResult("executor sku key is required"), nil
	}

	handler, ok := r.handlers[skuKey]
	if !ok {
		return failedIntegrationResult(fmt.Sprintf("integration handler not registered for sku %q", skuKey)), nil
	}

	return handler.Execute(ctx, req)
}

func failedIntegrationResult(message string) IntegrationExecutionResult {
	return IntegrationExecutionResult{
		Status: IntegrationStatusFailed,
		Error:  message,
	}
}

// RetryableError marks a transient integration failure for the Temporal adapter.
type RetryableError struct {
	Code    string
	Message string
	Cause   error
}

func NewRetryableError(code, message string, cause error) *RetryableError {
	return &RetryableError{Code: code, Message: message, Cause: cause}
}

func (e *RetryableError) Error() string {
	if e == nil {
		return "retryable integration error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "retryable integration error"
}

func (e *RetryableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
