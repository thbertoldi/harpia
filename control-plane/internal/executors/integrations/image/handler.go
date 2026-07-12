package image

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/catalog"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

// Handler satisfies the image-asset-generator integration SKU contract: it
// reads a versioned LinkedInPost, creates an ImageAsset, and returns a new
// version of that same post with the image ArtifactRef appended.
type Handler struct {
	artifacts runtime.ExecutorArtifactStore
	resolver  ProviderResolver
}

// NewHandler constructs an image handler. resolver selects the provider for an
// installation at execution time; pass DefaultProviderResolver for production.
func NewHandler(artifacts runtime.ExecutorArtifactStore, resolver ProviderResolver) *Handler {
	return &Handler{artifacts: artifacts, resolver: resolver}
}

func (h *Handler) SKUKey() string {
	return catalog.SKUImageAssetGenerator
}

func (h *Handler) Execute(ctx context.Context, req runtime.IntegrationExecutionRequest) (runtime.IntegrationExecutionResult, error) {
	if h == nil || h.artifacts == nil || h.resolver == nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("image integration handler is not configured")
	}

	config, err := ParseInstallationConfig(req.Installation.ConfigJSON)
	if err != nil {
		return failedResult(err), nil
	}
	provider, err := h.resolver(config)
	if err != nil {
		return failedResult(err), nil
	}

	postRef, err := linkedInPostInputRef(req.InputArtifacts)
	if err != nil {
		return failedResult(err), nil
	}
	postPayload, err := h.artifacts.LoadPayloadForType(ctx, req.TenantID, req.InputArtifacts, artifacts.TypeKeyLinkedInPost)
	if err != nil {
		return failedResult(err), nil
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyLinkedInPost, postPayload); err != nil {
		return failedResult(err), nil
	}

	post := &artifactsv1.LinkedInPost{}
	if err := protojson.Unmarshal(postPayload, post); err != nil {
		return failedResult(fmt.Errorf("%w: parse LinkedIn post artifact: %v", ErrInvalidInput, err)), nil
	}
	prompt, err := derivePrompt(post)
	if err != nil {
		return failedResult(err), nil
	}

	result, err := provider.Generate(ctx, GenerateRequest{
		Prompt:  prompt,
		Model:   config.Model,
		Size:    config.DefaultSize,
		Quality: config.DefaultQuality,
	})
	if err != nil {
		var providerErr *ProviderError
		if errors.As(err, &providerErr) {
			return runtime.IntegrationExecutionResult{}, runtime.NewRetryableError(
				runtime.ErrCodeImageGeneration,
				err.Error(),
				err,
			)
		}
		return failedResult(err), nil
	}

	payload, err := buildImageAssetPayload(prompt, result, deriveAltText(prompt))
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("marshal image asset payload: %w", err)
	}

	imageRef, err := h.artifacts.CreateValidatedPayloadRef(ctx, runtime.CreateArtifactRequest{
		TenantID:              req.TenantID,
		OutputArtifactTypeKey: artifacts.TypeKeyImageAsset,
		PlanExecutionID:       req.PlanExecutionID,
		StepExecutionID:       req.StepExecutionID,
		Payload:               payload,
	})
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("create image asset artifact: %w", err)
	}
	post.Images = append(post.Images, &artifactsv1.ArtifactRef{
		ArtifactId:        imageRef.ArtifactID,
		ArtifactVersionId: imageRef.ArtifactVersionID,
		ArtifactTypeKey:   imageRef.ArtifactTypeKey,
		ContentHash:       imageRef.ContentHash,
	})
	enrichedPayload, err := protojson.Marshal(post)
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("marshal enriched LinkedIn post: %w", err)
	}
	enrichedRef, err := h.artifacts.CreateValidatedVersionPayload(ctx, runtime.CreateArtifactVersionRequest{
		TenantID:                req.TenantID,
		ArtifactID:              postRef.ArtifactID,
		ExpectedContentHash:     postRef.ContentHash,
		SourceArtifactVersionID: postRef.ArtifactVersionID,
		OutputArtifactTypeKey:   artifacts.TypeKeyLinkedInPost,
		PlanExecutionID:         req.PlanExecutionID,
		StepExecutionID:         req.StepExecutionID,
		EditSummary:             "Generated image enrichment",
		Payload:                 enrichedPayload,
	})
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("create enriched LinkedIn post version: %w", err)
	}

	return runtime.IntegrationExecutionResult{
		Status:                  runtime.IntegrationStatusCompleted,
		OutputArtifactID:        enrichedRef.ArtifactID,
		OutputArtifactVersionID: enrichedRef.ArtifactVersionID,
		OutputArtifactTypeKey:   enrichedRef.ArtifactTypeKey,
		OutputContentHash:       enrichedRef.ContentHash,
	}, nil
}

