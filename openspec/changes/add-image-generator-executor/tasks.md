## 1. Integration handler + provider port

- [x] 1.1 Create `control-plane/internal/executors/integrations/image/config.go` — define the tenant-facing `InstallationConfig` with `Provider`, `Model`, `DefaultSize`, and `DefaultQuality`. Validate provider/model/size/quality and reject `api_key`, `key`, `api_key_env`, and unsupported config fields.
- [x] 1.2 Create `control-plane/internal/executors/integrations/image/provider.go` — define the `ImageProvider` port interface (`Generate(ctx, GenerateRequest) (GenerateResult, error)`) and the `GenerateRequest`/`GenerateResult` types.
- [x] 1.3 Create `control-plane/internal/executors/integrations/image/dalle.go` — implement `DallEProvider` that calls OpenAI's Images API (`POST /v1/images/generations`) with the prompt, model, size, and quality from the config + request. Parse the response, download the generated image, and return bytes + metadata. Map API errors to `RetryableError` for transient failures.
- [x] 1.4 Create `control-plane/internal/executors/integrations/image/noop.go` — implement `NoopProvider` that generates a simple SVG placeholder (solid color with the prompt text). Returns immediately without network calls.
- [x] 1.5 Create `control-plane/internal/executors/integrations/image/handler.go` — implement `Handler` satisfying `runtime.IntegrationHandler`. `SKUKey()` returns `image-asset-generator`. `Execute()` loads the `TextDraft` input artifact via `ExecutorArtifactStore`, extracts the prompt, selects the provider from config, calls `provider.Generate()`, stores the image bytes via `PayloadStore`, and creates an `ImageAsset` artifact via `ExecutorArtifactStore.CreateValidatedPayload()`.
- [x] 1.6 Create `control-plane/internal/executors/integrations/image/errors.go` — define `ErrInvalidConfig`, `ErrInvalidInput`, and `ProviderError` types, following the `integrations/rss/errors.go` pattern.

**Verification:** `cd control-plane && go build ./internal/executors/integrations/image/...`

## 2. Config validator + bootstrap registration

- [x] 2.1 Create `control-plane/internal/executors/integrations/image/config_validator.go` — implement `ConfigValidator` satisfying `runtime.ConfigValidator`. `SKUKey()` returns `image-asset-generator`. `ValidateConfig()` delegates to `ParseInstallationConfig`.
- [x] 2.2 Register in `control-plane/internal/executors/bootstrap/bootstrap.go`:
  - In `NewRuntime()`: add `image.NewHandler(...)` to the `IntegrationRegistry` constructor and `image.NewConfigValidator()` to the `ConfigValidatorRegistry`.
  - In `DefaultConfigValidators()`: add `image.NewConfigValidator()`.
- [x] 2.3 Update `control-plane/internal/executors/seed.go` — add `image-generation` to the `image-asset-generator` SKU compatibility metadata `Capabilities` field (currently empty).
- [x] 2.4 Add a server-controlled image `ProviderResolver` and configuration map (`openai → OPENAI_API_KEY` by default), inject it into worker bootstrap, and ensure only a server-configured environment string reaches `os.Getenv`. A missing OpenAI credential returns a non-retryable provider error without exposing the environment-variable name.

**Verification:** `cd control-plane && go build ./... && go test ./internal/executors/...`

## 3. Contract tests

- [x] 3.1 Create `control-plane/internal/executors/integrations/image/contract_test.go` — use `contracttest.Suite` following the RSS contract test pattern. Test: valid config acceptance, missing provider rejection, rejection of `api_key`/`key`/`api_key_env`, missing server OpenAI credential without environment-name disclosure, successful generation with noop provider (returns valid `ImageAsset`), and missing input artifact error.

**Verification:** `cd control-plane && go test ./internal/executors/integrations/image/... -v`

## 4. ImageAsset preview support

- [x] 4.1 Add `ImageAsset` case to `control-plane/internal/artifacts/preview.go` — return a presigned URL from the PayloadStore when available. If the PayloadStore does not support presigning or the image is small (< 256KB), return inline base64 data.

**Verification:** `cd control-plane && go test ./internal/artifacts/... -v`

## 5. i18n keys

- [x] 5.1 Update `en` + `pt-BR` keys in `frontend/src/lib/i18n/{en,pt-BR}.json` for the executor display name and credential-free configuration labels (`executors.image-asset-generator.provider`, `.model`, `.default_size`, `.default_quality`); remove obsolete API-environment copy.
- [x] 5.2 Extend `frontend/src/lib/integrations/executor-installations.ts` and `frontend/src/routes/admin/integrations/+page.svelte` with the `image-asset-generator` SKU and image form variant. Save exactly `provider`, `model`, `default_size`, and `default_quality`, with no credential field, and preserve RSS/LinkedIn behavior.
- [x] 5.3 Add frontend unit coverage for image SKU mapping and credential-free config JSON construction.

**Verification:** `cd frontend && bun run lint`

## 6. Smoke test

- [x] 6.1 Acceptance gate: verify G1–G4 with the focused executor and integration-form tests plus whole-surface gates. The existing optional-step binding coverage remains the G5 gate; do not modify its parallel-owned opt-out implementation.
