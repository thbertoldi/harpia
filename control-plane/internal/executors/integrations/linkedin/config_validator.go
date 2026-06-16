package linkedin

import (
	"encoding/json"

	"github.com/harpia/control-plane/internal/executors/catalog"
)

type ConfigValidator struct{}

func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

func (v *ConfigValidator) SKUKey() string {
	return catalog.SKULinkedInPublish
}

func (v *ConfigValidator) ValidateConfig(raw json.RawMessage) error {
	_, err := ParseInstallationConfig(raw)
	return err
}
