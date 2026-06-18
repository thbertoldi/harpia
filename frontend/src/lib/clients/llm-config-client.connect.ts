import { createClient } from "@connectrpc/connect";
import { getTenant } from "$lib/auth";
import {
  LLMConfigService,
  ManagedBy as ProtoManagedBy,
  type LLMProviderConfigMetadata,
} from "$lib/gen/harpia/llm_config/v1/llm_config_pb";
import { transport } from "$lib/transport";
import type {
  DeleteLLMProviderConfigRequest,
  DeleteLLMProviderConfigResponse,
  GetLLMProviderConfigsRequest,
  GetLLMProviderConfigsResponse,
  LLMConfigClient,
  LLMProvider,
  LLMProviderConfigMeta,
  ManagedBy,
  RotateLLMProviderConfigKeyRequest,
  RotateLLMProviderConfigKeyResponse,
  SetLLMProviderConfigRequest,
  SetLLMProviderConfigResponse,
} from "./llm-config-client";

const rpcClient = createClient(LLMConfigService, transport);

function requireTenantId(): string {
  const tenantId = getTenant()?.id;
  if (!tenantId) {
    throw new Error("missing tenant context");
  }
  return tenantId;
}

function toDomainManagedBy(value: ProtoManagedBy): ManagedBy {
  switch (value) {
    case ProtoManagedBy.TENANT_SELF:
      return "tenant_self";
    case ProtoManagedBy.PLATFORM:
      return "platform";
    case ProtoManagedBy.PROXY_VIRTUAL:
      return "proxy_virtual";
    default:
      return "platform";
  }
}

function toDomainProvider(provider: string): LLMProvider {
  if (provider === "anthropic" || provider === "openai" || provider === "ollama") {
    return provider;
  }
  throw new Error(`unsupported provider: ${provider}`);
}

function toDomainMeta(meta: LLMProviderConfigMetadata): LLMProviderConfigMeta {
  return {
    provider: toDomainProvider(meta.provider),
    default_model: meta.defaultModel || null,
    allowed_models: [...meta.allowedModels],
    has_key: meta.hasKey,
    managed_by: toDomainManagedBy(meta.managedBy),
    last_rotated_at: meta.lastRotatedAt || null,
  };
}

export class ConnectLLMConfigClient implements LLMConfigClient {
  async getLLMProviderConfigs(
    req: GetLLMProviderConfigsRequest,
  ): Promise<GetLLMProviderConfigsResponse> {
    void req;
    const response = await rpcClient.getLLMProviderConfigs({
      tenantId: requireTenantId(),
    });
    return {
      configs: response.configs.map(toDomainMeta),
    };
  }

  async setLLMProviderConfig(
    req: SetLLMProviderConfigRequest,
  ): Promise<SetLLMProviderConfigResponse> {
    const response = await rpcClient.setLLMProviderConfig({
      tenantId: requireTenantId(),
      provider: req.provider,
      apiKey: req.api_key,
      defaultModel: req.default_model ?? "",
      allowedModels: req.allowed_models ?? [],
      managedBy: ProtoManagedBy.TENANT_SELF,
    });
    if (!response.config) {
      throw new Error("setLLMProviderConfig returned no config");
    }
    return { config: toDomainMeta(response.config) };
  }

  async deleteLLMProviderConfig(
    req: DeleteLLMProviderConfigRequest,
  ): Promise<DeleteLLMProviderConfigResponse> {
    await rpcClient.deleteLLMProviderConfig({
      tenantId: requireTenantId(),
      provider: req.provider,
    });
    return {};
  }

  async rotateLLMProviderConfigKey(
    req: RotateLLMProviderConfigKeyRequest,
  ): Promise<RotateLLMProviderConfigKeyResponse> {
    const response = await rpcClient.rotateLLMProviderConfigKey({
      tenantId: requireTenantId(),
      provider: req.provider,
      newApiKey: req.new_api_key,
    });
    if (!response.config) {
      throw new Error("rotateLLMProviderConfigKey returned no config");
    }
    return { config: toDomainMeta(response.config) };
  }
}
