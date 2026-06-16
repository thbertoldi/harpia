package linkedin

import (
	"encoding/json"
	"fmt"
	"strings"
)

type InstallationConfig struct {
	OAuthCredentialID string `json:"oauth_credential_id"`
}

func ParseInstallationConfig(raw json.RawMessage) (InstallationConfig, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return InstallationConfig{}, fmt.Errorf("%w: oauth_credential_id is required", ErrInvalidConfig)
	}
	if !json.Valid(raw) {
		return InstallationConfig{}, fmt.Errorf("%w: config_json must be valid JSON", ErrInvalidConfig)
	}

	var config InstallationConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return InstallationConfig{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	config.OAuthCredentialID = strings.TrimSpace(config.OAuthCredentialID)
	if config.OAuthCredentialID == "" {
		return InstallationConfig{}, fmt.Errorf("%w: oauth_credential_id is required", ErrInvalidConfig)
	}

	return config, nil
}
