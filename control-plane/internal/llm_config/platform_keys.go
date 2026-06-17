package llm_config

import (
	"errors"
	"os"
	"strings"
	"sync"
)

// PlatformKeyStore returns platform-scoped provider credentials used as the
// second leg of the fallback chain (design §6). Platform keys live in
// Kubernetes Secrets (mounted file or env projection) and are refreshed only
// on pod restart. The store interface lets tests inject deterministic
// material without touching real env or files.
type PlatformKeyStore interface {
	// Lookup returns the platform-managed API key for the given provider, or
	// (empty, false) when no platform fallback is configured for it.
	Lookup(provider string) (string, bool)
}

// EnvPlatformKeyStore reads platform-key fallbacks from environment variables
// named HARPIA_PLATFORM_<PROVIDER_UPPER>_API_KEY. Empty values are treated
// as "not configured".
type EnvPlatformKeyStore struct {
	prefix string
	suffix string
	cache  sync.Map // provider -> string
}

// NewEnvPlatformKeyStore returns a store reading from
// HARPIA_PLATFORM_<PROVIDER>_API_KEY.
func NewEnvPlatformKeyStore() *EnvPlatformKeyStore {
	return &EnvPlatformKeyStore{
		prefix: "HARPIA_PLATFORM_",
		suffix: "_API_KEY",
	}
}

// Lookup implements PlatformKeyStore.
func (s *EnvPlatformKeyStore) Lookup(provider string) (string, bool) {
	if provider == "" {
		return "", false
	}
	if v, ok := s.cache.Load(provider); ok {
		key, _ := v.(string)
		return key, key != ""
	}
	envKey := s.prefix + strings.ToUpper(provider) + s.suffix
	value := strings.TrimSpace(os.Getenv(envKey))
	s.cache.Store(provider, value)
	return value, value != ""
}

// StaticPlatformKeyStore is a test-friendly implementation backed by an
// in-memory map.
type StaticPlatformKeyStore struct {
	keys map[string]string
}

// NewStaticPlatformKeyStore wraps the provided map.
func NewStaticPlatformKeyStore(keys map[string]string) *StaticPlatformKeyStore {
	copied := make(map[string]string, len(keys))
	for k, v := range keys {
		copied[k] = v
	}
	return &StaticPlatformKeyStore{keys: copied}
}

// Lookup implements PlatformKeyStore.
func (s *StaticPlatformKeyStore) Lookup(provider string) (string, bool) {
	v, ok := s.keys[provider]
	return v, ok && v != ""
}

// BlockedProvidersFromEnv parses HARPIA_LLM_BLOCKED_PROVIDERS (comma-separated)
// into a lookup set. This is the platform-policy gate for the fallback chain;
// if a provider is in this set the resolver returns
// LLM_PROVIDER_BLOCKED_BY_PLATFORM even when keys exist.
func BlockedProvidersFromEnv() map[string]struct{} {
	out := map[string]struct{}{}
	raw := strings.TrimSpace(os.Getenv("HARPIA_LLM_BLOCKED_PROVIDERS"))
	if raw == "" {
		return out
	}
	for _, item := range strings.Split(raw, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

// ErrPlatformNotConfigured signals to callers that no platform fallback exists
// for the requested provider.
var ErrPlatformNotConfigured = errors.New("no platform key configured for provider")
