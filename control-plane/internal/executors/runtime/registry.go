package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ConfigValidatorRegistry struct {
	validators map[string]ConfigValidator
}

func NewConfigValidatorRegistry(validators ...ConfigValidator) *ConfigValidatorRegistry {
	registry := &ConfigValidatorRegistry{validators: make(map[string]ConfigValidator, len(validators))}
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		key := strings.TrimSpace(validator.SKUKey())
		if key == "" {
			continue
		}
		registry.validators[key] = validator
	}
	return registry
}

func (r *ConfigValidatorRegistry) Validate(skuKey string, raw json.RawMessage) error {
	if r == nil {
		return nil
	}
	key := strings.TrimSpace(skuKey)
	if key == "" {
		return errors.New("executor sku key is required")
	}
	validator, ok := r.validators[key]
	if !ok {
		return nil
	}
	return validator.ValidateConfig(raw)
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
