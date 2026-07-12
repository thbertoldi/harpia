## Context

The `image-asset-generator` ExecutorSKU (`executors/catalog/catalog.go:11`, `executors/seed.go:116-127`) and the `ImageAsset` proto message (`proto/harpia/artifacts/v1/artifacts.proto:127-136`) already exist as scaffolding from the `rich-linkedin-content` change. The `linkedin-content-studio` template declares the `generate-image` step as an optional capability (`optional_capabilities: ["image-generation"]`). However, the integration handler that would actually call an image provider API was deferred (task 3.4, marked DEFERRED).

Two established integration executors serve as the reference pattern: `executors/integrations/rss/` (RSS feed fetcher) and `executors/integrations/linkedin/` (LinkedIn publisher). Both follow the same 5-file structure: `handler.go` (implements `runtime.IntegrationHandler`), `config.go` (parses `InstallationConfig` from `ConfigJSON`), `config_validator.go` (wraps config parsing as a `runtime.ConfigValidator`), `errors.go`, and `contract_test.go`.

The handler runs in the Go control-plane Temporal worker (not the Python agent-runtime), dispatched by `runExecutorActivity` → `RunIntegrationActivity` → `IntegrationRegistry.Run` → `handler.Execute`. The handler receives an `IntegrationExecutionRequest` containing the input artifact payload and the installation's `ConfigJSON`; it returns an `IntegrationExecutionResult` with the output artifact ID.

## Goals / Non-Goals

**Goals:**
- Implement the `image-asset-generator` integration handler so the `generate-image` step can actually produce images.
- Support at least one image provider (OpenAI DALL-E via Images API) as the initial adapter, with a clean port interface for adding more.
- Validate installation config (provider, model, dimensions) and prevent tenant credential references.
- Register in bootstrap so the executor is live.
- Support `ImageAsset` preview rendering (presigned URL or inline fallback).
- Declare the `image-generation` capability on the SKU compatibility metadata.

**Non-Goals:**
- Multiple image providers at launch (start with one; the port abstraction allows adding more later).
- Editing/transforming existing images (generation only).
- Synchronous in-chat image rendering UI (the artifact flows through the standard step execution pipeline).
- Image moderation/safety policy enforcement (the provider API handles this; Harpia does not add its own layer pre-v1).
- Storing generated images in the artifact store beyond the existing PayloadStore (Garage S3) path.

## Decisions

### D1: Integration executor (not agent executor)

The image generator is an `EXECUTOR_KIND_INTEGRATION`, not an agent. Rationale: it is a deterministic API call with typed input (`TextDraft` → prompt) and typed output (`ImageAsset`). No LLM reasoning is needed at execution time — the prompt is derived from the text draft by a deterministic transformation. This matches the RSS and LinkedIn pattern. The SKU seed already declares it as integration with `connection_type: image_provider`.

### D2: Provider port abstraction

Define an `ImageProvider` interface (port) in the handler package:
```go
type ImageProvider interface {
    Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
}
```
The initial adapter (`DallEProvider`) calls OpenAI's Images API (`/v1/images/generations`). A `NoopProvider` serves as the dev/test default (returns a placeholder image). New providers (Stability AI, etc.) add adapters without touching the handler.

### D3: Prompt derivation from TextDraft

The input artifact is `harpia.artifacts.v1.TextDraft`. The handler extracts the draft text and uses it as (or derives a prompt from) the image generation request. The derivation is a simple extraction: the draft's `body` or `summary` field becomes the prompt. A future enhancement could add a prompt-refinement step, but pre-v1 the text draft IS the prompt.

### D4: Config shape

Tenant `InstallationConfig` JSON:
```json
{
  "provider": "openai",
  "model": "dall-e-3",
  "default_size": "1024x1024",
  "default_quality": "standard"
}
```
The tenant never supplies a credential or an environment-variable name. The config validator accepts only these four fields and rejects `api_key`, `key`, `api_key_env`, and other credential-like fields. At execution time, a server-owned `ProviderResolver` maps the selected provider to an environment variable using `config.Config.ImageProviderAPIKeyEnvs`; its default is `openai → OPENAI_API_KEY`, with deployment override through `HARPIA_IMAGE_PROVIDER_API_KEY_ENVS`. The resolver calls `os.Getenv` only with this server-owned mapping. Missing OpenAI credentials produce a non-retryable provider error that names `openai`, never the environment variable.

### D5: Output artifact

The handler creates an `ImageAsset` artifact via `ExecutorArtifactStore.CreateValidatedPayload`. The image bytes are stored in the PayloadStore (Garage S3); the `ImageAsset` proto carries metadata (`prompt`, `mime_type`, `width`, `height`, `alt_text`). The `payload_uri` points to the stored image.

### D6: Preview rendering

Add an `ImageAsset` case to `artifacts/preview.go` that returns a presigned URL from the PayloadStore (if the store supports presigning) or falls back to inline base64 bytes for small images.

## Risks / Trade-offs

- **[API key in environment vs. encrypted store]** → Pre-v1, environment variables via Kubernetes Secrets are acceptable. The provider-to-environment map is server configuration, never tenant data, which keeps both the credential and its lookup name out of `config_json` and Temporal history.
- **[Provider API rate limits / cost]** → Image generation has real per-call cost. The handler propagates provider rate-limit errors as `RetryableError` so Temporal backs off. Budget/credit enforcement is a separate concern (existing budget system applies).
- **[Large image payloads in Temporal history]** → The handler stores image bytes in the PayloadStore (S3) and returns only the artifact ID in the `IntegrationExecutionResult`. No image bytes enter Temporal activity input/output.
- **[Provider outage]** → Propagated as `RetryableError` with the standard Temporal retry policy (3 attempts). After exhaustion, the step fails and the plan execution surfaces the error.
