/**
 * In-memory mock implementation of LLMConfigClient.
 * Used for development and tests until #33 merges.
 *
 * Keyed by provider (one config per provider per simulated tenant).
 * The mock never stores or returns raw key values — it only records
 * that a key is present (has_key = true) and updates last_rotated_at.
 */

import type {
  DeleteLLMProviderConfigRequest,
  DeleteLLMProviderConfigResponse,
  GetLLMProviderConfigsRequest,
  GetLLMProviderConfigsResponse,
  LLMConfigClient,
  LLMProvider,
  LLMProviderConfigMeta,
  RotateLLMProviderConfigKeyRequest,
  RotateLLMProviderConfigKeyResponse,
  SetLLMProviderConfigRequest,
  SetLLMProviderConfigResponse,
} from "./llm-config-client";

const DEFAULT_CONFIGS: Record<LLMProvider, LLMProviderConfigMeta> = {
  anthropic: {
    provider: "anthropic",
    default_model: "claude-3-5-sonnet-20241022",
    allowed_models: [],
    has_key: false,
    managed_by: "platform",
    last_rotated_at: null,
  },
  deepseek: {
    provider: "deepseek",
    default_model: "deepseek-v4-flash",
    allowed_models: [],
    has_key: false,
    managed_by: "platform",
    last_rotated_at: null,
  },
  openai: {
    provider: "openai",
    default_model: "gpt-4o",
    allowed_models: [],
    has_key: false,
    managed_by: "platform",
    last_rotated_at: null,
  },
  ollama: {
    provider: "ollama",
    default_model: "llama3:8b",
    allowed_models: [],
    has_key: false,
    managed_by: "platform",
    last_rotated_at: null,
  },
};

function now(): string {
  return new Date().toISOString();
}

function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj)) as T;
}

export class MockLLMConfigClient implements LLMConfigClient {
  private store: Map<LLMProvider, LLMProviderConfigMeta>;

  constructor(
    initial?: Partial<Record<LLMProvider, Partial<LLMProviderConfigMeta>>>,
  ) {
    this.store = new Map();
    const providers: LLMProvider[] = [
      "anthropic",
      "deepseek",
      "ollama",
      "openai",
    ];
    for (const p of providers) {
      const defaults = deepClone(DEFAULT_CONFIGS[p]);
      const overrides = initial?.[p] ?? {};
      this.store.set(p, { ...defaults, ...overrides });
    }
  }

  async getLLMProviderConfigs(
    req: GetLLMProviderConfigsRequest,
  ): Promise<GetLLMProviderConfigsResponse> {
    void req;
    const configs = Array.from(this.store.values()).map((c) => deepClone(c));
    return { configs };
  }

  async setLLMProviderConfig(
    req: SetLLMProviderConfigRequest,
  ): Promise<SetLLMProviderConfigResponse> {
    if (!req.api_key || req.api_key.trim() === "") {
      throw new Error("api_key is required");
    }

    const existing =
      this.store.get(req.provider) ?? deepClone(DEFAULT_CONFIGS[req.provider]);
    const updated: LLMProviderConfigMeta = {
      ...existing,
      provider: req.provider,
      has_key: true,
      managed_by: "tenant_self",
      last_rotated_at: now(),
      default_model: req.default_model ?? existing.default_model,
      allowed_models: req.allowed_models ?? existing.allowed_models,
    };

    this.store.set(req.provider, updated);
    return { config: deepClone(updated) };
  }

  async deleteLLMProviderConfig(
    req: DeleteLLMProviderConfigRequest,
  ): Promise<DeleteLLMProviderConfigResponse> {
    const reset = deepClone(DEFAULT_CONFIGS[req.provider]);
    this.store.set(req.provider, reset);
    return {};
  }

  async rotateLLMProviderConfigKey(
    req: RotateLLMProviderConfigKeyRequest,
  ): Promise<RotateLLMProviderConfigKeyResponse> {
    if (!req.new_api_key || req.new_api_key.trim() === "") {
      throw new Error("new_api_key is required");
    }

    const existing =
      this.store.get(req.provider) ?? deepClone(DEFAULT_CONFIGS[req.provider]);
    const updated: LLMProviderConfigMeta = {
      ...existing,
      has_key: true,
      managed_by: "tenant_self",
      last_rotated_at: now(),
    };

    this.store.set(req.provider, updated);
    return { config: deepClone(updated) };
  }
}
