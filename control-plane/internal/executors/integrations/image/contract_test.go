package image_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
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

// newImageArtifactStore seeds an in-memory artifact store for the composable
// LinkedInPost → LinkedInPost image enrichment contract.
func newImageArtifactStore() *runtime.ExecutorArtifactStoreAdapter {
	return runtime.NewExecutorArtifactStore(
		&contracttest.MemoryArtifactRepo{
			Types: map[string]*artifacts.ArtifactType{
				artifacts.TypeKeyLinkedInPost: {
					ID:  uuid.MustParse("66666666-6666-6666-6666-666666666666"),
					Key: artifacts.TypeKeyLinkedInPost,
				},
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

func TestImageHandlerEnrichesPinnedLinkedInPost(t *testing.T) {
	store := newImageArtifactStore()
	handler := image.NewHandler(store, noopResolver)
	tenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	postPayload, err := protojson.Marshal(&artifactsv1.LinkedInPost{
		Text:     &artifactsv1.LinkedInPostDraft{Text: "A practical update for operators.", Hook: "Practical update", Hashtags: []string{"harpia"}},
		Carousel: &artifactsv1.CarouselDraft{Title: "Existing carousel", Slides: []*artifactsv1.CarouselSlide{{Heading: "One", Body: "Preserved"}}},
	})
	if err != nil {
		t.Fatalf("marshal post: %v", err)
	}
	postRef, err := store.CreateValidatedPayloadRef(context.Background(), runtime.CreateArtifactRequest{
		TenantID: tenantID, OutputArtifactTypeKey: artifacts.TypeKeyLinkedInPost, StepExecutionID: "author-step", Payload: postPayload,
	})
	if err != nil {
		t.Fatalf("create authored post: %v", err)
	}

	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID: tenantID, StepExecutionID: "image-step", OutputArtifactTypeKey: artifacts.TypeKeyLinkedInPost,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: postRef.ArtifactTypeKey, ArtifactID: postRef.ArtifactID,
			ArtifactVersionID: postRef.ArtifactVersionID, ContentHash: postRef.ContentHash,
		}},
		Installation: runtime.InstallationSnapshot{ExecutorSKUKey: executors.SKUImageAssetGenerator, ConfigJSON: json.RawMessage(`{"provider":"noop"}`)},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusCompleted {
		t.Fatalf("Status = %q, want completed (%s)", result.Status, result.Error)
	}
	if result.OutputArtifactID != postRef.ArtifactID || result.OutputArtifactVersionID == postRef.ArtifactVersionID {
		t.Fatalf("output ref = %s/%s, want same artifact with a new version", result.OutputArtifactID, result.OutputArtifactVersionID)
	}
	enrichedPayload, err := store.LoadPinnedPayload(context.Background(), tenantID, runtime.VersionedArtifactRef{
		ArtifactID: result.OutputArtifactID, ArtifactVersionID: result.OutputArtifactVersionID,
		ArtifactTypeKey: result.OutputArtifactTypeKey, ContentHash: result.OutputContentHash,
	})
	if err != nil {
		t.Fatalf("load enriched post: %v", err)
	}
	enriched := &artifactsv1.LinkedInPost{}
	if err := protojson.Unmarshal(enrichedPayload, enriched); err != nil {
		t.Fatalf("unmarshal enriched post: %v", err)
	}
	if enriched.GetText().GetText() != "A practical update for operators." || enriched.GetCarousel().GetTitle() != "Existing carousel" {
		t.Fatalf("enrichment did not preserve accepted post content: %#v", enriched)
	}
	if len(enriched.GetImages()) != 1 || enriched.GetImages()[0].GetArtifactTypeKey() != artifacts.TypeKeyImageAsset {
		t.Fatalf("images = %#v, want one ImageAsset ref", enriched.GetImages())
	}
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
