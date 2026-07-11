package llm_config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	llmconfigv1 "github.com/harpia/control-plane/gen/harpia/llm_config/v1"
	"github.com/harpia/control-plane/internal/identity"
	cryptoenv "github.com/harpia/control-plane/internal/llm_config/crypto"
)

// AdminRole is the role string the request-context interceptor emits for
// users with Leader-level tenant admin privileges. Mutating LLM config RPCs
// gate on this role (design §8). Engineer retains read-only metadata access.
const AdminRole = "admin"

// Handler implements the LLMConfigService RPCs.
type Handler struct {
	repo     repository
	keyring  *cryptoenv.Keyring
	resolver resolverService
}

type repository interface {
	GetByProvider(ctx context.Context, tenantID uuid.UUID, provider string) (*Config, error)
	UpdateMetadata(ctx context.Context, tenantID uuid.UUID, provider, defaultModel string, allowedModels []string, managedBy string) (*Config, error)
	Upsert(ctx context.Context, tenantID uuid.UUID, cfg *Config) (*Config, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Config, error)
	Delete(ctx context.Context, tenantID uuid.UUID, provider string) error
}

type resolverService interface {
	Resolve(ctx context.Context, tenantID uuid.UUID, provider, requestedModel string) (*ResolvedCredential, error)
}

// HandlerOptions wires dependencies.
type HandlerOptions struct {
	Repo     *Repository
	Keyring  *cryptoenv.Keyring
	Resolver *Resolver
}

// NewHandler constructs a service handler.
func NewHandler(opts HandlerOptions) (*Handler, error) {
	if opts.Repo == nil {
		return nil, errors.New("repository is required")
	}
	if opts.Keyring == nil {
		return nil, errors.New("keyring is required")
	}
	if opts.Resolver == nil {
		opts.Resolver = NewResolver(opts.Repo, opts.Keyring, NewEnvPlatformKeyStore(), BlockedProvidersFromEnv())
	}
	return &Handler{repo: opts.Repo, keyring: opts.Keyring, resolver: opts.Resolver}, nil
}

