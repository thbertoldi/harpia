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
// reads a TextDraft input, derives a prompt, calls the configured image
// provider, stores the image bytes in the PayloadStore (inline base64 in the
// ImageAsset payload for preview rendering), and creates an ImageAsset artifact.
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

	draftPayload, err := h.artifacts.LoadPayloadForType(ctx, req.TenantID, req.InputArtifacts, artifacts.TypeKeyTextDraft)
	if err != nil {
		return failedResult(err), nil
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyTextDraft, draftPayload); err != nil {
		return failedResult(err), nil
	}

	prompt, err := derivePrompt(draftPayload)
	if err != nil {
		return failedResult(err), nil
	}

	provider, err := h.resolver(config)
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

	outputArtifactID, err := h.artifacts.CreateValidatedPayload(ctx, runtime.CreateArtifactRequest{
		TenantID:              req.TenantID,
		OutputArtifactTypeKey: req.OutputArtifactTypeKey,
		PlanExecutionID:       req.PlanExecutionID,
		StepExecutionID:       req.StepExecutionID,
		Payload:               payload,
	})
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("create image asset artifact: %w", err)
	}

	return runtime.IntegrationExecutionResult{
		Status:           runtime.IntegrationStatusCompleted,
		OutputArtifactID: outputArtifactID,
	}, nil
}

// derivePrompt extracts the image generation prompt from a TextDraft payload.
// The draft body is the prompt; the title is used as a fallback so a
// title-only draft still produces an image.
func derivePrompt(draftPayload []byte) (string, error) {
	draft := &artifactsv1.TextDraft{}
	if err := protojson.Unmarshal(draftPayload, draft); err != nil {
		return "", fmt.Errorf("%w: parse text draft artifact: %v", ErrInvalidInput, err)
	}
	if body := strings.TrimSpace(draft.Body); body != "" {
		return body, nil
	}
	if title := strings.TrimSpace(draft.Title); title != "" {
		return title, nil
	}
	return "", fmt.Errorf("%w: text draft body is empty; cannot derive prompt", ErrInvalidInput)
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
