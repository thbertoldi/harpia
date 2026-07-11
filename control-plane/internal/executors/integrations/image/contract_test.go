package image_test

import (
	"context"
	"encoding/json"
	"errors"
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
	if err := validator.ValidateConfig(json.RawMessage(`{"provider":"openai","api_key_env":"OPENAI_API_KEY","model":"dall-e-3"}`)); err != nil {
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
	if err := validator.ValidateConfig(json.RawMessage(`{"api_key_env":"OPENAI_API_KEY"}`)); err == nil {
		t.Fatal("expected missing provider to be rejected")
	}
}

func TestImageConfigValidatorRejectsInlineAPIKey(t *testing.T) {
	cases := []string{
		`{"provider":"openai","api_key_env":"OPENAI_API_KEY","api_key":"sk-secret"}`,
		`{"provider":"openai","api_key_env":"OPENAI_API_KEY","key":"sk-secret"}`,
		`{"provider":"openai","api_key_env":"OPENAI_API_KEY","token":"sk-secret"}`,
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
	if err := validator.ValidateConfig(json.RawMessage(`{"provider":"midjourney","api_key_env":"X"}`)); err == nil {
		t.Fatal("expected unknown provider to be rejected")
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
