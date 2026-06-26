import { beforeEach, describe, expect, it } from "vitest";
import { MockLLMConfigClient } from "./llm-config-client.mock";
import {
  getLLMConfigClient,
  resetLLMConfigClient,
  setLLMConfigClient,
} from "./llm-config-client";

describe("MockLLMConfigClient", () => {
  let client: MockLLMConfigClient;

  beforeEach(() => {
    client = new MockLLMConfigClient();
  });

  describe("getLLMProviderConfigs", () => {
    it("returns all four providers with has_key=false by default", async () => {
      const res = await client.getLLMProviderConfigs({});
      expect(res.configs.map((cfg) => cfg.provider).sort()).toEqual([
        "anthropic",
        "deepseek",
        "ollama",
        "openai",
      ]);
      for (const cfg of res.configs) {
        expect(cfg.has_key).toBe(false);
        expect(cfg.managed_by).toBe("platform");
        expect(cfg.last_rotated_at).toBeNull();
      }
    });

    it("never includes an api_key field in returned configs", async () => {
      await client.setLLMProviderConfig({
        provider: "anthropic",
        api_key: "sk-ant-secret-key",
      });
      const res = await client.getLLMProviderConfigs({});
      const cfg = res.configs.find((c) => c.provider === "anthropic");
      expect(cfg).toBeDefined();
      expect(cfg).not.toHaveProperty("api_key");
      expect(cfg).not.toHaveProperty("encrypted_api_key");
    });
  });

  describe("setLLMProviderConfig", () => {
    it("sets has_key=true and managed_by=tenant_self after save", async () => {
      const res = await client.setLLMProviderConfig({
        provider: "anthropic",
        api_key: "sk-ant-test",
      });
      expect(res.config.has_key).toBe(true);
      expect(res.config.managed_by).toBe("tenant_self");
      expect(res.config.last_rotated_at).not.toBeNull();
    });

    it("records last_rotated_at as a valid ISO timestamp", async () => {
      const before = new Date().toISOString();
      const res = await client.setLLMProviderConfig({
        provider: "openai",
        api_key: "sk-openai-test",
      });
      const after = new Date().toISOString();
      expect(res.config.last_rotated_at).not.toBeNull();
      expect(res.config.last_rotated_at! >= before).toBe(true);
      expect(res.config.last_rotated_at! <= after).toBe(true);
    });

    it("stores default_model and allowed_models from request", async () => {
      const res = await client.setLLMProviderConfig({
        provider: "openai",
        api_key: "sk-openai-test",
        default_model: "gpt-4o-mini",
        allowed_models: ["gpt-4o", "gpt-4o-mini"],
      });
      expect(res.config.default_model).toBe("gpt-4o-mini");
      expect(res.config.allowed_models).toEqual(["gpt-4o", "gpt-4o-mini"]);
    });

    it("stores DeepSeek default model and allowed models", async () => {
      const res = await client.setLLMProviderConfig({
        provider: "deepseek",
        api_key: "ds-test",
        default_model: "deepseek-v4-flash",
        allowed_models: ["deepseek-v4-flash", "deepseek-v4-pro"],
      });
      expect(res.config.provider).toBe("deepseek");
      expect(res.config.default_model).toBe("deepseek-v4-flash");
      expect(res.config.allowed_models).toEqual([
        "deepseek-v4-flash",
        "deepseek-v4-pro",
      ]);
      expect(JSON.stringify(res)).not.toContain("ds-test");
    });

    it("throws when api_key is empty", async () => {
      await expect(
        client.setLLMProviderConfig({ provider: "anthropic", api_key: "" }),
      ).rejects.toThrow();
    });

    it("never echoes the api_key in the response", async () => {
      const res = await client.setLLMProviderConfig({
        provider: "anthropic",
        api_key: "sk-ant-secret",
      });
      const jsonStr = JSON.stringify(res);
      expect(jsonStr).not.toContain("sk-ant-secret");
    });
  });

  describe("deleteLLMProviderConfig", () => {
    it("resets provider to has_key=false, managed_by=platform after delete", async () => {
      await client.setLLMProviderConfig({
        provider: "anthropic",
        api_key: "sk-ant-test",
      });

      await client.deleteLLMProviderConfig({ provider: "anthropic" });

      const res = await client.getLLMProviderConfigs({});
      const cfg = res.configs.find((c) => c.provider === "anthropic");
      expect(cfg?.has_key).toBe(false);
      expect(cfg?.managed_by).toBe("platform");
      expect(cfg?.last_rotated_at).toBeNull();
    });

    it("does not affect other providers when deleting one", async () => {
      await client.setLLMProviderConfig({
        provider: "openai",
        api_key: "sk-openai-test",
      });
      await client.setLLMProviderConfig({
        provider: "anthropic",
        api_key: "sk-ant-test",
      });

      await client.deleteLLMProviderConfig({ provider: "anthropic" });

      const res = await client.getLLMProviderConfigs({});
      const openai = res.configs.find((c) => c.provider === "openai");
      expect(openai?.has_key).toBe(true);
    });
  });

  describe("rotateLLMProviderConfigKey", () => {
    it("updates last_rotated_at and keeps has_key=true", async () => {
      await client.setLLMProviderConfig({
        provider: "anthropic",
        api_key: "sk-ant-old",
      });

      const rotateRes = await client.rotateLLMProviderConfigKey({
        provider: "anthropic",
        new_api_key: "sk-ant-new",
      });

      expect(rotateRes.config.has_key).toBe(true);
      expect(rotateRes.config.managed_by).toBe("tenant_self");
      expect(rotateRes.config.last_rotated_at).not.toBeNull();
    });

    it("throws when new_api_key is empty", async () => {
      await expect(
        client.rotateLLMProviderConfigKey({
          provider: "anthropic",
          new_api_key: "",
        }),
      ).rejects.toThrow();
    });

    it("never echoes the new key in the response", async () => {
      const res = await client.rotateLLMProviderConfigKey({
        provider: "anthropic",
        new_api_key: "sk-ant-secret-rotation",
      });
      const jsonStr = JSON.stringify(res);
      expect(jsonStr).not.toContain("sk-ant-secret-rotation");
    });
  });

  describe("constructor overrides", () => {
    it("accepts initial state with has_key=true for a provider", async () => {
      const seeded = new MockLLMConfigClient({
        anthropic: {
          has_key: true,
          managed_by: "tenant_self",
          last_rotated_at: "2026-06-01T00:00:00.000Z",
        },
      });
      const res = await seeded.getLLMProviderConfigs({});
      const cfg = res.configs.find((c) => c.provider === "anthropic");
      expect(cfg?.has_key).toBe(true);
      expect(cfg?.managed_by).toBe("tenant_self");
    });
  });
});

describe("getLLMConfigClient factory", () => {
  beforeEach(() => {
    resetLLMConfigClient();
  });

  it("returns the same singleton instance on repeated calls", () => {
    const a = getLLMConfigClient();
    const b = getLLMConfigClient();
    expect(a).toBe(b);
  });

  it("uses injected client when setLLMConfigClient is called", async () => {
    const custom = new MockLLMConfigClient({
      anthropic: { has_key: true, managed_by: "tenant_self" },
    });
    setLLMConfigClient(custom);
    const client = getLLMConfigClient();
    const res = await client.getLLMProviderConfigs({});
    const anthropic = res.configs.find((c) => c.provider === "anthropic");
    expect(anthropic?.has_key).toBe(true);
  });
});
