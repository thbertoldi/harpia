## Why

The `linkedin-content-studio` plan template already declares an optional `generate-image` step (`optional_capabilities: ["image-generation"]`), and the `image-asset-generator` ExecutorSKU + `ImageAsset` proto exist as scaffolding. The integration handler is now implemented, but the admin integration surface does not recognize or configure that SKU, so a tenant cannot register it.

The original installation contract also accepted a tenant-selected `api_key_env`, which the runtime passed directly to `os.Getenv`. That allows a tenant to select an unrelated worker secret and exfiltrate it through the OpenAI request. Credential environment selection must be server-controlled.

## What Changes

- Implement the `image-asset-generator` integration executor following the established RSS/LinkedIn pattern (`executions/integrations/<sku>/handler.go` + `config.go` + `config_validator.go`).
- Define the tenant-facing `InstallationConfig` as only `provider`, `model`, `default_size`, and `default_quality`; reject `api_key`, `key`, and `api_key_env` fields on create/update.
- Resolve provider credentials through a server-controlled provider-to-environment map (default `openai → OPENAI_API_KEY`) supplied by control-plane configuration. The runtime never calls `os.Getenv` with tenant config.
- Register the handler and config validator in `executors/bootstrap/bootstrap.go` (both `NewRuntime` and `DefaultConfigValidators`).
- Add contract tests via `executors/contracttest.Suite`.
- Wire `ImageAsset → image_preview` in `artifacts/preview.go` (presigned URL from PayloadStore or inline-bytes fallback).
- Declare the `image-generation` capability on the `image-asset-generator` SKU compatibility metadata so capability-based team recommendation and step compatibility checks consider it.
- Wire bilingual (`en` + `pt-BR`) executor display and credential-free configuration labels.
- Extend the admin integrations UI to list, create, update, and immediately confirm `image-asset-generator` installations. The image form exposes provider/model/size/quality only; it never renders or returns a credential field.

## Capabilities

### New Capabilities

- `image-generation-executor`: Integration executor that accepts a `TextDraft` artifact (or derived prompt), calls an external image generation API, and produces an `ImageAsset` artifact. Covers installation config, provider abstraction, config validation, and preview rendering.

### Modified Capabilities

(none — the `image-asset-generator` SKU seed and `ImageAsset` proto already exist; this change fills in the handler implementation behind the existing contract)

## Impact

- **Proto**: no changes (existing `ImageAsset` message and `image-asset-generator` SKU key are sufficient).
- **Control-plane Go**: new `executors/integrations/image/` package (handler, config, config_validator, errors); updates to `bootstrap.go` and `catalog.go` capability metadata; preview support in `artifacts/preview.go`.
- **Agent-runtime Python**: no changes (integration executors run in the Go control-plane worker, not the Python agent-runtime).
- **Frontend**: admin integration installation mapping and form gain an image-generator variant, including bilingual labels and an inline save confirmation.
- **Database/migrations**: no schema changes (SKU seed is code-level, existing `executor_installations` table is sufficient).
- **Dependencies**: no heavy provider SDK; the existing REST adapter remains. Control-plane config gains a server-only provider-to-environment map override.
