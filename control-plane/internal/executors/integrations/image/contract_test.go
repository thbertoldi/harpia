package image_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/contracttest"
	"github.com/harpia/control-plane/internal/executors/integrations/image"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

// noopResolver always returns the noop provider, so the contract suite can run
// the full TextDraft → ImageAsset path without external API calls.
func noopResolver(_ image.InstallationConfig) (image.ImageProvider, error) {
	return image.NewNoopProvider(), nil
}

// newImageArtifactStore seeds an in-memory artifact store for the image contract
// tests, registering the TextDraft (input) and ImageAsset (output) types.
func newImageArtifactStore() runtime.ExecutorArtifactStore {
	return runtime.NewExecutorArtifactStore(
		&contracttest.MemoryArtifactRepo{
			Types: map[string]*artifacts.ArtifactType{
				artifacts.TypeKeyImageAsset: {
					ID:  uuid.MustParse("77777777-7777-7777-7777-777777777777"),
					Key: artifacts.TypeKeyImageAsset,
				},
			},
		},
		&contracttest.MemoryPayloadStore{},
	)
}

// validTextDraftLiteral is a schema-valid TextDraft literal the handler derives
// the generation prompt from.
const validTextDraftLiteral = `{"title":"Product launch","body":"A bold illustration of our product launch announcement."}`

func TestImageConfigValidatorAcceptsValidConfig(t *testing.T) {
	validator := image.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{"provider":"openai","model":"dall-e-3","default_size":"1024x1024","default_quality":"standard"}`)); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestImageConfigValidatorAcceptsNoopConfig(t *testing.T) {
	validator := image.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{"provider":"noop"}`)); err != nil {
		t.Fatalf("expected noop config accepted, got error: %v", err)
	}
}

func TestImageConfigValidatorRejectsMissingProvider(t *testing.T) {
	validator := image.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{"model":"dall-e-3"}`)); err == nil {
		t.Fatal("expected missing provider to be rejected")
	}
}

func TestImageConfigValidatorRejectsCredentialFields(t *testing.T) {
	cases := []string{
		`{"provider":"openai","api_key":"sk-secret"}`,
		`{"provider":"openai","key":"sk-secret"}`,
		`{"provider":"openai","api_key_env":"OPENAI_API_KEY"}`,
		`{"provider":"openai","API_KEY":"sk-secret"}`,
	}
	validator := image.NewConfigValidator()
	for i, raw := range cases {
		if err := validator.ValidateConfig(json.RawMessage(raw)); err == nil {
			t.Fatalf("cases[%d]: expected inline API key to be rejected", i)
		}
	}
}

func TestImageConfigValidatorRejectsUnknownProvider(t *testing.T) {
	validator := image.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{"provider":"midjourney"}`)); err == nil {
		t.Fatal("expected unknown provider to be rejected")
	}
}

func TestProviderResolverRequiresServerConfiguredOpenAIKey(t *testing.T) {
	const serverEnv = "HARPIA_TEST_IMAGE_OPENAI_KEY"
	t.Setenv(serverEnv, "")

	resolver := image.NewProviderResolver(map[string]string{
		image.ProviderOpenAI: serverEnv,
	})
	_, err := resolver(image.InstallationConfig{Provider: image.ProviderOpenAI})
	if err == nil {
		t.Fatal("expected missing server API key to fail")
	}
	if _, ok := err.(*image.ProviderError); !ok {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if got := err.Error(); !strings.Contains(got, image.ProviderOpenAI) || strings.Contains(got, serverEnv) {
		t.Fatalf("provider error must name the provider but not the environment variable: %q", got)
	}
}

func TestImageHandlerFailsTerminallyWhenOpenAIServerKeyIsMissing(t *testing.T) {
	const serverEnv = "HARPIA_TEST_IMAGE_OPENAI_KEY"
	t.Setenv(serverEnv, "")

	handler := image.NewHandler(newImageArtifactStore(), image.NewProviderResolver(map[string]string{
		image.ProviderOpenAI: serverEnv,
	}))
	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		StepExecutionID:       "step-missing-openai-key",
		OutputArtifactTypeKey: artifacts.TypeKeyImageAsset,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyTextDraft,
			LiteralJSON:     validTextDraftLiteral,
		}},
		Installation: runtime.InstallationSnapshot{
			ExecutorSKUKey: executors.SKUImageAssetGenerator,
			ConfigJSON:     json.RawMessage(`{"provider":"openai"}`),
		},
	})
	contracttest.RequireTerminalFailure(t, result, err)
	if !strings.Contains(result.Error, image.ProviderOpenAI) || strings.Contains(result.Error, serverEnv) {
		t.Fatalf("terminal provider failure must name openai but not the environment variable: %q", result.Error)
	}
}

func TestImageIntegrationContract(t *testing.T) {
	store := newImageArtifactStore()
	handler := image.NewHandler(store, noopResolver)

	contracttest.Run(t, contracttest.Suite{
		Name:                    executors.SKUImageAssetGenerator,
		Handler:                 handler,
		Store:                   store,
		SKUKey:                  executors.SKUImageAssetGenerator,
		InputArtifactTypeKey:    artifacts.TypeKeyTextDraft,
		OutputArtifactTypeKey:   artifacts.TypeKeyImageAsset,
		ValidInstallationConfig: json.RawMessage(`{"provider":"noop"}`),
		ValidInputLiteralJSON:   validTextDraftLiteral,
		RetryableTrigger: func(ctx context.Context, req runtime.IntegrationExecutionRequest) error {
			deadHandler := image.NewHandler(store, failingResolver(image.NewProviderError("openai", errors.New("upstream rate limited"))))
			_, err := deadHandler.Execute(ctx, req)
			return err
		},
	})
}

// failingResolver is a ProviderResolver that returns a provider whose Generate
// always fails, used to exercise the retryable error mapping path.
func failingResolver(err error) image.ProviderResolver {
	return func(_ image.InstallationConfig) (image.ImageProvider, error) {
		return failingProvider{err: err}, nil
	}
}

type failingProvider struct {
	err error
}

func (p failingProvider) Generate(_ context.Context, _ image.GenerateRequest) (image.GenerateResult, error) {
	return image.GenerateResult{}, p.err
}
