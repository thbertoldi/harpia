package executors_test

import (
	"context"
	"testing"

	"github.com/harpia/control-plane/internal/executors"
)

func TestIntegrationRegistryRequiresSKUKey(t *testing.T) {
	registry := executors.NewIntegrationRegistry()
	result, err := registry.Run(context.Background(), executors.IntegrationExecutionRequest{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != executors.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}
