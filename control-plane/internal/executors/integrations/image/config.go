package image

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProviderOpenAI = "openai"
	ProviderNoop   = "noop"

	defaultModel   = "dall-e-3"
	defaultSize    = "1024x1024"
	defaultQuality = "standard"
)

// knownModels are the OpenAI Images API models the DALL-E adapter supports.
var knownModels = map[string]struct{}{
	"dall-e-3": {},
	"dall-e-2": {},
}

// knownSizes are the dimensions accepted by the OpenAI Images API for dall-e-3.
var knownSizes = map[string]struct{}{
	"1024x1024": {},
	"1792x1024": {},
	"1024x1792": {},
	"512x512":   {}, // dall-e-2
	"256x256":   {}, // dall-e-2
}

// knownQualities are the quality tiers accepted by the OpenAI Images API.
var knownQualities = map[string]struct{}{
	"standard": {},
	"hd":       {},
}

// InstallationConfig is the tenant-side configuration for an image-asset-generator
// installation. Provider credentials are deliberately excluded: the server resolves
// them from its own provider-to-environment configuration at execution time.
type InstallationConfig struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	DefaultSize    string `json:"default_size"`
	DefaultQuality string `json:"default_quality"`
}

// ParseInstallationConfig parses and validates an image-asset-generator
// installation config. It rejects empty configs, unknown providers, inline API
// keys, and provider-specific invalid model/size/quality values.
func ParseInstallationConfig(raw json.RawMessage) (InstallationConfig, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return InstallationConfig{}, fmt.Errorf("%w: provider is required", ErrInvalidConfig)
	}
	if !json.Valid(raw) {
		return InstallationConfig{}, fmt.Errorf("%w: config_json must be valid JSON", ErrInvalidConfig)
	}

	if err := validateTenantConfigFields(raw); err != nil {
		return InstallationConfig{}, err
	}

	var config InstallationConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return InstallationConfig{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	config.Provider = strings.TrimSpace(config.Provider)
	if config.Provider == "" {
		return InstallationConfig{}, fmt.Errorf("%w: provider is required", ErrInvalidConfig)
	}
	if config.Provider != ProviderOpenAI && config.Provider != ProviderNoop {
		return InstallationConfig{}, fmt.Errorf("%w: provider must be openai or noop", ErrInvalidConfig)
	}

	config.Model = strings.TrimSpace(config.Model)
	config.DefaultSize = strings.TrimSpace(config.DefaultSize)
	config.DefaultQuality = strings.TrimSpace(config.DefaultQuality)

	if config.Provider == ProviderOpenAI {
		if err := validateOpenAIDefaults(&config); err != nil {
			return InstallationConfig{}, err
		}
	}

	return config, nil
}

// validateTenantConfigFields permits only the documented tenant configuration
// shape. In particular, credential-like keys and api_key_env are rejected so
// a tenant cannot influence which worker secret os.Getenv reads.
func validateTenantConfigFields(raw json.RawMessage) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("%w: config_json must be a JSON object", ErrInvalidConfig)
	}
	for field := range probe {
		switch normalized := strings.ToLower(strings.TrimSpace(field)); normalized {
		case "api_key", "key", "api_key_env", "apikey", "secret", "token":
			return fmt.Errorf("%w: credential field %q is not allowed", ErrInvalidConfig, field)
		case "provider", "model", "default_size", "default_quality":
			continue
		default:
			return fmt.Errorf("%w: unsupported field %q", ErrInvalidConfig, field)
		}
	}
	return nil
}

func validateOpenAIDefaults(config *InstallationConfig) error {
	if config.Model == "" {
		config.Model = defaultModel
	}
	if _, ok := knownModels[config.Model]; !ok {
		return fmt.Errorf("%w: model %q is not supported", ErrInvalidConfig, config.Model)
	}

	if config.DefaultSize == "" {
		config.DefaultSize = defaultSize
	}
	if _, ok := knownSizes[config.DefaultSize]; !ok {
		return fmt.Errorf("%w: default_size %q is not supported", ErrInvalidConfig, config.DefaultSize)
	}

	if config.DefaultQuality == "" {
		config.DefaultQuality = defaultQuality
	}
	if _, ok := knownQualities[config.DefaultQuality]; !ok {
		return fmt.Errorf("%w: default_quality %q is not supported", ErrInvalidConfig, config.DefaultQuality)
	}
	return nil
}
