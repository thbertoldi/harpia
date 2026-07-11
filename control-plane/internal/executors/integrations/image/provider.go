package image

import "context"

// ImageProvider is the port for image generation backends. New providers
// (Stability AI, etc.) implement this interface and register in the handler's
// provider resolver without changes to the handler's Execute logic.
type ImageProvider interface {
	// Generate produces image bytes for the given prompt. Implementations MUST
	// return a *ProviderError for transient failures (rate limit, upstream
	// outage, network) so the handler can map them to a retryable Temporal error.
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
}

// GenerateRequest carries the per-call generation parameters. Model, Size, and
// Quality fall back to the installation defaults when empty.
type GenerateRequest struct {
	Prompt  string
	Model   string
	Size    string
	Quality string
}

// GenerateResult is the generated image payload plus the metadata recorded on
// the ImageAsset artifact.
type GenerateResult struct {
	ImageBytes []byte
	MimeType   string
	Width      int32
	Height     int32
}

// ProviderResolver selects a configured provider for an installation. The
// handler invokes it at execution time so the OpenAI adapter can resolve the
// API key from the worker environment lazily.
type ProviderResolver func(cfg InstallationConfig) (ImageProvider, error)
