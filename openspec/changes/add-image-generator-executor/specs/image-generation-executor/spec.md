## ADDED Requirements

### Requirement: Image generation executor handler

The system SHALL provide an integration executor handler for the `image-asset-generator` SKU key that accepts a `TextDraft` input artifact, calls an external image provider API, and produces an `ImageAsset` output artifact.

#### Scenario: Successful image generation

- **WHEN** the `generate-image` step executes with a valid `TextDraft` input and a configured `image-asset-generator` installation
- **THEN** the handler calls the configured image provider API with a prompt derived from the text draft, receives image bytes, stores them in the PayloadStore, and creates an `ImageAsset` artifact with metadata (prompt, mime_type, width, height, alt_text)

#### Scenario: Missing input artifact

- **WHEN** the handler executes without a valid `TextDraft` input artifact payload
- **THEN** the handler returns an error indicating the input artifact is required, and the step execution fails

#### Scenario: Provider API failure

- **WHEN** the image provider API returns an error or is unreachable
- **THEN** the handler returns a `RetryableError` so Temporal retries the activity, and after retry exhaustion the step execution fails with a descriptive error

### Requirement: Image provider port abstraction

The system SHALL define an `ImageProvider` port interface so that multiple image generation providers can be supported without modifying the handler. The initial implementation SHALL include an OpenAI DALL-E adapter and a Noop adapter for development/testing.

#### Scenario: Adding a new provider

- **WHEN** a new image provider adapter is implemented (e.g., Stability AI)
- **THEN** it implements the `ImageProvider` interface and is registered in the handler, without changes to the handler's `Execute` logic or artifact creation path

#### Scenario: Noop provider in development

- **WHEN** no real API key is configured and the provider is set to `noop`
- **THEN** the handler returns a placeholder `ImageAsset` with a generated SVG or solid-color image, allowing the plan to complete without external API calls

### Requirement: Installation config validation

The system SHALL validate the `image-asset-generator` installation config at creation and update time via a registered `ConfigValidator`. The config MUST specify a `provider` and MAY contain only `provider`, `model`, `default_size`, and `default_quality`. Credential values and credential environment names MUST NOT be stored in `config_json`.

#### Scenario: Valid config accepted

- **WHEN** an installation is created with `{"provider": "openai", "model": "dall-e-3", "default_size": "1024x1024", "default_quality": "standard"}`
- **THEN** the config validator accepts it without error

#### Scenario: Missing provider rejected

- **WHEN** an installation is created with `{"model": "dall-e-3"}` (no `provider`)
- **THEN** the config validator rejects it with an error indicating `provider` is required

#### Scenario: Credential fields in config_json rejected

- **WHEN** an installation is created or updated with an `api_key`, `key`, or `api_key_env` field in `config_json`
- **THEN** the config validator rejects it

### Requirement: Server-controlled credential resolution

The system SHALL map image providers to credential environment names from server configuration, defaulting `openai` to `OPENAI_API_KEY`. No tenant-controlled config string may be passed to `os.Getenv`.

#### Scenario: OpenAI server credential missing

- **WHEN** an OpenAI image installation executes and the server-configured OpenAI credential is unset
- **THEN** the step fails with a non-retryable `ProviderError` that names `openai` and does not disclose the environment-variable name

### Requirement: ImageAsset preview rendering

The system SHALL render a preview for `ImageAsset` artifacts so users can see generated images in the chat thread and artifact browser.

#### Scenario: Preview via presigned URL

- **WHEN** an `ImageAsset` artifact is opened in the preview panel and the PayloadStore supports presigned URLs
- **THEN** the system returns a presigned URL that the browser loads directly from the object store

#### Scenario: Preview via inline fallback

- **WHEN** the PayloadStore does not support presigned URLs or the image is below a size threshold
- **THEN** the system returns the image as inline base64-encoded data in the preview response

### Requirement: Capability declaration on SKU

The system SHALL declare the `image-generation` capability on the `image-asset-generator` SKU compatibility metadata so that capability-based team recommendation and step compatibility checks correctly identify it as a candidate for steps requiring `image-generation`.

#### Scenario: Step compatibility match

- **WHEN** a plan step declares `required_capabilities: ["image-generation"]` or `optional_capabilities: ["image-generation"]`
- **THEN** the `image-asset-generator` SKU is included in the list of compatible executors for that step

### Requirement: Image generator installation administration

The admin integrations surface SHALL list `image-asset-generator` alongside the RSS and LinkedIn integration SKUs and save its tenant config as only `provider`, `model`, `default_size`, and `default_quality`.

#### Scenario: Create a noop installation

- **WHEN** an administrator creates an image generator installation with `provider: noop`
- **THEN** the installation saves and appears in the integrations list with no credential input or stored credential field, and the UI immediately confirms the save