// SetLLMProviderConfig writes or updates a tenant's provider configuration.
//
// The raw API key arrives as a write-only field; this handler envelope-
// encrypts it immediately, persists ciphertext, and never logs the plaintext.
// When `api_key` is empty on an existing row, only metadata is updated.
func (h *Handler) SetLLMProviderConfig(ctx context.Context, req *connect.Request[llmconfigv1.SetLLMProviderConfigRequest]) (*connect.Response[llmconfigv1.SetLLMProviderConfigResponse], error) {
	tenantID, err := requireTenantAdmin(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if err := validateProvider(req.Msg.Provider); err != nil {
		return nil, err
	}
	provider, err := normalizeProvider(req.Msg.Provider)
	if err != nil {
		return nil, err
	}

	managedBy := managedByFromProto(req.Msg.ManagedBy)

	if req.Msg.ApiKey == "" {
		if _, err := h.repo.GetByProvider(ctx, tenantID, provider); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("api_key is required for new configurations"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updated, err := h.repo.UpdateMetadata(ctx, tenantID, provider, req.Msg.DefaultModel, req.Msg.AllowedModels, managedBy)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&llmconfigv1.SetLLMProviderConfigResponse{Config: toProtoMetadata(updated)}), nil
	}

	record, err := cryptoenv.EncryptWithKeyring(h.keyring, []byte(req.Msg.ApiKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("encrypt api key: %w", err))
	}

	saved, err := h.repo.Upsert(ctx, tenantID, &Config{
		TenantID:      tenantID,
		Provider:      provider,
		KEKVersion:    record.KEKVersion,
		EncryptedDEK:  record.EncryptedDEK,
		EncryptedKey:  record.EncryptedPayload,
		DefaultModel:  req.Msg.DefaultModel,
		AllowedModels: req.Msg.AllowedModels,
		ManagedBy:     managedBy,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&llmconfigv1.SetLLMProviderConfigResponse{Config: toProtoMetadata(saved)}), nil
}

// GetLLMProviderConfigs returns metadata only. Ciphertext, plaintext, and KEK
// material are never echoed across this boundary.
func (h *Handler) GetLLMProviderConfigs(ctx context.Context, req *connect.Request[llmconfigv1.GetLLMProviderConfigsRequest]) (*connect.Response[llmconfigv1.GetLLMProviderConfigsResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	rows, err := h.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*llmconfigv1.LLMProviderConfigMetadata, 0, len(rows))
	for i := range rows {
		out = append(out, toProtoMetadata(&rows[i]))
	}
	return connect.NewResponse(&llmconfigv1.GetLLMProviderConfigsResponse{Configs: out}), nil
}

// DeleteLLMProviderConfig removes a tenant's provider row.
func (h *Handler) DeleteLLMProviderConfig(ctx context.Context, req *connect.Request[llmconfigv1.DeleteLLMProviderConfigRequest]) (*connect.Response[llmconfigv1.DeleteLLMProviderConfigResponse], error) {
	tenantID, err := requireTenantAdmin(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if err := validateProvider(req.Msg.Provider); err != nil {
		return nil, err
	}
	provider, err := normalizeProvider(req.Msg.Provider)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Delete(ctx, tenantID, provider); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&llmconfigv1.DeleteLLMProviderConfigResponse{}), nil
}

// RotateLLMProviderConfigKey replaces the stored ciphertext with a freshly
// envelope-encrypted bundle. The old DEK is discarded; the new row references
// the active KEK version, naturally pulling rotated rows forward.
func (h *Handler) RotateLLMProviderConfigKey(ctx context.Context, req *connect.Request[llmconfigv1.RotateLLMProviderConfigKeyRequest]) (*connect.Response[llmconfigv1.RotateLLMProviderConfigKeyResponse], error) {
	tenantID, err := requireTenantAdmin(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if err := validateProvider(req.Msg.Provider); err != nil {
		return nil, err
	}
	provider, err := normalizeProvider(req.Msg.Provider)
	if err != nil {
		return nil, err
	}
	if req.Msg.NewApiKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_api_key is required"))
	}
	existing, err := h.repo.GetByProvider(ctx, tenantID, provider)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	record, err := cryptoenv.EncryptWithKeyring(h.keyring, []byte(req.Msg.NewApiKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("encrypt new api key: %w", err))
	}
	saved, err := h.repo.Upsert(ctx, tenantID, &Config{
		TenantID:      tenantID,
		Provider:      provider,
		KEKVersion:    record.KEKVersion,
		EncryptedDEK:  record.EncryptedDEK,
		EncryptedKey:  record.EncryptedPayload,
		DefaultModel:  existing.DefaultModel,
		AllowedModels: existing.AllowedModels,
		ManagedBy:     existing.ManagedBy,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&llmconfigv1.RotateLLMProviderConfigKeyResponse{Config: toProtoMetadata(saved)}), nil
}

// ResolveLLMProviderForTenant is the internal-only resolver entrypoint used by
// trusted runtimes (agent-runtime, Temporal worker).
func (h *Handler) ResolveLLMProviderForTenant(ctx context.Context, req *connect.Request[llmconfigv1.ResolveLLMProviderForTenantRequest]) (*connect.Response[llmconfigv1.ResolveLLMProviderForTenantResponse], error) {
	tenantID, err := requireResolverCaller(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if err := validateProvider(req.Msg.Provider); err != nil {
		return nil, err
	}
	provider, err := normalizeProvider(req.Msg.Provider)
	if err != nil {
		return nil, err
	}
	resolved, err := h.resolver.Resolve(ctx, tenantID, provider, req.Msg.RequestedModel)
	if err != nil {
		return nil, toResolveConnectError(err)
	}
	return connect.NewResponse(&llmconfigv1.ResolveLLMProviderForTenantResponse{
		Credentials: &llmconfigv1.CredentialBundle{
			Provider:      resolved.Provider,
			ApiKey:        resolved.APIKey.Reveal(),
			DefaultModel:  resolved.DefaultModel,
			AllowedModels: resolved.AllowedModels,
			Source:        managedByToProto(string(resolved.Source)),
		},
	}), nil
}

func toResolveConnectError(err error) error {
	var rerr *ResolveError
	if errors.As(err, &rerr) {
		switch rerr.Reason {
		case ReasonNoProviderConfigured:
			return connect.NewError(connect.CodeFailedPrecondition, errors.New(rerr.Error()))
		case ReasonProviderBlockedByPlatform:
			return connect.NewError(connect.CodePermissionDenied, errors.New(rerr.Error()))
		case ReasonKeyDecryptionFailed:
			return connect.NewError(connect.CodeInternal, errors.New(rerr.Error()))
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}

// requireTenantAdmin enforces tenant membership AND admin role.
func requireTenantAdmin(ctx context.Context, requestedTenantID string) (uuid.UUID, error) {
	tenantID, err := identity.RequireTenant(ctx, requestedTenantID)
	if err != nil {
		return uuid.Nil, err
	}
	rc, ok := identity.RequestContextFrom(ctx)
	if !ok {
		return uuid.Nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing request context"))
	}
	if !hasRole(rc.Roles, AdminRole) {
		return uuid.Nil, connect.NewError(connect.CodePermissionDenied, errors.New("tenant.admin permission required"))
	}
	return tenantID, nil
}

// requireResolverCaller enforces that only trusted internal runtimes can
// receive decrypted credential bundles.
func requireResolverCaller(ctx context.Context, requestedTenantID string) (uuid.UUID, error) {
	if !identity.IsInternalServiceCaller(ctx) {
		return uuid.Nil, connect.NewError(
			connect.CodePermissionDenied,
			errors.New("internal service authorization required"),
		)
	}
	return identity.RequireTenant(ctx, requestedTenantID)
}

func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func validateProvider(provider string) error {
	if strings.TrimSpace(provider) == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("provider is required"))
	}
	return nil
}

func toProtoMetadata(c *Config) *llmconfigv1.LLMProviderConfigMetadata {
	if c == nil {
		return nil
	}
	allowed := c.AllowedModels
	if allowed == nil {
		allowed = []string{}
	}
	return &llmconfigv1.LLMProviderConfigMetadata{
		Id:            c.ID.String(),
		TenantId:      c.TenantID.String(),
		Provider:      c.Provider,
		DefaultModel:  c.DefaultModel,
		AllowedModels: allowed,
		HasKey:        len(c.EncryptedKey) > 0,
		ManagedBy:     managedByToProto(c.ManagedBy),
		KekVersion:    c.KEKVersion,
		LastRotatedAt: formatTime(c.LastRotatedAt),
		CreatedAt:     formatTime(c.CreatedAt),
		UpdatedAt:     formatTime(c.UpdatedAt),
	}
}

func managedByToProto(s string) llmconfigv1.ManagedBy {
	switch s {
	case "tenant_self":
		return llmconfigv1.ManagedBy_MANAGED_BY_TENANT_SELF
	case "platform":
		return llmconfigv1.ManagedBy_MANAGED_BY_PLATFORM
	case "proxy_virtual":
		return llmconfigv1.ManagedBy_MANAGED_BY_PROXY_VIRTUAL
	default:
		return llmconfigv1.ManagedBy_MANAGED_BY_UNSPECIFIED
	}
}

func managedByFromProto(p llmconfigv1.ManagedBy) string {
	switch p {
	case llmconfigv1.ManagedBy_MANAGED_BY_TENANT_SELF, llmconfigv1.ManagedBy_MANAGED_BY_UNSPECIFIED:
		return "tenant_self"
	case llmconfigv1.ManagedBy_MANAGED_BY_PLATFORM:
		return "platform"
	case llmconfigv1.ManagedBy_MANAGED_BY_PROXY_VIRTUAL:
		return "proxy_virtual"
	default:
		return "tenant_self"
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