// derivePrompt extracts an image-generation prompt from the composed post.
func derivePrompt(post *artifactsv1.LinkedInPost) (string, error) {
	if post == nil || post.Text == nil {
		return "", fmt.Errorf("%w: LinkedIn post text is required", ErrInvalidInput)
	}
	if text := strings.TrimSpace(post.Text.Text); text != "" {
		return text, nil
	}
	if hook := strings.TrimSpace(post.Text.Hook); hook != "" {
		return hook, nil
	}
	return "", fmt.Errorf("%w: LinkedIn post text is empty; cannot derive prompt", ErrInvalidInput)
}

func linkedInPostInputRef(refs []runtime.InputArtifactRef) (runtime.VersionedArtifactRef, error) {
	for _, ref := range refs {
		if ref.ArtifactTypeKey != artifacts.TypeKeyLinkedInPost {
			continue
		}
		if strings.TrimSpace(ref.ArtifactID) == "" || strings.TrimSpace(ref.ArtifactVersionID) == "" || strings.TrimSpace(ref.ContentHash) == "" {
			return runtime.VersionedArtifactRef{}, fmt.Errorf("%w: LinkedIn post input must be version-pinned", ErrInvalidInput)
		}
		return runtime.VersionedArtifactRef{
			ArtifactID: ref.ArtifactID, ArtifactVersionID: ref.ArtifactVersionID,
			ArtifactTypeKey: ref.ArtifactTypeKey, ContentHash: ref.ContentHash,
		}, nil
	}
	return runtime.VersionedArtifactRef{}, fmt.Errorf("%w: missing LinkedIn post input", ErrInvalidInput)
}

// deriveAltText produces a short accessible label for the generated image.
func deriveAltText(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "Generated image"
	}
	return truncForLabel(trimmed, 120)
}

// buildImageAssetPayload composes the stored ImageAsset payload JSON. It carries
// the proto metadata fields plus an image_base64 rendering hint (and alt_text in
// the snake-case shape the preview heuristic reads) so BuildPreview can render
// the image without PayloadStore access. protojson validation tolerates the
// rendering hint via DiscardUnknown (see artifacts.ValidatePayload).
func buildImageAssetPayload(prompt string, result GenerateResult, altText string) ([]byte, error) {
	if len(result.ImageBytes) == 0 {
		return nil, fmt.Errorf("%w: provider returned no image bytes", ErrInvalidInput)
	}
	mimeType := strings.TrimSpace(result.MimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}

	asset := &artifactsv1.ImageAsset{
		Prompt:   prompt,
		MimeType: mimeType,
		Width:    result.Width,
		Height:   result.Height,
		AltText:  altText,
	}
	encoded, err := protojson.Marshal(asset)
	if err != nil {
		return nil, fmt.Errorf("marshal image asset proto: %w", err)
	}

	var merged map[string]any
	if err := json.Unmarshal(encoded, &merged); err != nil {
		return nil, fmt.Errorf("decode image asset proto json: %w", err)
	}
	// Rendering hints consumed by the preview layer. These are intentionally NOT
	// proto fields; validation discards them. They let BuildPreview inline the
	// image without a separate PayloadStore round-trip.
	merged["image_base64"] = encodeBase64(result.ImageBytes)
	merged["image_url"] = ""

	payload, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode image asset payload: %w", err)
	}
	return payload, nil
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func failedResult(err error) runtime.IntegrationExecutionResult {
	message := "integration failed"
	if err != nil {
		message = err.Error()
	}
	return runtime.IntegrationExecutionResult{
		Status: runtime.IntegrationStatusFailed,
		Error:  message,
	}
}
