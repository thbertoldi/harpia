package image

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProviderOpenAI = "openai"
	ProviderNoop   = "noop"

	defaultModel    = "dall-e-3"
	defaultSize     = "1024x1024"
	defaultQuality  = "standard"
	noAPIKeyEnvName = "HARPIA_IMAGE_NOOP_KEY" // referenced by noop configs that still carry api_key_env
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
// installation. The API key is referenced by environment variable name
// (APIKeyEnv), never stored inline in config_json.
type InstallationConfig struct {
	Provider       string `json:"provider"`
	APIKeyEnv      string `json:"api_key_env"`
	Model          string `json:"model"`
	DefaultSize    string `json:"default_size"`
	DefaultQuality string `json:"default_quality"`
}

// ParseInstallationConfig parses and validates an image-asset-generator
// installation config. It rejects empty configs, unknown providers, inline API
// keys (api_key/key fields), and provider-specific invalid model/size/quality
// values. For the noop provider the api_key_env is optional (dev path); for
// openai it is required.
func ParseInstallationConfig(raw json.RawMessage) (InstallationConfig, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return InstallationConfig{}, fmt.Errorf("%w: provider is required", ErrInvalidConfig)
	}
	if !json.Valid(raw) {
		return InstallationConfig{}, fmt.Errorf("%w: config_json must be valid JSON", ErrInvalidConfig)
	}

	if err := rejectInlineAPIKey(raw); err != nil {
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

	config.APIKeyEnv = strings.TrimSpace(config.APIKeyEnv)
	if config.Provider == ProviderOpenAI && config.APIKeyEnv == "" {
		return InstallationConfig{}, fmt.Errorf("%w: api_key_env is required for provider openai", ErrInvalidConfig)
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

// rejectInlineAPIKey fails when config_json carries an inline api_key or key
// field. The API key must live in the worker environment and be referenced by
// api_key_env so it never enters config_json or Temporal history.
func rejectInlineAPIKey(raw json.RawMessage) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("%w: config_json must be a JSON object", ErrInvalidConfig)
	}
	for field := range probe {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "api_key", "key", "apikey", "secret", "token":
			return fmt.Errorf("%w: api key must be referenced via api_key_env, not stored inline as %q", ErrInvalidConfig, field)
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
