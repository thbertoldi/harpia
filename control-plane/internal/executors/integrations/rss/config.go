package rss

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type InstallationConfig struct {
	Feeds []string `json:"feeds"`
}

func ParseInstallationConfig(raw json.RawMessage) (InstallationConfig, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return InstallationConfig{}, fmt.Errorf("%w: feeds are required", ErrInvalidConfig)
	}
	if !json.Valid(raw) {
		return InstallationConfig{}, fmt.Errorf("%w: config_json must be valid JSON", ErrInvalidConfig)
	}

	var config InstallationConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return InstallationConfig{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if len(config.Feeds) == 0 {
		return InstallationConfig{}, fmt.Errorf("%w: feeds must not be empty", ErrInvalidConfig)
	}

	normalized := make([]string, 0, len(config.Feeds))
	seen := make(map[string]struct{}, len(config.Feeds))
	for i, feedURL := range config.Feeds {
		feedURL = strings.TrimSpace(feedURL)
		if feedURL == "" {
			return InstallationConfig{}, fmt.Errorf("%w: feeds[%d] is empty", ErrInvalidConfig, i)
		}
		parsed, err := url.Parse(feedURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return InstallationConfig{}, fmt.Errorf("%w: feeds[%d] must be an absolute URL", ErrInvalidConfig, i)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return InstallationConfig{}, fmt.Errorf("%w: feeds[%d] must use http or https", ErrInvalidConfig, i)
		}
		if _, ok := seen[feedURL]; ok {
			continue
		}
		seen[feedURL] = struct{}{}
		normalized = append(normalized, feedURL)
	}
	if len(normalized) == 0 {
		return InstallationConfig{}, fmt.Errorf("%w: feeds must not be empty", ErrInvalidConfig)
	}

	config.Feeds = normalized
	return config, nil
}
