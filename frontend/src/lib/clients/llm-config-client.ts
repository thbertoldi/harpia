/**
 * LLM Provider Config client interface and factory.
 *
 * Mirrors the RPC contract defined in the #33 design note
 * (docs/notes/2026-06-16-tenant-llm-config-design.md, section 5).
 */

import { ConnectLLMConfigClient } from "./llm-config-client.connect";

// ---------------------------------------------------------------------------
// Stable error codes from the design note (section 6 — fallback chain)
// ---------------------------------------------------------------------------

export type LLMProviderErrorCode =
  | "LLM_NO_PROVIDER_CONFIGURED"
  | "LLM_PROVIDER_BLOCKED_BY_PLATFORM"
  | "LLM_KEY_DECRYPTION_FAILED"
  | "UNKNOWN";

// ---------------------------------------------------------------------------
// Domain types matching the proto contract (section 5)
// ---------------------------------------------------------------------------

export type LLMProvider = "anthropic" | "openai" | "ollama";

/**
 * `managed_by` mirrors the design note's enum.
 * `proxy_virtual` is reserved for future use.
 */
export type ManagedBy = "platform" | "tenant_self" | "proxy_virtual";

/**
 * Metadata returned by GetLLMProviderConfigs.
 * Intentionally never includes encrypted_api_key or any decrypted form.
 */
export interface LLMProviderConfigMeta {
  provider: LLMProvider;
  default_model: string | null;
  allowed_models: string[];
  has_key: boolean;
  managed_by: ManagedBy;
  last_rotated_at: string | null; // ISO-8601 timestamp or null
}

// ---------------------------------------------------------------------------
// Request / response shapes
// ---------------------------------------------------------------------------

export interface SetLLMProviderConfigRequest {
  provider: LLMProvider;
  /** Write-only. Never echoed back. */
  api_key: string;
  default_model?: string | null;
  allowed_models?: string[];
}

export interface SetLLMProviderConfigResponse {
  config: LLMProviderConfigMeta;
}

export type GetLLMProviderConfigsRequest = Record<string, never>;

export interface GetLLMProviderConfigsResponse {
  configs: LLMProviderConfigMeta[];
}

export interface DeleteLLMProviderConfigRequest {
  provider: LLMProvider;
}

export type DeleteLLMProviderConfigResponse = Record<string, never>;

export interface RotateLLMProviderConfigKeyRequest {
  provider: LLMProvider;
  /** Write-only replacement key. Never echoed back. */
  new_api_key: string;
}

export interface RotateLLMProviderConfigKeyResponse {
  config: LLMProviderConfigMeta;
}

// ---------------------------------------------------------------------------
// Client interface
// ---------------------------------------------------------------------------

export interface LLMConfigClient {
  getLLMProviderConfigs(
    req: GetLLMProviderConfigsRequest,
  ): Promise<GetLLMProviderConfigsResponse>;

  setLLMProviderConfig(
    req: SetLLMProviderConfigRequest,
  ): Promise<SetLLMProviderConfigResponse>;

  deleteLLMProviderConfig(
    req: DeleteLLMProviderConfigRequest,
  ): Promise<DeleteLLMProviderConfigResponse>;

  rotateLLMProviderConfigKey(
    req: RotateLLMProviderConfigKeyRequest,
  ): Promise<RotateLLMProviderConfigKeyResponse>;
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

let _client: LLMConfigClient | null = null;

export function getLLMConfigClient(): LLMConfigClient {
  if (!_client) {
    _client = new ConnectLLMConfigClient();
  }
  return _client;
}

/** Inject a client (used in tests). */
export function setLLMConfigClient(client: LLMConfigClient): void {
  _client = client;
}

export function resetLLMConfigClient(): void {
  _client = null;
}
