package llm_config

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	cryptoenv "github.com/harpia/control-plane/internal/llm_config/crypto"
	"github.com/harpia/control-plane/internal/llm_config/secret"
)

// Stable error reason strings (design §6).
const (
	ReasonNoProviderConfigured      = "LLM_NO_PROVIDER_CONFIGURED"
	ReasonProviderBlockedByPlatform = "LLM_PROVIDER_BLOCKED_BY_PLATFORM"
	ReasonKeyDecryptionFailed       = "LLM_KEY_DECRYPTION_FAILED"
)

// ResolutionSource identifies which leg of the fallback chain produced the
// credential (tenant_self or platform). The proto layer maps this to
// `ManagedBy`.
type ResolutionSource string

const (
	SourceTenantSelf ResolutionSource = "tenant_self"
	SourcePlatform   ResolutionSource = "platform"
)

// ResolveError is the typed error surfaced to handler callers carrying one of
// the stable Reason* constants. It implements `error` so it can travel through
// the normal error chain, and the handler maps it to a Connect error envelope.
type ResolveError struct {
	Reason  string
	Wrapped error
}

// Error implements error.
func (e *ResolveError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %v", e.Reason, e.Wrapped)
	}
	return e.Reason
}

// Unwrap surfaces the underlying cause.
func (e *ResolveError) Unwrap() error { return e.Wrapped }

// ResolvedCredential is the in-process result of the fallback chain.
//
// The plaintext API key is wrapped in secret.Redacted; callers must call
// Reveal() at the exact moment they hand it to the provider SDK and never
// log the result.
type ResolvedCredential struct {
	Provider      string
	DefaultModel  string
	AllowedModels []string
	APIKey        secret.Redacted
	Source        ResolutionSource
}

// Resolver implements the fallback chain (design §6):
//
//  1. Tenant-scoped key/config if present and permitted.
//  2. Platform-scoped provider key/config if tenant key absent.
//  3. Terminal error otherwise.
//
// The resolver consults BlockedProviders before either leg: a globally
// blocked provider always returns LLM_PROVIDER_BLOCKED_BY_PLATFORM even when
// the tenant has uploaded a key.
type Resolver struct {
	repo             *Repository
	keyring          *cryptoenv.Keyring
	platformKeys     PlatformKeyStore
	blockedProviders map[string]struct{}
}

// NewResolver wires a resolver with the keyring, repository, platform keys,
// and (optionally) the blocked-provider set.
func NewResolver(repo *Repository, keyring *cryptoenv.Keyring, platform PlatformKeyStore, blocked map[string]struct{}) *Resolver {
	if blocked == nil {
		blocked = map[string]struct{}{}
	}
	if platform == nil {
		platform = NewEnvPlatformKeyStore()
	}
	return &Resolver{
		repo:             repo,
		keyring:          keyring,
		platformKeys:     platform,
		blockedProviders: blocked,
	}
}

// Resolve runs the fallback chain for (tenant, provider).
//
// requestedModel is currently advisory; if a tenant set `allowed_models` and
// the requested model is not in the list, resolution fails with
// LLM_PROVIDER_BLOCKED_BY_PLATFORM (design §6 makes provider-blocking the
// signal for "platform policy disallowed the call").
func (r *Resolver) Resolve(ctx context.Context, tenantID uuid.UUID, provider, requestedModel string) (*ResolvedCredential, error) {
	normalized, err := normalizeProvider(provider)
	if err != nil {
		return nil, &ResolveError{Reason: ReasonNoProviderConfigured, Wrapped: err}
	}
	provider = normalized

	if _, blocked := r.blockedProviders[provider]; blocked {
		return nil, &ResolveError{Reason: ReasonProviderBlockedByPlatform, Wrapped: fmt.Errorf("provider %q is blocked by platform policy", provider)}
	}

	tenantCfg, err := r.repo.GetByProvider(ctx, tenantID, provider)
	switch {
	case errors.Is(err, ErrNotFound):
		// Fall through to platform leg.
	case err != nil:
		return nil, err
	default:
		if requestedModel != "" && !modelAllowed(tenantCfg.AllowedModels, requestedModel) {
			return nil, &ResolveError{
				Reason:  ReasonProviderBlockedByPlatform,
				Wrapped: fmt.Errorf("model %q not in tenant allowed_models", requestedModel),
			}
		}
		plaintext, err := cryptoenv.DecryptWithKeyring(r.keyring, tenantCfg.AsEncryptedRecord())
		if err != nil {
			return nil, &ResolveError{Reason: ReasonKeyDecryptionFailed, Wrapped: err}
		}
		return &ResolvedCredential{
			Provider:      tenantCfg.Provider,
			DefaultModel:  tenantCfg.DefaultModel,
			AllowedModels: tenantCfg.AllowedModels,
			APIKey:        secret.NewRedacted(string(plaintext)),
			Source:        SourceTenantSelf,
		}, nil
	}

	platformKey, ok := r.platformKeys.Lookup(provider)
	if !ok {
		return nil, &ResolveError{Reason: ReasonNoProviderConfigured, Wrapped: ErrPlatformNotConfigured}
	}
	return &ResolvedCredential{
		Provider: provider,
		APIKey:   secret.NewRedacted(platformKey),
		Source:   SourcePlatform,
	}, nil
}

func modelAllowed(allowed []string, requested string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, m := range allowed {
		if m == requested {
			return true
		}
	}
	return false
}
