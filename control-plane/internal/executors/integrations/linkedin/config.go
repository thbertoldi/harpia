package linkedin

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ModeOAuth        = "oauth"
	ModeApprovalOnly = "approval_only"
)

type InstallationConfig struct {
	Mode              string `json:"mode"`
	OAuthCredentialID string `json:"oauth_credential_id"`
	AuthorURN         string `json:"author_urn"`
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

	config.Mode = strings.TrimSpace(config.Mode)
	if config.Mode == "" {
		config.Mode = ModeOAuth
	}
	if config.Mode != ModeOAuth && config.Mode != ModeApprovalOnly {
		return InstallationConfig{}, fmt.Errorf("%w: mode must be oauth or approval_only", ErrInvalidConfig)
	}

	config.OAuthCredentialID = strings.TrimSpace(config.OAuthCredentialID)
	config.AuthorURN = strings.TrimSpace(config.AuthorURN)
	if config.Mode == ModeOAuth && config.OAuthCredentialID == "" {
		return InstallationConfig{}, fmt.Errorf("%w: oauth_credential_id is required", ErrInvalidConfig)
	}
	if config.Mode == ModeOAuth && config.AuthorURN == "" {
		return InstallationConfig{}, fmt.Errorf("%w: author_urn is required", ErrInvalidConfig)
	}
	if config.Mode == ModeApprovalOnly {
		config.OAuthCredentialID = ""
		config.AuthorURN = ""
	}

	return config, nil
}
