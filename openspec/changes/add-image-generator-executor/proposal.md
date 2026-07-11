## Why

The `linkedin-content-studio` plan template already declares an optional `generate-image` step (`optional_capabilities: ["image-generation"]`), and the `image-asset-generator` ExecutorSKU + `ImageAsset` proto exist as scaffolding. But the integration handler, config validator, bootstrap registration, and preview support are all missing — the step can never actually run. Users who opt into image generation get a silent SKIPPED step or a runtime error.

## What Changes

- Implement the `image-asset-generator` integration executor following the established RSS/LinkedIn pattern (`executions/integrations/<sku>/handler.go` + `config.go` + `config_validator.go`).
- Define `InstallationConfig` for image providers: provider selection (e.g., `openai/dall-e`, `stability/sdxl`), API key reference, default model, image dimensions, style presets.
- Register the handler and config validator in `executors/bootstrap/bootstrap.go` (both `NewRuntime` and `DefaultConfigValidators`).
- Add contract tests via `executors/contracttest.Suite`.
- Wire `ImageAsset → image_preview` in `artifacts/preview.go` (presigned URL from PayloadStore or inline-bytes fallback).
- Declare the `image-generation` capability on the `image-asset-generator` SKU compatibility metadata so capability-based team recommendation and step compatibility checks consider it.
- Add i18n keys (`en` + `pt-BR`) for the executor display name and configuration labels.

## Capabilities

### New Capabilities

- `image-generation-executor`: Integration executor that accepts a `TextDraft` artifact (or derived prompt), calls an external image generation API, and produces an `ImageAsset` artifact. Covers installation config, provider abstraction, config validation, and preview rendering.

### Modified Capabilities

(none — the `image-asset-generator` SKU seed and `ImageAsset` proto already exist; this change fills in the handler implementation behind the existing contract)

## Impact

- **Proto**: no changes (existing `ImageAsset` message and `image-asset-generator` SKU key are sufficient).
- **Control-plane Go**: new `executors/integrations/image/` package (handler, config, config_validator, errors); updates to `bootstrap.go` and `catalog.go` capability metadata; preview support in `artifacts/preview.go`.
- **Agent-runtime Python**: no changes (integration executors run in the Go control-plane worker, not the Python agent-runtime).
- **Frontend**: no changes beyond existing `ImageAsset` artifact rendering (preview rendering is backend-driven).
- **Database/migrations**: no schema changes (SKU seed is code-level, existing `executor_installations` table is sufficient).
- **Dependencies**: may add an HTTP client for the image provider API (no heavy SDK; REST calls only).
