package image

import (
	"fmt"
	"os"
	"strings"
)

// NewProviderResolver creates a server-controlled provider resolver. The map is
// deployment configuration, not tenant installation data. A nil or partial map
// retains the default OpenAI mapping.
func NewProviderResolver(providerAPIKeyEnvs map[string]string) ProviderResolver {
	envByProvider := map[string]string{ProviderOpenAI: "OPENAI_API_KEY"}
	for provider, envName := range providerAPIKeyEnvs {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		envByProvider[provider] = strings.TrimSpace(envName)
	}

	return func(cfg InstallationConfig) (ImageProvider, error) {
		switch cfg.Provider {
		case ProviderNoop:
			return NewNoopProvider(), nil
		case ProviderOpenAI:
			envName := envByProvider[ProviderOpenAI]
			apiKey := strings.TrimSpace(os.Getenv(envName))
			if apiKey == "" {
				return nil, NewProviderError(ProviderOpenAI, fmt.Errorf("credentials are not configured"))
			}
			return NewDallEProvider(apiKey), nil
		default:
			return nil, fmt.Errorf("%w: unknown provider %q", ErrInvalidConfig, cfg.Provider)
		}
	}
}

// DefaultProviderResolver uses the default server-side provider mapping.
var DefaultProviderResolver = NewProviderResolver(nil)
