# Tenant LLM Config (Issue #33)

This package stores tenant-scoped LLM provider configuration and resolves
credentials for trusted internal callers.

## KEK rotation

- KEKs are loaded at startup from either:
  - env vars: `HARPIA_LLM_KEK_<VERSION>_B64` + `HARPIA_LLM_KEK_ACTIVE`, or
  - mounted directory: `HARPIA_LLM_KEK_MOUNT_DIR` containing version files and an `active` file.
- Each tenant key write generates a new DEK and wraps it with the active KEK version.
- Rotation is forward-only:
  1. Add new version material.
  2. Restart control-plane pods.
  3. Mark new version active.
  4. Keep old versions loaded until all rows referencing them are rotated.

## Adding a provider

1. Ensure provider names are stable lowercase strings (for example `openai`, `anthropic`).
2. Add optional platform fallback key via env:
   `HARPIA_PLATFORM_<PROVIDER_UPPER>_API_KEY`.
3. Optionally block provider globally:
   `HARPIA_LLM_BLOCKED_PROVIDERS=openai,anthropic`.
4. Clients call `SetLLMProviderConfig` with provider metadata and optional `allowed_models`.

## `managed_by` semantics

- `tenant_self`: tenant supplied key (default).
- `platform`: resolved from platform-managed fallback key.
- `proxy_virtual`: reserved compatibility hook for future proxy/virtual credential flows.

`GetLLMProviderConfigs` always returns metadata only. Secret material is never
included in public responses.
