package image

import (
	"fmt"
	"os"
)

// DefaultProviderResolver is the production provider resolver. It reads the
// OpenAI API key lazily from the worker environment (referenced by
// config.APIKeyEnv) so the key never enters config_json or Temporal history.
// The noop provider needs no credentials and is the dev/test default.
func DefaultProviderResolver(cfg InstallationConfig) (ImageProvider, error) {
	switch cfg.Provider {
	case ProviderNoop:
		return NewNoopProvider(), nil
	case ProviderOpenAI:
		apiKey := os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" {
			return nil, NewProviderError(ProviderOpenAI, fmt.Errorf("api key environment variable %q is not set", cfg.APIKeyEnv))
		}
		return NewDallEProvider(apiKey), nil
	default:
		return nil, fmt.Errorf("%w: unknown provider %q", ErrInvalidConfig, cfg.Provider)
	}
}
