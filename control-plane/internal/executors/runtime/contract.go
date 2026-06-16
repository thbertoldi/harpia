package runtime

import "encoding/json"

// IntegrationContract describes the stable SKU contract used by plan slot bindings
// and integration handlers.
type IntegrationContract struct {
	SKUKey                 string
	InputArtifactTypeKeys  []string
	OutputArtifactTypeKeys []string
}

// ConfigValidator validates tenant installation config_json before persistence
// and during plan readiness checks.
type ConfigValidator interface {
	SKUKey() string
	ValidateConfig(raw json.RawMessage) error
}
