package linkedin_test

import (
	"encoding/json"
	"testing"

	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/integrations/linkedin"
)

func TestConfigValidatorSKUKey(t *testing.T) {
	if got := linkedin.NewConfigValidator().SKUKey(); got != executors.SKULinkedInPublish {
		t.Fatalf("SKUKey() = %q, want %q", got, executors.SKULinkedInPublish)
	}
}

func TestConfigValidatorRejectsInvalidConfig(t *testing.T) {
	validator := linkedin.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected invalid config error")
	}
}

func TestConfigValidatorAcceptsApprovalOnlyConfig(t *testing.T) {
	validator := linkedin.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{"mode":"approval_only"}`)); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}
