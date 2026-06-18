package llm_config

import (
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// normalizeProvider canonicalizes provider identifiers to stable lowercase
// strings so blocklists, DB unique keys, and env fallbacks stay consistent.
func normalizeProvider(provider string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	if normalized == "" {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("provider is required"))
	}
	return normalized, nil
}
