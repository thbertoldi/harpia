package llm_config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	llmconfigv1 "github.com/harpia/control-plane/gen/harpia/llm_config/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type stubRepo struct {
	listByTenantFn func(context.Context, uuid.UUID) ([]Config, error)
}

func (s *stubRepo) GetByProvider(context.Context, uuid.UUID, string) (*Config, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepo) UpdateMetadata(context.Context, uuid.UUID, string, string, []string, string) (*Config, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepo) Upsert(context.Context, uuid.UUID, *Config) (*Config, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Config, error) {
	if s.listByTenantFn != nil {
		return s.listByTenantFn(ctx, tenantID)
	}
	return nil, nil
}

func (s *stubRepo) Delete(context.Context, uuid.UUID, string) error {
	return errors.New("not implemented")
}

type stubResolver struct {
	resolveFn func(context.Context, uuid.UUID, string, string) (*ResolvedCredential, error)
}

func (s *stubResolver) Resolve(ctx context.Context, tenantID uuid.UUID, provider, requestedModel string) (*ResolvedCredential, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, tenantID, provider, requestedModel)
	}
	return nil, errors.New("not implemented")
}

func withRequestContext(tenantID uuid.UUID, role string) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID:    tenantID,
		TenantAlias: "tenant-a",
		UserID:      "test-user",
		Roles:       []string{role},
		Tenants: []identity.TenantMembership{
			{
				TenantID: tenantID,
				Slug:     "tenant-a",
				Role:     role,
			},
		},
	})
}

func TestSetLLMProviderConfigRequiresAdminRole(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{
		repo:     &stubRepo{},
		resolver: &stubResolver{},
	}

	_, err := handler.SetLLMProviderConfig(
		withRequestContext(tenantID, "viewer"),
		connect.NewRequest(&llmconfigv1.SetLLMProviderConfigRequest{
			TenantId: tenantID.String(),
			Provider: "openai",
			ApiKey:   "sk-test-value",
		}),
	)
	if err == nil {
		t.Fatal("expected permission denied")
	}
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("code = %v, want %v", got, connect.CodePermissionDenied)
	}
}

func TestGetLLMProviderConfigsMetadataOnly(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{
		repo: &stubRepo{
			listByTenantFn: func(_ context.Context, gotTenantID uuid.UUID) ([]Config, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenant_id = %s, want %s", gotTenantID, tenantID)
				}
				return []Config{
					{
						ID:            uuid.New(),
						TenantID:      tenantID,
						Provider:      "openai",
						KEKVersion:    "v1",
						EncryptedDEK:  []byte{1, 2, 3},
						EncryptedKey:  []byte{9, 8, 7},
						DefaultModel:  "gpt-4o-mini",
						AllowedModels: []string{"gpt-4o-mini"},
						ManagedBy:     "tenant_self",
					},
				}, nil
			},
		},
		resolver: &stubResolver{},
	}

	resp, err := handler.GetLLMProviderConfigs(
		withRequestContext(tenantID, AdminRole),
		connect.NewRequest(&llmconfigv1.GetLLMProviderConfigsRequest{
			TenantId: tenantID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("GetLLMProviderConfigs: %v", err)
	}
	if len(resp.Msg.Configs) != 1 {
		t.Fatalf("configs len = %d, want 1", len(resp.Msg.Configs))
	}
	cfg := resp.Msg.Configs[0]
	if !cfg.GetHasKey() {
		t.Fatal("expected has_key=true when encrypted key exists")
	}
	if cfg.GetProvider() != "openai" {
		t.Fatalf("provider = %q, want openai", cfg.GetProvider())
	}
}

func TestResolveLLMProviderForTenantMapsStableErrorCodes(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	tests := []struct {
		name   string
		err    error
		code   connect.Code
		reason string
	}{
		{
			name:   "no provider configured",
			err:    &ResolveError{Reason: ReasonNoProviderConfigured},
			code:   connect.CodeFailedPrecondition,
			reason: ReasonNoProviderConfigured,
		},
		{
			name:   "provider blocked",
			err:    &ResolveError{Reason: ReasonProviderBlockedByPlatform},
			code:   connect.CodePermissionDenied,
			reason: ReasonProviderBlockedByPlatform,
		},
		{
			name:   "decryption failed",
			err:    &ResolveError{Reason: ReasonKeyDecryptionFailed},
			code:   connect.CodeInternal,
			reason: ReasonKeyDecryptionFailed,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &Handler{
				repo: &stubRepo{},
				resolver: &stubResolver{
					resolveFn: func(context.Context, uuid.UUID, string, string) (*ResolvedCredential, error) {
						return nil, tt.err
					},
				},
			}

			_, err := handler.ResolveLLMProviderForTenant(
				withRequestContext(tenantID, AdminRole),
				connect.NewRequest(&llmconfigv1.ResolveLLMProviderForTenantRequest{
					TenantId: tenantID.String(),
					Provider: "openai",
				}),
			)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := connect.CodeOf(err); got != tt.code {
				t.Fatalf("code = %v, want %v", got, tt.code)
			}
			if !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("error %q did not include reason %q", err.Error(), tt.reason)
			}
		})
	}
}
