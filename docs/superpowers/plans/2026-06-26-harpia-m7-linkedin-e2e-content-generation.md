# Harpia M7 LinkedIn E2E Content Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Monday 2026-06-29 demo path where a Platform Engineer configures DeepSeek and grouped news feeds, Ana manually selects the LinkedIn plan, uses Suggest to configure a sports content run, and the plan executes RSS -> newsletter writer -> LinkedIn voice -> publish approval visible in `/inbox`.

**Architecture:** Keep M7 as a real product path through existing plan configuration, executor installation, Temporal workflow, artifact, and inbox surfaces. DeepSeek is added as an OpenAI-compatible LLM provider; RSS installations become many per SKU and grouped in the UI; LinkedIn publish gains an approval-only dry-run mode; Python `RunAgentActivity` becomes the agent bridge that loads and persists artifacts. The plan avoids pricing work and avoids real LinkedIn OAuth for Monday.

**Tech Stack:** Go 1.25 control-plane, Temporal Go SDK, Python 3.12 agent-runtime, ConnectRPC Python/Go generated clients, Svelte 5 frontend, Vitest, Playwright, PostgreSQL migrations already present for executor/installations/config.

---

## File Map

- Create `agent-runtime/src/harpia_agents/llm/providers/deepseek.py`: DeepSeek OpenAI-compatible provider.
- Modify `agent-runtime/src/harpia_agents/llm/pricing.py`: DeepSeek model registry with zero-cost Monday estimates.
- Modify `agent-runtime/src/harpia_agents/llm/registry.py`: include DeepSeek in default model routing.
- Modify `agent-runtime/src/harpia_agents/agents/registry.py`: resolve DeepSeek tenant credentials for M7 agents.
- Modify `agents/newsletter-writer-senior/0.1.0.yaml` and `agents/linkedin-voice-senior/0.1.0.yaml`: default model `deepseek-v4-flash`.
- Test `agent-runtime/tests/llm/test_deepseek_provider.py` and `agent-runtime/tests/test_registry_tenant_llm.py`.
- Modify `frontend/src/lib/clients/llm-config-client.ts`, `.connect.ts`, `.mock.ts`, `.test.ts`: first-class DeepSeek BYOK surface.
- Modify `frontend/src/routes/admin/settings/+page.svelte`: Platform Engineer can save DeepSeek key/model.
- Modify `frontend/src/lib/integrations/executor-installations.ts`: grouped integrations, many RSS feed groups, LinkedIn approval-only mode.
- Modify `frontend/src/routes/admin/integrations/+page.svelte`: grouped integration UI with Add feed group and approval-only LinkedIn.
- Test `frontend/src/lib/integrations/executor-installations.test.ts`.
- Modify `control-plane/internal/executors/integrations/linkedin/config.go`, `handler.go`, `config_validator_test.go`, `config_test.go`, `handler_test.go`, `contract_test.go`: approval-only LinkedIn dry-run.
- Modify `control-plane/internal/workflow/plans.go`: internal seed artifact convention for content preferences.
- Modify `control-plane/internal/workflow/worker.go` and `control-plane/cmd/api/main.go`: do not register Go `RunAgentActivity` in the API worker.
- Create `agent-runtime/src/harpia_agents/artifacts/client.py`: internal ArtifactService client wrapper.
- Modify `agent-runtime/src/harpia_agents/temporal/worker.py`: load upstream artifacts, run both agents, persist output artifact, return real artifact IDs and elicitation metadata.
- Test `agent-runtime/tests/test_temporal_agent_activity.py`.
- Create `frontend/src/lib/plans/linkedin-suggestions.ts`: deterministic LinkedIn plan suggestion builder.
- Modify `frontend/src/lib/plans/assistant.ts` and `.test.ts`: apply suggested plan configuration.
- Modify `frontend/src/lib/components/thread/BindingMatrixCard.svelte`: Suggest button/dialog on the binding matrix card.
- Modify `frontend/src/lib/mocks/plan-catalog.ts`: demo-ready sports RSS groups and LinkedIn approval-only installation.
- Modify `frontend/src/lib/inbox/aggregator.test.ts`: publish approval appears as an inbox item with configuration and artifact pointers.
- Modify `frontend/e2e/engineer-journey.spec.ts`: reflect DeepSeek, grouped RSS, approval-only LinkedIn, Suggest, and inbox approval.

## Execution Notes

- Work on the existing branch unless execution mode creates an isolated worktree.
- Commit after each task. Do not include untracked `.svelte-kit/` or `docs/superpowers/brainstorming/`.
- Use these model IDs only for DeepSeek: `deepseek-v4-flash`, `deepseek-v4-pro`.
- Treat DeepSeek pricing as out of scope. Registry pricing entries are zero so model routing works without presenting price claims.
- Monday demo requires ending in a pending publish approval, not public publishing.

### Task 1: DeepSeek Agent Runtime Provider

**Files:**
- Create: `agent-runtime/src/harpia_agents/llm/providers/deepseek.py`
- Modify: `agent-runtime/src/harpia_agents/llm/pricing.py`
- Modify: `agent-runtime/src/harpia_agents/llm/registry.py`
- Modify: `agent-runtime/src/harpia_agents/agents/registry.py`
- Modify: `agents/newsletter-writer-senior/0.1.0.yaml`
- Modify: `agents/linkedin-voice-senior/0.1.0.yaml`
- Test: `agent-runtime/tests/llm/test_deepseek_provider.py`
- Test: `agent-runtime/tests/test_registry_tenant_llm.py`

- [ ] **Step 1: Write failing DeepSeek provider tests**

Add `agent-runtime/tests/llm/test_deepseek_provider.py`:

```python
import pytest
from pytest_httpx import HTTPXMock

from harpia_agents.llm.errors import AuthenticationError, RateLimitError
from harpia_agents.llm.provider import ChatMessage
from harpia_agents.llm.providers.deepseek import DeepSeekProvider


def _messages() -> list[ChatMessage]:
    return [ChatMessage(role="user", content="Write a sports update.")]


@pytest.mark.asyncio
async def test_complete_uses_deepseek_openai_compatible_endpoint(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.deepseek.com/chat/completions",
        match_headers={"authorization": "Bearer ds-key"},
        json={
            "choices": [{"message": {"content": "Sports draft"}}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 4},
        },
    )
    provider = DeepSeekProvider(api_key="ds-key")

    result = await provider.complete("deepseek-v4-flash", _messages())

    assert result.provider == "deepseek"
    assert result.model_id == "deepseek-v4-flash"
    assert result.content == "Sports draft"
    assert result.usage.input_tokens == 10
    assert result.usage.output_tokens == 4


@pytest.mark.asyncio
async def test_auth_failure_maps_to_authentication_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.deepseek.com/chat/completions",
        status_code=401,
        text="invalid api key",
    )
    provider = DeepSeekProvider(api_key="bad")

    with pytest.raises(AuthenticationError):
        await provider.complete("deepseek-v4-flash", _messages())


@pytest.mark.asyncio
async def test_rate_limit_maps_to_rate_limit_error(httpx_mock: HTTPXMock) -> None:
    httpx_mock.add_response(
        method="POST",
        url="https://api.deepseek.com/chat/completions",
        status_code=429,
        text="rate limited",
    )
    provider = DeepSeekProvider(api_key="ds-key")

    with pytest.raises(RateLimitError):
        await provider.complete("deepseek-v4-pro", _messages())
```

- [ ] **Step 2: Verify provider test fails**

Run from repo root:

```bash
uv run --project agent-runtime pytest agent-runtime/tests/llm/test_deepseek_provider.py -q
```

Expected: FAIL with `ModuleNotFoundError: No module named 'harpia_agents.llm.providers.deepseek'`.

- [ ] **Step 3: Implement DeepSeek provider and registry routing**

Add to `agent-runtime/src/harpia_agents/llm/pricing.py`:

```python
DEEPSEEK_PRICING: dict[str, ModelPricing] = {
    "deepseek-v4-flash": ModelPricing(input_per_million_usd=0.0, output_per_million_usd=0.0),
    "deepseek-v4-pro": ModelPricing(input_per_million_usd=0.0, output_per_million_usd=0.0),
}
```

Create `agent-runtime/src/harpia_agents/llm/providers/deepseek.py`:

```python
"""DeepSeek OpenAI-compatible implementation for chat completion requests."""

from __future__ import annotations

import os

from harpia_agents.llm.pricing import DEEPSEEK_PRICING, ModelPricing
from harpia_agents.llm.providers.openai import OpenAIProvider


class DeepSeekProvider(OpenAIProvider):
    name = "deepseek"
    supported_models = frozenset(DEEPSEEK_PRICING.keys())
    pricing: dict[str, ModelPricing] = DEEPSEEK_PRICING

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = "https://api.deepseek.com",
        timeout: float = 30.0,
    ) -> None:
        key = os.getenv("DEEPSEEK_API_KEY", "") if api_key is None else api_key
        super().__init__(api_key=key, base_url=base_url, timeout=timeout)
        self._api_key = key
```

Modify `agent-runtime/src/harpia_agents/llm/registry.py`:

```python
from harpia_agents.llm.providers.deepseek import DeepSeekProvider
```

and include DeepSeek in `LLMRegistry.default()`:

```python
providers: list[LLMProvider] = [
    AnthropicProvider(),
    DeepSeekProvider(),
    OpenAIProvider(),
    OllamaProvider(),
]
```

Modify `agent-runtime/src/harpia_agents/agents/registry.py` imports:

```python
from harpia_agents.llm.providers.deepseek import DeepSeekProvider
```

Change `_default_provider_for_manifest`:

```python
def _default_provider_for_manifest(manifest_id: str) -> str | None:
    providers = {
        "newsletter-writer-senior": "deepseek",
        "linkedin-voice-senior": "deepseek",
    }
    return providers.get(manifest_id)
```

Change `_build_registry_with_tenant_credentials` provider list:

```python
providers: list[LLMProvider] = [
    DeepSeekProvider(api_key=api_key if resolved.provider == "deepseek" else None),
    OpenAIProvider(api_key=api_key if resolved.provider == "openai" else None),
    AnthropicProvider(api_key=api_key if resolved.provider == "anthropic" else None),
    OllamaProvider(),
]
```

Change `model_id` in both agent manifests:

```yaml
model_id: deepseek-v4-flash
```

- [ ] **Step 4: Add tenant credential test for DeepSeek**

Append to `agent-runtime/tests/test_registry_tenant_llm.py`:

```python
def test_build_registry_with_tenant_credentials_uses_deepseek_provider() -> None:
    registry = _build_registry_with_tenant_credentials(
        ResolvedProviderCredentials(
            provider="deepseek",
            api_key=RedactedSecret("ds-test"),
            default_model="deepseek-v4-flash",
            allowed_models=("deepseek-v4-flash",),
            source=1,
        )
    )

    provider = registry.resolve("deepseek-v4-flash")
    assert provider.name == "deepseek"
```

- [ ] **Step 5: Verify DeepSeek runtime tests pass**

Run from repo root:

```bash
uv run --project agent-runtime pytest agent-runtime/tests/llm/test_deepseek_provider.py agent-runtime/tests/test_registry_tenant_llm.py -q
```

Expected: PASS for the DeepSeek provider tests and tenant credential tests.

- [ ] **Step 6: Commit**

```bash
git add agent-runtime/src/harpia_agents/llm/providers/deepseek.py agent-runtime/src/harpia_agents/llm/pricing.py agent-runtime/src/harpia_agents/llm/registry.py agent-runtime/src/harpia_agents/agents/registry.py agents/newsletter-writer-senior/0.1.0.yaml agents/linkedin-voice-senior/0.1.0.yaml agent-runtime/tests/llm/test_deepseek_provider.py agent-runtime/tests/test_registry_tenant_llm.py
git commit -m "feat(m7): add deepseek agent runtime provider"
```

### Task 2: DeepSeek BYOK Admin Settings

**Files:**
- Modify: `frontend/src/lib/clients/llm-config-client.ts`
- Modify: `frontend/src/lib/clients/llm-config-client.connect.ts`
- Modify: `frontend/src/lib/clients/llm-config-client.mock.ts`
- Modify: `frontend/src/lib/clients/llm-config-client.test.ts`
- Modify: `frontend/src/routes/admin/settings/+page.svelte`

- [ ] **Step 1: Write failing frontend client tests**

Modify the first test in `frontend/src/lib/clients/llm-config-client.test.ts`:

```ts
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
```

Add this test under `setLLMProviderConfig`:

```ts
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
```

- [ ] **Step 2: Verify frontend client tests fail**

Run from `frontend/`:

```bash
bun run test -- src/lib/clients/llm-config-client.test.ts
```

Expected: FAIL because `deepseek` is not assignable to `LLMProvider` and the mock returns three providers.

- [ ] **Step 3: Add DeepSeek to frontend LLM config types and mock**

Modify `frontend/src/lib/clients/llm-config-client.ts`:

```ts
export type LLMProvider = "anthropic" | "deepseek" | "openai" | "ollama";
```

Modify `frontend/src/lib/clients/llm-config-client.connect.ts`:

```ts
function toDomainProvider(provider: string): LLMProvider {
  if (
    provider === "anthropic" ||
    provider === "deepseek" ||
    provider === "openai" ||
    provider === "ollama"
  ) {
    return provider;
  }
  throw new Error(`unsupported provider: ${provider}`);
}
```

Modify `frontend/src/lib/clients/llm-config-client.mock.ts`:

```ts
deepseek: {
  provider: "deepseek",
  default_model: "deepseek-v4-flash",
  allowed_models: [],
  has_key: false,
  managed_by: "platform",
  last_rotated_at: null,
},
```

and set:

```ts
const providers: LLMProvider[] = ["anthropic", "deepseek", "openai", "ollama"];
```

- [ ] **Step 4: Add DeepSeek to Admin Settings UI**

Modify `frontend/src/routes/admin/settings/+page.svelte` provider catalogue:

```ts
const PROVIDERS: { id: LLMProvider; label: string }[] = [
  { id: "anthropic", label: "Anthropic" },
  { id: "deepseek", label: "DeepSeek" },
  { id: "openai", label: "OpenAI" },
  { id: "ollama", label: "Ollama" },
];

const PROVIDER_MODELS: Record<LLMProvider, string[]> = {
  anthropic: [
    "claude-opus-4-5",
    "claude-3-5-sonnet-20241022",
    "claude-3-5-haiku-20241022",
  ],
  deepseek: ["deepseek-v4-flash", "deepseek-v4-pro"],
  openai: ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo"],
  ollama: ["llama3:8b", "llama3:70b", "mistral:7b"],
};
```

Add DeepSeek defaults to `providerConfigs`, `keyInputs`, `selectedModels`, and `allowedModels`:

```ts
deepseek: {
  provider: "deepseek",
  default_model: "deepseek-v4-flash",
  allowed_models: [],
  has_key: false,
  managed_by: "platform",
  last_rotated_at: null,
},
```

```ts
deepseek: "",
```

```ts
deepseek: "deepseek-v4-flash",
```

```ts
deepseek: [],
```

- [ ] **Step 5: Verify DeepSeek frontend tests pass**

Run from `frontend/`:

```bash
bun run test -- src/lib/clients/llm-config-client.test.ts
bun run check
```

Expected: both commands pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/clients/llm-config-client.ts frontend/src/lib/clients/llm-config-client.connect.ts frontend/src/lib/clients/llm-config-client.mock.ts frontend/src/lib/clients/llm-config-client.test.ts frontend/src/routes/admin/settings/+page.svelte
git commit -m "feat(m7): expose deepseek byo key settings"
```

### Task 3: Grouped RSS Feed Installations UI

**Files:**
- Modify: `frontend/src/lib/integrations/executor-installations.ts`
- Modify: `frontend/src/lib/integrations/executor-installations.test.ts`
- Modify: `frontend/src/routes/admin/integrations/+page.svelte`

- [ ] **Step 1: Write failing tests for many RSS installations and approval-only LinkedIn values**

In `frontend/src/lib/integrations/executor-installations.test.ts`, extend `DemoIntegrationFormValues` setup:

```ts
const BASE_VALUES: DemoIntegrationFormValues = {
  displayName: "Demo integration",
  enabled: true,
  feedsText: "",
  linkedinMode: "oauth",
  oauthCredentialId: "",
};
```

Add tests:

```ts
it("builds LinkedIn approval-only config without an OAuth credential", () => {
  const config = buildIntegrationConfigJSON("linkedin", {
    ...BASE_VALUES,
    linkedinMode: "approval_only",
    oauthCredentialId: "",
  });

  expect(JSON.parse(config)).toEqual({
    mode: "approval_only",
  });
  expect(validateIntegrationForm("linkedin", {
    ...BASE_VALUES,
    linkedinMode: "approval_only",
  })).toBeNull();
});

it("uses installation id based form keys so many RSS groups do not collide", () => {
  const first = {
    kind: "rss",
    sku: create(ExecutorSKUSchema, { id: "sku-rss", key: "rss-news-feed" }),
    installation: create(ExecutorInstallationSchema, { id: "inst-rss-1" }),
    connectionStatus: ConnectionStatus.CONNECTED,
    configured: true,
  } as DemoIntegrationCard;
  const second = {
    ...first,
    installation: create(ExecutorInstallationSchema, { id: "inst-rss-2" }),
  } as DemoIntegrationCard;

  expect(formKeyForCard(first)).toBe("inst-rss-1");
  expect(formKeyForCard(second)).toBe("inst-rss-2");
  expect(formKeyForNewCard("rss-news-feed", 3)).toBe("new:rss-news-feed:3");
});
```

- [ ] **Step 2: Verify integration tests fail**

Run from `frontend/`:

```bash
bun run test -- src/lib/integrations/executor-installations.test.ts
```

Expected: FAIL because `linkedinMode`, `formKeyForCard`, and `formKeyForNewCard` do not exist.

- [ ] **Step 3: Update integration domain helpers**

Modify `frontend/src/lib/integrations/executor-installations.ts`:

```ts
export type LinkedInIntegrationMode = "oauth" | "approval_only";

export interface DemoIntegrationFormValues {
  displayName: string;
  enabled: boolean;
  feedsText: string;
  linkedinMode: LinkedInIntegrationMode;
  oauthCredentialId: string;
}

export interface DemoIntegrationGroup {
  kind: DemoIntegrationKind;
  sku: ExecutorSKU;
  entitlement?: ExecutorEntitlement;
  cards: DemoIntegrationCard[];
  canAdd: boolean;
}

export interface DemoIntegrationContext {
  groups: DemoIntegrationGroup[];
  cards: DemoIntegrationCard[];
}
```

Add key helpers:

```ts
export function formKeyForCard(card: DemoIntegrationCard): string {
  return card.installation?.id || `new:${card.sku.key}:0`;
}

export function formKeyForNewCard(skuKey: string, index: number): string {
  return `new:${skuKey}:${index}`;
}
```

Change `formValuesFromCard` return shape:

```ts
const mode =
  config.mode === "approval_only" ? "approval_only" : "oauth";

return {
  displayName:
    card.installation?.displayName || card.sku.displayName || card.sku.key,
  enabled: card.installation?.enabled ?? true,
  feedsText: Array.isArray(config.feeds)
    ? config.feeds.filter(isString).join("\n")
    : "",
  linkedinMode: mode,
  oauthCredentialId: isString(config.oauth_credential_id)
    ? config.oauth_credential_id
    : "",
};
```

Change LinkedIn validation:

```ts
if (
  kind === "linkedin" &&
  values.linkedinMode === "oauth" &&
  values.oauthCredentialId.trim().length === 0
) {
  return "linkedinCredentialRequired";
}
```

Change LinkedIn config building:

```ts
if (values.linkedinMode === "approval_only") {
  return JSON.stringify({ mode: "approval_only" });
}

return JSON.stringify({
  mode: "oauth",
  oauth_credential_id: values.oauthCredentialId.trim(),
});
```

Change `loadDemoIntegrationContext` to keep all installations:

```ts
const installationsBySkuID = new Map<string, ExecutorInstallation[]>();
for (const installation of installations) {
  const current = installationsBySkuID.get(installation.executorSkuId) ?? [];
  installationsBySkuID.set(installation.executorSkuId, [...current, installation]);
}
```

Build groups:

```ts
const groups = skus
  .filter((sku) => isDemoIntegrationSkuKey(sku.key))
  .sort(
    (left, right) =>
      (order.get(left.key as DemoIntegrationSkuKey) ?? 99) -
      (order.get(right.key as DemoIntegrationSkuKey) ?? 99),
  )
  .map((sku) => {
    const kind = integrationKindForSkuKey(sku.key);
    if (!kind) throw new Error(`Unsupported integration SKU: ${sku.key}`);
    const skuInstallations = installationsBySkuID.get(sku.id) ?? [];
    const cards = skuInstallations.map((installation) => ({
      kind,
      sku,
      entitlement: entitlementBySkuID.get(sku.id),
      installation,
      connectionStatus: connectionStatusFromInstallation(installation),
      configured: isInstallationConfigured(installation),
    }));
    if (cards.length === 0) {
      cards.push({
        kind,
        sku,
        entitlement: entitlementBySkuID.get(sku.id),
        connectionStatus: ConnectionStatus.UNSPECIFIED,
        configured: false,
      });
    }
    return {
      kind,
      sku,
      entitlement: entitlementBySkuID.get(sku.id),
      cards,
      canAdd: kind === "rss",
    };
  });

return { groups, cards: groups.flatMap((group) => group.cards) };
```

- [ ] **Step 4: Update Admin Integrations UI to render groups**

Modify `frontend/src/routes/admin/integrations/+page.svelte` state:

```ts
let groups = $state<DemoIntegrationGroup[]>([]);
let forms = $state<Record<string, DemoIntegrationFormValues>>({});
let newCardCounters = $state<Record<string, number>>({});
```

Load groups:

```ts
const context = await loadDemoIntegrationContext(tenantId);
groups = context.groups;
cards = context.cards;
forms = Object.fromEntries(
  context.cards.map((card) => [formKeyForCard(card), formValuesFromCard(card)]),
);
```

Use `formKeyForCard(card)` anywhere the page currently uses `card.sku.key` as the form key, saving key, saved key, label id suffix, and test id suffix.

Add local RSS new-card action:

```ts
function addFeedGroup(group: DemoIntegrationGroup) {
  const next = (newCardCounters[group.sku.key] ?? 0) + 1;
  newCardCounters = { ...newCardCounters, [group.sku.key]: next };
  const card: DemoIntegrationCard = {
    kind: group.kind,
    sku: group.sku,
    entitlement: group.entitlement,
    connectionStatus: ConnectionStatus.UNSPECIFIED,
    configured: false,
  };
  const key = formKeyForNewCard(group.sku.key, next);
  forms = { ...forms, [key]: formValuesFromCard(card) };
  groups = groups.map((entry) =>
    entry.sku.id === group.sku.id
      ? { ...entry, cards: [...entry.cards, card] }
      : entry,
  );
}
```

Render grouped sections with this shape:

```svelte
{#each groups as group (group.sku.id)}
  <section class="space-y-3">
    <div class="flex items-center justify-between gap-3">
      <HarpyHeading tag="h2" class="text-lg text-text">
        {group.sku.displayName}
      </HarpyHeading>
      {#if group.canAdd}
        <button
          type="button"
          onclick={() => addFeedGroup(group)}
          class="rounded-md border border-border px-3 py-1.5 text-sm text-text-muted hover:bg-surface-hover hover:text-text"
        >
          {translate("integrations.rss.addGroup", $locale)}
        </button>
      {/if}
    </div>
    <div class="grid gap-4 md:grid-cols-2">
      {#each group.cards as card, index (formKeyForCard(card) + index)}
        <!-- move the existing article markup here and read form with formFor(card) -->
      {/each}
    </div>
  </section>
{/each}
```

For LinkedIn forms, add a mode select before credential input:

```svelte
<select
  id={`linkedin-mode-${formKeyForCard(card)}`}
  value={form.linkedinMode}
  onchange={(event) =>
    updateForm(card, {
      linkedinMode: (event.currentTarget as HTMLSelectElement)
        .value as DemoIntegrationFormValues["linkedinMode"],
    })}
  class="w-full rounded-md border border-border bg-surface px-3 py-2 font-body text-sm text-text outline-none focus:border-primary"
>
  <option value="approval_only">
    {translate("integrations.linkedin.mode.approvalOnly", $locale)}
  </option>
  <option value="oauth">
    {translate("integrations.linkedin.mode.oauth", $locale)}
  </option>
</select>
```

Show the credential input only when `form.linkedinMode === "oauth"`.

- [ ] **Step 5: Add minimal i18n strings**

Add these keys to the existing frontend i18n catalog file that contains `integrations.rss.feedUrls`:

```ts
"integrations.rss.addGroup": "Add feed group",
"integrations.linkedin.mode.label": "Mode",
"integrations.linkedin.mode.approvalOnly": "Approval only",
"integrations.linkedin.mode.oauth": "LinkedIn OAuth",
```

If the catalog is JSON rather than TS, use the same key/value pairs in JSON syntax.

- [ ] **Step 6: Verify integrations tests and Svelte check**

Run from `frontend/`:

```bash
bun run test -- src/lib/integrations/executor-installations.test.ts
bun run check
```

Expected: both commands pass.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/integrations/executor-installations.ts frontend/src/lib/integrations/executor-installations.test.ts frontend/src/routes/admin/integrations/+page.svelte frontend/src/lib/i18n
git commit -m "feat(m7): group multiple integration feed sources"
```

### Task 4: LinkedIn Approval-Only Dry-Run Integration

**Files:**
- Modify: `control-plane/internal/executors/integrations/linkedin/config.go`
- Modify: `control-plane/internal/executors/integrations/linkedin/handler.go`
- Modify: `control-plane/internal/executors/integrations/linkedin/config_test.go`
- Modify: `control-plane/internal/executors/integrations/linkedin/config_validator_test.go`
- Modify: `control-plane/internal/executors/integrations/linkedin/handler_test.go`
- Modify: `control-plane/internal/executors/integrations/linkedin/contract_test.go`

- [ ] **Step 1: Write failing LinkedIn config tests**

Append to `control-plane/internal/executors/integrations/linkedin/config_test.go`:

```go
func TestParseInstallationConfigApprovalOnly(t *testing.T) {
	cfg, err := ParseInstallationConfig([]byte(`{"mode":"approval_only"}`))
	if err != nil {
		t.Fatalf("ParseInstallationConfig returned error: %v", err)
	}
	if cfg.Mode != ModeApprovalOnly {
		t.Fatalf("mode = %q, want %q", cfg.Mode, ModeApprovalOnly)
	}
	if cfg.OAuthCredentialID != "" {
		t.Fatalf("oauth credential id = %q, want empty", cfg.OAuthCredentialID)
	}
}
```

Append to `control-plane/internal/executors/integrations/linkedin/handler_test.go`:

```go
func TestHandlerApprovalOnlyCreatesDryRunConfirmationWithoutPublisher(t *testing.T) {
	store := newLinkedInTestArtifactStore()
	publisher := &linkedin.FakePublisher{}
	handler := linkedin.NewHandler(store, publisher)

	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		StepExecutionID:       "step-publish-linkedin",
		OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyLinkedInPostDraft,
			LiteralJSON:     []byte(`{"text":"Ready for approval","hook":"Hook","hashtags":["sports"]}`),
		}},
		Installation: runtime.InstallationSnapshot{
			ConfigJSON: []byte(`{"mode":"approval_only"}`),
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != runtime.IntegrationStatusCompleted {
		t.Fatalf("status = %q, want completed: %s", result.Status, result.Error)
	}
	if publisher.Calls != 0 {
		t.Fatalf("publisher calls = %d, want 0", publisher.Calls)
	}
	payload := store.PayloadForArtifact(t, result.OutputArtifactID)
	if !strings.Contains(string(payload), `"platform":"linkedin-dry-run"`) {
		t.Fatalf("payload = %s, want dry-run platform", payload)
	}
	if !strings.Contains(string(payload), `"externalId":"dry-run-step-publish-linkedin"`) {
		t.Fatalf("payload = %s, want deterministic dry-run external id", payload)
	}
}
```

- [ ] **Step 2: Verify LinkedIn tests fail**

Run from `control-plane/`:

```bash
go test ./internal/executors/integrations/linkedin
```

Expected: FAIL because `Mode`, `ModeApprovalOnly`, and dry-run behavior do not exist.

- [ ] **Step 3: Implement approval-only config parsing**

Modify `control-plane/internal/executors/integrations/linkedin/config.go`:

```go
const (
	ModeOAuth        = "oauth"
	ModeApprovalOnly = "approval_only"
)

type InstallationConfig struct {
	Mode              string `json:"mode"`
	OAuthCredentialID string `json:"oauth_credential_id"`
}
```

In `ParseInstallationConfig`, after unmarshal:

```go
config.Mode = strings.TrimSpace(config.Mode)
if config.Mode == "" {
	config.Mode = ModeOAuth
}
if config.Mode != ModeOAuth && config.Mode != ModeApprovalOnly {
	return InstallationConfig{}, fmt.Errorf("%w: mode must be oauth or approval_only", ErrInvalidConfig)
}

config.OAuthCredentialID = strings.TrimSpace(config.OAuthCredentialID)
if config.Mode == ModeOAuth && config.OAuthCredentialID == "" {
	return InstallationConfig{}, fmt.Errorf("%w: oauth_credential_id is required", ErrInvalidConfig)
}
if config.Mode == ModeApprovalOnly {
	config.OAuthCredentialID = ""
}
```

- [ ] **Step 4: Implement dry-run confirmation path**

Modify `control-plane/internal/executors/integrations/linkedin/handler.go` after draft parsing and before `h.publisher.Publish`:

```go
if config.Mode == ModeApprovalOnly {
	return h.createConfirmation(ctx, req, &artifactsv1.PublishConfirmation{
		Platform:    "linkedin-dry-run",
		ExternalId:  "dry-run-" + strings.TrimSpace(req.StepExecutionID),
		Url:         "",
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
	})
}
```

Extract confirmation creation into a helper used by both real and dry-run paths:

```go
func (h *Handler) createConfirmation(ctx context.Context, req runtime.IntegrationExecutionRequest, confirmation *artifactsv1.PublishConfirmation) (runtime.IntegrationExecutionResult, error) {
	payload, err := protojson.Marshal(confirmation)
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("marshal publish confirmation: %w", err)
	}
	outputArtifactID, err := h.artifacts.CreateValidatedPayload(ctx, runtime.CreateArtifactRequest{
		TenantID:              req.TenantID,
		OutputArtifactTypeKey: req.OutputArtifactTypeKey,
		StepExecutionID:       req.StepExecutionID,
		Payload:               payload,
	})
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("create publish confirmation artifact: %w", err)
	}
	return runtime.IntegrationExecutionResult{
		Status:           runtime.IntegrationStatusCompleted,
		OutputArtifactID: outputArtifactID,
	}, nil
}
```

Replace the existing marshal/create block after real publish with:

```go
return h.createConfirmation(ctx, req, confirmation)
```

- [ ] **Step 5: Verify LinkedIn tests pass**

Run from `control-plane/`:

```bash
go test ./internal/executors/integrations/linkedin
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/executors/integrations/linkedin
git commit -m "feat(m7): add linkedin approval-only dry run"
```

### Task 5: Suggest LinkedIn Sports Plan Configuration

**Files:**
- Create: `frontend/src/lib/plans/linkedin-suggestions.ts`
- Modify: `frontend/src/lib/plans/assistant.ts`
- Modify: `frontend/src/lib/plans/assistant.test.ts`
- Modify: `frontend/src/lib/components/thread/BindingMatrixCard.svelte`
- Modify: `frontend/src/lib/mocks/plan-catalog.ts`
- Modify: `control-plane/internal/workflow/plans.go`
- Modify: `control-plane/internal/workflow/plans_test.go`

- [ ] **Step 1: Write failing suggestion helper tests**

Extend the existing imports in `frontend/src/lib/plans/assistant.test.ts` so they include `applyLinkedInSuggestion`, `PublishApprovalMode`, `SeedArtifactBinding`, and `SlotBinding`:

```ts
import { selectChip, editBinding, applyLinkedInSuggestion } from "./assistant";
import {
  PlanConfigurationStatus,
  PublishApprovalMode,
  type PlanConfiguration,
  type PlanTemplate,
  type SeedArtifactBinding,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";
```

Append this test block:

```ts
describe("applyLinkedInSuggestion", () => {
  it("binds the Monday sports LinkedIn plan and stores content preferences", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      seedArtifacts: [],
      slotBindings: [],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
    } as unknown as PlanConfiguration;
    const template = {
      id: "tpl",
      steps: [
        { key: "fetch-news", defaultExecutorSkuKey: "rss-news-feed" },
        { key: "write-draft", defaultExecutorSkuKey: "newsletter-writer-senior" },
        { key: "adapt-for-linkedin", defaultExecutorSkuKey: "linkedin-voice-senior" },
        { key: "publish-linkedin", defaultExecutorSkuKey: "linkedin-publish" },
      ],
    } as PlanTemplate;

    await applyLinkedInSuggestion({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template,
      topic: "sports",
      installationIdsByStep: {
        "fetch-news": "inst-rss-sports",
        "write-draft": "inst-newsletter",
        "adapt-for-linkedin": "inst-linkedin-voice",
        "publish-linkedin": "inst-linkedin-approval",
      },
      today: new Date("2026-06-26T12:00:00Z"),
    });

    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.slotBindings.map((binding: SlotBinding) => binding.stepKey)).toEqual([
      "fetch-news",
      "write-draft",
      "adapt-for-linkedin",
      "publish-linkedin",
    ]);
    expect(call.behaviorPolicies.publishApprovalMode).toBe(
      PublishApprovalMode.REQUIRE_APPROVAL,
    );
    const dateSeed = call.seedArtifacts.find(
      (seed: SeedArtifactBinding) => seed.stepKey === "fetch-news",
    );
    expect(JSON.parse(dateSeed.literalJson)).toEqual({
      startDate: "2026-06-20",
      endDate: "2026-06-26",
    });
    const preferences = call.seedArtifacts.find(
      (seed: SeedArtifactBinding) =>
        seed.inputName === "harpia.internal.ContentPreferences",
    );
    expect(JSON.parse(preferences.literalJson)).toMatchObject({
      topic: "sports",
      tone: "analytical, concise, and practical",
    });
  });
});
```

- [ ] **Step 2: Verify suggestion tests fail**

Run from `frontend/`:

```bash
bun run test -- src/lib/plans/assistant.test.ts
```

Expected: FAIL because `applyLinkedInSuggestion` does not exist.

- [ ] **Step 3: Implement suggestion builder**

Create `frontend/src/lib/plans/linkedin-suggestions.ts`:

```ts
import { create } from "@bufbuild/protobuf";
import {
  ElicitationTimeoutBehavior,
  ExecutorKind,
  PlanBehaviorPoliciesSchema,
  SeedArtifactBindingSchema,
  SlotBindingSchema,
  PublishApprovalMode,
  type PlanBehaviorPolicies,
  type SeedArtifactBinding,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export interface LinkedInSuggestionInput {
  topic: string;
  installationIdsByStep: Record<string, string>;
  today: Date;
}

export interface LinkedInSuggestion {
  seedArtifacts: SeedArtifactBinding[];
  slotBindings: SlotBinding[];
  behaviorPolicies: PlanBehaviorPolicies;
}

function isoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setUTCDate(next.getUTCDate() + days);
  return next;
}

export function buildLinkedInSuggestion(input: LinkedInSuggestionInput): LinkedInSuggestion {
  const topic = input.topic.trim() || "sports";
  const end = input.today;
  const start = addDays(end, -6);
  return {
    seedArtifacts: [
      create(SeedArtifactBindingSchema, {
        stepKey: "fetch-news",
        inputName: "date_range",
        literalJson: JSON.stringify({
          startDate: isoDate(start),
          endDate: isoDate(end),
        }),
      }),
      create(SeedArtifactBindingSchema, {
        stepKey: "write-draft",
        inputName: "harpia.internal.ContentPreferences",
        literalJson: JSON.stringify({
          topic,
          tone: "analytical, concise, and practical",
          topics_to_avoid: "",
        }),
      }),
    ],
    slotBindings: [
      create(SlotBindingSchema, {
        stepKey: "fetch-news",
        executorKind: ExecutorKind.INTEGRATION,
        executorInstallationId: input.installationIdsByStep["fetch-news"],
      }),
      create(SlotBindingSchema, {
        stepKey: "write-draft",
        executorKind: ExecutorKind.AGENT,
        executorInstallationId: input.installationIdsByStep["write-draft"],
      }),
      create(SlotBindingSchema, {
        stepKey: "adapt-for-linkedin",
        executorKind: ExecutorKind.AGENT,
        executorInstallationId: input.installationIdsByStep["adapt-for-linkedin"],
      }),
      create(SlotBindingSchema, {
        stepKey: "publish-linkedin",
        executorKind: ExecutorKind.INTEGRATION,
        executorInstallationId: input.installationIdsByStep["publish-linkedin"],
      }),
    ],
    behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
      elicitationTimeoutBehavior:
        ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
      elicitationTimeoutHours: 0,
      publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
    }),
  };
}
```

- [ ] **Step 4: Implement apply helper**

Add to `frontend/src/lib/plans/assistant.ts`:

```ts
import { buildLinkedInSuggestion } from "$lib/plans/linkedin-suggestions";
```

Add:

```ts
export async function applyLinkedInSuggestion(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  topic: string;
  installationIdsByStep: Record<string, string>;
  today?: Date;
}): Promise<PlanConfiguration> {
  const suggestion = buildLinkedInSuggestion({
    topic: args.topic,
    installationIdsByStep: args.installationIdsByStep,
    today: args.today ?? new Date(),
  });
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "SYSTEM",
    "STEP_REBOUND",
    "",
    JSON.stringify({
      reason: "linkedin_suggestion",
      topic: args.topic,
      bound_steps: suggestion.slotBindings.map((binding) => binding.stepKey),
    }),
  );
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    seedArtifacts: suggestion.seedArtifacts,
    slotBindings: suggestion.slotBindings,
    overseerBindings: args.existingConfiguration.overseerBindings,
    behaviorPolicies: suggestion.behaviorPolicies,
    schedule: args.existingConfiguration.schedule,
  });
  if (!response.planConfiguration) {
    throw new Error(
      "applyLinkedInSuggestion: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}
```

- [ ] **Step 5: Allow internal content preference seed artifacts in workflow validation**

Append a test to `control-plane/internal/workflow/plans_test.go` near `stepInputArtifacts` tests:

```go
func TestStepInputArtifactsAllowsInternalContentPreferencesSeed(t *testing.T) {
	step := &plansv1.PlanStep{
		Key:                 "write-draft",
		InputArtifactTypeId: "harpia.artifacts.v1.NewsList",
	}
	inputs, err := stepInputArtifacts(
		step,
		[]string{"fetch-news"},
		map[string][]ArtifactRef{
			"write-draft": {{
				Source:      "seed",
				StepKey:     "write-draft",
				InputName:   "harpia.internal.ContentPreferences",
				LiteralJSON: `{"tone":"analytical"}`,
			}},
		},
		map[string]ArtifactRef{
			"fetch-news": {
				Source:          "step_output",
				StepKey:         "fetch-news",
				ArtifactID:      "art-news",
				ArtifactTypeKey: "harpia.artifacts.v1.NewsList",
			},
		},
	)
	if err != nil {
		t.Fatalf("stepInputArtifacts returned error: %v", err)
	}
	if got := inputs[0].ArtifactTypeKey; got != "harpia.internal.ContentPreferences" {
		t.Fatalf("internal seed artifact type = %q", got)
	}
}
```

Modify `seedArtifactsByStep` in `control-plane/internal/workflow/plans.go`:

```go
artifactTypeKey := ""
if strings.HasPrefix(strings.TrimSpace(seed.InputName), "harpia.internal.") {
	artifactTypeKey = strings.TrimSpace(seed.InputName)
}
seeds[stepKey] = append(seeds[stepKey], ArtifactRef{
	Source:          "seed",
	StepKey:         stepKey,
	InputName:       seed.InputName,
	ArtifactID:      seed.ArtifactId,
	LiteralJSON:     seed.LiteralJson,
	ArtifactTypeKey: artifactTypeKey,
})
```

Modify `validateStepInputArtifactTypes`:

```go
if strings.HasPrefix(actual, "harpia.internal.") {
	continue
}
```

- [ ] **Step 6: Wire Suggest button into Binding Matrix**

Modify `frontend/src/lib/components/thread/BindingMatrixCard.svelte` imports:

```ts
import { applyLinkedInSuggestion } from "$lib/plans/assistant";
```

Add state:

```ts
let suggestOpen = $state(false);
let suggestedTopic = $state("sports");
```

Add helper:

```ts
function installationIdsByStep(): Record<string, string> {
  const ids: Record<string, string> = {};
  for (const row of rows) {
    const ready = row.options.find((option) => option.id);
    if (ready?.id) ids[row.step_key] = ready.id;
  }
  return ids;
}

async function onSuggest() {
  if (!configuration || !template) return;
  saving = true;
  saveError = false;
  try {
    const next = await applyLinkedInSuggestion({
      tenantId,
      configurationId,
      existingConfiguration: configuration,
      template,
      topic: suggestedTopic,
      installationIdsByStep: installationIdsByStep(),
    });
    configuration = next;
    if (payload) {
      payload = {
        ...payload,
        rows: payload.rows.map((row) => ({
          ...row,
          current_executor_id:
            next.slotBindings.find((binding) => binding.stepKey === row.step_key)
              ?.executorInstallationId ?? row.current_executor_id,
        })),
      };
    }
    suggestOpen = false;
  } catch {
    saveError = true;
  } finally {
    saving = false;
  }
}
```

Add a compact Suggest button in the card header:

```svelte
<button
  type="button"
  onclick={() => (suggestOpen = true)}
  disabled={!configuration || !template || saving || submitted}
  class="rounded-md border border-plumage px-3 py-1.5 text-[12px] text-crown-ash hover:border-talon-gold hover:text-talon-gold disabled:opacity-50"
>
  Suggest
</button>
```

Add a small dialog below the header:

```svelte
{#if suggestOpen}
  <div class="border-b border-plumage bg-surface-deep px-5 py-4">
    <label class="mb-1 block text-[11px] text-crown-ash">
      Topic
    </label>
    <div class="flex gap-2">
      <input
        value={suggestedTopic}
        oninput={(event) =>
          (suggestedTopic = (event.currentTarget as HTMLInputElement).value)}
        class="min-w-0 flex-1 rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[13px] text-cream"
      />
      <button
        type="button"
        onclick={onSuggest}
        class="rounded-md bg-talon-gold px-3 py-2 text-[12px] font-semibold text-obsidian"
      >
        Apply
      </button>
    </div>
  </div>
{/if}
```

- [ ] **Step 7: Verify suggestion tests**

Run from repo root:

```bash
(cd frontend && bun run test -- src/lib/plans/assistant.test.ts)
(cd frontend && bun run check)
(cd control-plane && go test ./internal/workflow)
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/lib/plans/linkedin-suggestions.ts frontend/src/lib/plans/assistant.ts frontend/src/lib/plans/assistant.test.ts frontend/src/lib/components/thread/BindingMatrixCard.svelte frontend/src/lib/mocks/plan-catalog.ts control-plane/internal/workflow/plans.go control-plane/internal/workflow/plans_test.go
git commit -m "feat(m7): suggest linkedin sports plan configuration"
```

### Task 6: Agent Runtime Artifact Bridge

**Files:**
- Create: `agent-runtime/src/harpia_agents/artifacts/client.py`
- Modify: `agent-runtime/src/harpia_agents/temporal/worker.py`
- Test: `agent-runtime/tests/test_temporal_agent_activity.py`

- [ ] **Step 1: Write failing agent activity tests**

Create `agent-runtime/tests/test_temporal_agent_activity.py`:

```python
from __future__ import annotations

import json
from dataclasses import dataclass, field

import pytest

from harpia_agents.temporal import worker


@dataclass
class FakeArtifactClient:
    payloads: dict[str, dict[str, object]]
    created: list[dict[str, object]] = field(default_factory=list)

    async def get_payload(self, *, tenant_id: str, artifact_id: str) -> dict[str, object]:
        return self.payloads[artifact_id]

    async def create_payload(
        self,
        *,
        tenant_id: str,
        artifact_type_key: str,
        payload: dict[str, object],
        step_execution_id: str,
    ) -> str:
        self.created.append(
            {
                "tenant_id": tenant_id,
                "artifact_type_key": artifact_type_key,
                "payload": payload,
                "step_execution_id": step_execution_id,
            }
        )
        return f"artifact-{len(self.created)}"


@pytest.mark.asyncio
async def test_run_newsletter_agent_loads_news_artifact_and_persists_text_draft(monkeypatch: pytest.MonkeyPatch) -> None:
    fake = FakeArtifactClient(
        payloads={
            "news-1": {
                "articles": [
                    {
                        "title": "Final match",
                        "url": "https://example.com/match",
                        "summary": "A decisive game.",
                        "source": "Sports",
                        "publishedAt": "2026-06-25T12:00:00Z",
                    }
                ]
            }
        }
    )

    async def fake_run_registered_agent(*args, **kwargs):
        from harpia.artifacts.v1.artifacts_pb2 import TextDraft

        assert kwargs["input_payload"].articles[0].title == "Final match"
        assert kwargs["elicitation_responses"]["tone"] == "analytical"
        return TextDraft(title="Newsletter", body="Draft body")

    monkeypatch.setattr(worker, "ArtifactPayloadClient", lambda: fake)
    monkeypatch.setattr(worker, "run_registered_agent", fake_run_registered_agent)

    result = await worker.run_agent_activity(
        {
            "tenant_id": "00000000-0000-4000-8000-000000000001",
            "step_execution_id": "step-write",
            "output_artifact_type_key": "harpia.artifacts.v1.TextDraft",
            "executor_installation_snapshot": {
                "manifest_id": "newsletter-writer-senior",
            },
            "input_artifacts": [
                {
                    "artifact_type_key": "harpia.internal.ContentPreferences",
                    "input_name": "harpia.internal.ContentPreferences",
                    "literal_json": json.dumps({"tone": "analytical", "topic": "sports"}),
                },
                {
                    "artifact_type_key": "harpia.artifacts.v1.NewsList",
                    "artifact_id": "news-1",
                },
            ],
        }
    )

    assert result == {"status": "completed", "output_artifact_id": "artifact-1"}
    assert fake.created[0]["artifact_type_key"] == "harpia.artifacts.v1.TextDraft"
    assert fake.created[0]["payload"] == {"title": "Newsletter", "body": "Draft body"}


@pytest.mark.asyncio
async def test_run_linkedin_agent_loads_text_draft_and_persists_linkedin_draft(monkeypatch: pytest.MonkeyPatch) -> None:
    fake = FakeArtifactClient(
        payloads={
            "draft-1": {
                "title": "Newsletter",
                "body": "Draft body",
            }
        }
    )

    async def fake_run_registered_agent(*args, **kwargs):
        from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft

        assert kwargs["input_payload"].title == "Newsletter"
        return LinkedInPostDraft(text="LinkedIn text", hook="Newsletter", hashtags=["sports"])

    monkeypatch.setattr(worker, "ArtifactPayloadClient", lambda: fake)
    monkeypatch.setattr(worker, "run_registered_agent", fake_run_registered_agent)

    result = await worker.run_agent_activity(
        {
            "tenant_id": "00000000-0000-4000-8000-000000000001",
            "step_execution_id": "step-linkedin",
            "output_artifact_type_key": "harpia.artifacts.v1.LinkedInPostDraft",
            "executor_installation_snapshot": {
                "manifest_id": "linkedin-voice-senior",
            },
            "input_artifacts": [
                {
                    "artifact_type_key": "harpia.artifacts.v1.TextDraft",
                    "artifact_id": "draft-1",
                }
            ],
        }
    )

    assert result == {"status": "completed", "output_artifact_id": "artifact-1"}
    assert fake.created[0]["payload"] == {
      "text": "LinkedIn text",
      "hook": "Newsletter",
      "hashtags": ["sports"],
    }
```

- [ ] **Step 2: Verify activity tests fail**

Run from repo root:

```bash
uv run --project agent-runtime pytest agent-runtime/tests/test_temporal_agent_activity.py -q
```

Expected: FAIL because `ArtifactPayloadClient` does not exist and `run_agent_activity` returns inline IDs.

- [ ] **Step 3: Add ArtifactService client wrapper**

Create `agent-runtime/src/harpia_agents/artifacts/client.py`:

```python
"""Internal ArtifactService client for Temporal agent activities."""

from __future__ import annotations

import json
import os

from connectrpc.errors import ConnectError
from harpia.artifacts.v1.artifacts_connect import ArtifactServiceClient
from harpia.artifacts.v1.artifacts_pb2 import (
    CreateArtifactWithPayloadRequest,
    GetArtifactPayloadRequest,
)


def _control_plane_internal_base_url() -> str:
    explicit = os.environ.get("HARPIA_CONTROL_PLANE_INTERNAL_URL", "").strip()
    if explicit:
        return explicit.rstrip("/")
    public = os.environ.get("HARPIA_CONTROL_PLANE_URL", "http://localhost:8080").rstrip("/")
    return f"{public}/internal"


def _default_internal_auth_token() -> str:
    configured = os.environ.get("HARPIA_INTERNAL_AUTH_TOKEN", "").strip()
    if configured:
        return configured
    if os.environ.get("HARPIA_ALLOW_DEV_AUTH", "").lower() in {"1", "true", "yes"}:
        return "dev-internal-token"
    return ""


class ArtifactPayloadClient:
    def __init__(
        self,
        client: ArtifactServiceClient | None = None,
        *,
        base_url: str | None = None,
        auth_token: str | None = None,
        timeout_ms: int = 5000,
    ) -> None:
        self._client = client or ArtifactServiceClient(
            base_url=base_url or _control_plane_internal_base_url()
        )
        self._auth_token = auth_token if auth_token is not None else _default_internal_auth_token()
        self._timeout_ms = timeout_ms

    def _headers(self, tenant_id: str) -> dict[str, str]:
        if not self._auth_token:
            raise ConnectError("HARPIA_INTERNAL_AUTH_TOKEN is required outside dev mode")
        return {
            "Authorization": f"Bearer {self._auth_token}",
            "X-Tenant-ID": tenant_id,
        }

    async def get_payload(self, *, tenant_id: str, artifact_id: str) -> dict[str, object]:
        response = await self._client.get_artifact_payload(
            GetArtifactPayloadRequest(tenant_id=tenant_id, artifact_id=artifact_id),
            headers=self._headers(tenant_id),
            timeout_ms=self._timeout_ms,
        )
        decoded = json.loads(response.payload_json.decode("utf-8"))
        if not isinstance(decoded, dict):
            raise ValueError("artifact payload must decode to object")
        return decoded

    async def create_payload(
        self,
        *,
        tenant_id: str,
        artifact_type_key: str,
        payload: dict[str, object],
        step_execution_id: str,
    ) -> str:
        response = await self._client.create_artifact_with_payload(
            CreateArtifactWithPayloadRequest(
                tenant_id=tenant_id,
                artifact_type_key=artifact_type_key,
                payload_json=json.dumps(payload).encode("utf-8"),
                step_execution_id=step_execution_id,
            ),
            headers=self._headers(tenant_id),
            timeout_ms=self._timeout_ms,
        )
        if not response.HasField("artifact"):
            raise ConnectError("create artifact response missing artifact")
        return response.artifact.id
```

- [ ] **Step 4: Replace inline activity behavior**

Modify `agent-runtime/src/harpia_agents/temporal/worker.py` imports:

```python
from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, NewsList, TextDraft
from harpia_agents.artifacts.client import ArtifactPayloadClient
```

Remove `uuid` import.

Add helpers:

```python
def _parse_literal_json(raw: object) -> dict[str, object] | None:
    if not isinstance(raw, str) or not raw.strip():
        return None
    decoded = json.loads(raw)
    if not isinstance(decoded, dict):
        raise ValueError("literal_json must decode to object")
    return decoded


async def _resolve_input_payload(
    input_payload: dict,
    *,
    tenant_id: str,
    artifact_type_key: str,
) -> dict[str, object]:
    client = ArtifactPayloadClient()
    for artifact in input_payload.get("input_artifacts", []):
        if not isinstance(artifact, dict):
            continue
        if str(artifact.get("artifact_type_key", "")).strip() != artifact_type_key:
            continue
        artifact_id = str(artifact.get("artifact_id", "")).strip()
        if artifact_id:
            return await client.get_payload(tenant_id=tenant_id, artifact_id=artifact_id)
        literal = _parse_literal_json(artifact.get("literal_json"))
        if literal is not None:
            return literal
    raise ValueError(f"missing input artifact {artifact_type_key}")


def _extract_content_preferences(input_payload: dict) -> dict[str, str]:
    responses: dict[str, str] = {}
    for artifact in input_payload.get("input_artifacts", []):
        if not isinstance(artifact, dict):
            continue
        if str(artifact.get("input_name", "")).strip() != "harpia.internal.ContentPreferences":
            continue
        literal = _parse_literal_json(artifact.get("literal_json")) or {}
        for key in ("tone", "topic", "topics_to_avoid"):
            value = literal.get(key)
            if isinstance(value, str) and value.strip():
                responses[key] = value.strip()
    return responses
```

Change `run_agent_activity` input resolution:

```python
tenant_id = require_temporal_tenant(input_payload)
...
if manifest_id == "newsletter-writer-senior":
    payload = await _resolve_input_payload(
        input_payload,
        tenant_id=tenant_id,
        artifact_type_key="harpia.artifacts.v1.NewsList",
    )
    agent_input = NewsList()
    ParseDict(payload, agent_input)
    elicitation_responses = {
        **_extract_content_preferences(input_payload),
        **(_extract_elicitation_responses(input_payload) or {}),
    }
elif manifest_id == "linkedin-voice-senior":
    payload = await _resolve_input_payload(
        input_payload,
        tenant_id=tenant_id,
        artifact_type_key="harpia.artifacts.v1.TextDraft",
    )
    agent_input = TextDraft()
    ParseDict(payload, agent_input)
    elicitation_responses = _extract_elicitation_responses(input_payload)
else:
    raise ValueError(f"unsupported manifest id: {manifest_id}")
```

Call the agent:

```python
result = await run_registered_agent(
    manifest_id,
    tenant_id=tenant_id,
    input_payload=agent_input,
    llm_registry=_LLM_REGISTRY,
    elicitation_responses=elicitation_responses,
)
```

Return elicitation details:

```python
if isinstance(result, ElicitationRequest):
    return {
        "status": "elicitation_requested",
        "elicitation_thread_id": result.thread_id,
        "elicitation_prompt": result.question,
        "elicitation_schema_json": json.dumps({
            "type": "object",
            "required": list(result.required_fields),
            "properties": {field: {"type": "string"} for field in result.required_fields},
        }),
    }
```

Persist outputs:

```python
if isinstance(result, TextDraft):
    output_type = "harpia.artifacts.v1.TextDraft"
elif isinstance(result, LinkedInPostDraft):
    output_type = "harpia.artifacts.v1.LinkedInPostDraft"
else:
    raise ValueError("unsupported agent result type")

payload = MessageToDict(result, preserving_proto_field_name=True)
artifact_id = await ArtifactPayloadClient().create_payload(
    tenant_id=tenant_id,
    artifact_type_key=str(input_payload.get("output_artifact_type_key") or output_type),
    payload=payload,
    step_execution_id=str(input_payload.get("step_execution_id", "")),
)
return {"status": "completed", "output_artifact_id": artifact_id}
```

- [ ] **Step 5: Verify agent activity tests pass**

Run from repo root:

```bash
uv run --project agent-runtime pytest agent-runtime/tests/test_temporal_agent_activity.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add agent-runtime/src/harpia_agents/artifacts/client.py agent-runtime/src/harpia_agents/temporal/worker.py agent-runtime/tests/test_temporal_agent_activity.py
git commit -m "feat(m7): persist agent activity artifacts"
```

### Task 7: Temporal Worker Ownership for RunAgentActivity

**Files:**
- Modify: `control-plane/internal/workflow/worker.go`
- Modify: `control-plane/cmd/api/main.go`

- [ ] **Step 1: Remove Go worker registration for agent activity**

Modify `control-plane/internal/workflow/worker.go`:

```go
		w.RegisterActivity(planActivities.RunIntegrationActivity)
		// RunAgentActivity is owned by the Python agent-runtime worker. If the
		// API worker registers it too, Temporal may dispatch agent work to the
		// old Go implementation and fail real plan executions.
		w.RegisterActivity(planActivities.ResumeStepExecutionActivity)
```

Leave `PlanActivities.RunAgentActivity` in `plans.go` for existing isolated workflow unit tests that explicitly mock or register the activity.

- [ ] **Step 2: Verify workflow tests still pass**

Run from `control-plane/`:

```bash
go test ./internal/workflow
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add control-plane/internal/workflow/worker.go control-plane/cmd/api/main.go
git commit -m "fix(m7): let python worker own agent activity"
```

### Task 8: Inbox Approval Visibility

**Files:**
- Modify: `frontend/src/lib/inbox/aggregator.test.ts`
- Modify if required by failing test: `frontend/src/lib/inbox/aggregator.ts`
- Modify if required by failing test: `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte`

- [ ] **Step 1: Write failing or confirming inbox test**

Add to `frontend/src/lib/inbox/aggregator.test.ts`:

```ts
it("surfaces pending publish approvals with configuration and input artifact pointers", async () => {
  const sources = singleEmitSources({
    watchElicitations: () => yieldOnce([]),
    watchApprovalRequests: () =>
      yieldOnce([
        makeApproval({
          id: "approval-publish",
          planConfigurationId: "config-linkedin",
          planExecutionId: "exec-linkedin",
          planStepKey: "publish-linkedin",
          stepExecutionId: "step-publish",
          inputArtifactId: "artifact-linkedin-draft",
          requestedAt: "2026-06-26T15:00:00Z",
        }),
      ]),
    loadFeedback: async () => [],
  });
  const iter = watchInbox("tenant-1", sources)[Symbol.asyncIterator]();
  let item: InboxItem | undefined;
  for (let i = 0; i < 5; i++) {
    const { value, done } = await iter.next();
    if (done) break;
    item = value!.find((entry) => entry.id === "approval-publish");
    if (item) break;
  }

  expect(item?.kind).toBe("approval");
  expect(item?.configurationId).toBe("config-linkedin");
  expect(item?.planExecutionId).toBe("exec-linkedin");
  expect(item?.stepExecutionId).toBe("step-publish");
  expect(item && "inputArtifactId" in item ? item.inputArtifactId : "").toBe(
    "artifact-linkedin-draft",
  );
});
```

- [ ] **Step 2: Run inbox test**

Run from `frontend/`:

```bash
bun run test -- src/lib/inbox/aggregator.test.ts
```

Expected: PASS if existing aggregator already supports it. If it fails, update `toInboxApproval` in `frontend/src/lib/inbox/aggregator.ts` to copy `configurationId`, `planExecutionId`, `stepExecutionId`, and `inputArtifactId` from the approval request as asserted.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/inbox/aggregator.test.ts frontend/src/lib/inbox/aggregator.ts frontend/src/lib/components/inbox/InboxApprovalEntry.svelte
git commit -m "test(m7): cover publish approval inbox visibility"
```

### Task 9: E2E Demo Flow and Final Verification

**Files:**
- Modify: `frontend/e2e/engineer-journey.spec.ts`
- Modify if required: `frontend/src/lib/mocks/plan-catalog.ts`
- Modify if required: local test fixtures under `frontend/e2e`

- [ ] **Step 1: Update Playwright journey expectations**

In `frontend/e2e/engineer-journey.spec.ts`, replace the singleton RSS section with grouped feed expectations:

```ts
const rssCards = page.getByTestId(/integration-card-rss-news-feed/);
await expect(rssCards.first()).toBeVisible();
await page.getByRole("button", { name: /add feed group/i }).click();
await expect(page.getByTestId(/integration-card-rss-news-feed/)).toHaveCount(2);
```

Update the RSS feed fill to use sports feeds:

```ts
await page
  .locator("textarea")
  .filter({ hasText: "" })
  .first()
  .fill("https://feeds.folha.uol.com.br/esporte/rss091.xml\nhttps://hnrss.org/frontpage");
```

Update LinkedIn setup to approval-only mode:

```ts
const linkedinCard = page.getByTestId(/integration-card-linkedin-publish/).first();
await linkedinCard.locator("select").selectOption("approval_only");
await linkedinCard.getByRole("button", { name: /save/i }).click();
```

Add the Suggest flow after selecting the LinkedIn plan:

```ts
await page.getByRole("button", { name: "Suggest" }).click();
await page.getByLabel("Topic").fill("sports");
await page.getByRole("button", { name: "Apply" }).click();
await expect(page.getByText("4/4")).toBeVisible();
```

Add inbox assertion:

```ts
await page.goto("/inbox");
await expect(page.getByText(/publish-linkedin/i)).toBeVisible();
await expect(page.getByText(/approval/i)).toBeVisible();
```

- [ ] **Step 2: Run focused frontend tests**

Run from `frontend/`:

```bash
bun run test -- src/lib/clients/llm-config-client.test.ts src/lib/integrations/executor-installations.test.ts src/lib/plans/assistant.test.ts src/lib/inbox/aggregator.test.ts
bun run check
```

Expected: both commands pass.

- [ ] **Step 3: Run focused backend and agent tests**

Run from repo root:

```bash
(cd control-plane && go test ./internal/executors/integrations/linkedin ./internal/workflow)
uv run --project agent-runtime pytest agent-runtime/tests/llm/test_deepseek_provider.py agent-runtime/tests/test_registry_tenant_llm.py agent-runtime/tests/test_temporal_agent_activity.py -q
```

Expected: all commands pass.

- [ ] **Step 4: Run E2E journey**

Run from `frontend/` with the app stack that existing E2E tests use:

```bash
bun run test:e2e -- engineer-journey.spec.ts
```

Expected: PASS. If the environment is not running, record the missing service and run the highest-signal unit/integration tests from Steps 2 and 3.

- [ ] **Step 5: Manual Monday rehearsal checklist**

Use the deployed dev stack or local stack:

```bash
scripts/start-server.sh
```

Expected: frontend and API become reachable on the project’s configured dev ports.

Then verify in browser:

```text
1. Platform Engineer opens /admin/settings and saves DeepSeek with model deepseek-v4-flash.
2. Platform Engineer opens /admin/integrations, creates one Sports headlines RSS group with https://feeds.folha.uol.com.br/esporte/rss091.xml and one optional HN group with https://hnrss.org/frontpage.
3. Platform Engineer sets LinkedIn Publish to Approval only.
4. Ana opens /new, manually chooses Weekly Newsletter (LinkedIn), opens the binding matrix, clicks Suggest, enters sports, and applies it.
5. Ana saves and runs the plan.
6. The plan executes fetch-news, write-draft, and adapt-for-linkedin with real artifact IDs.
7. publish-linkedin pauses for approval.
8. /inbox shows the pending approval with a preview link to the LinkedIn post draft.
```

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/engineer-journey.spec.ts frontend/src/lib/mocks/plan-catalog.ts
git commit -m "test(m7): cover linkedin content demo flow"
```

## Final Verification

- [ ] Run from `frontend/`:

```bash
bun run test
bun run check
```

Expected: Vitest and Svelte check pass.

- [ ] Run from `control-plane/`:

```bash
go test ./...
```

Expected: all Go tests pass.

- [ ] Run from repo root:

```bash
uv run --project agent-runtime pytest -q
```

Expected: all Python tests pass.

- [ ] Run from repo root:

```bash
git status --short
```

Expected: only intentional tracked changes are present. Untracked `.svelte-kit/` and `docs/superpowers/brainstorming/` remain uncommitted unless the user explicitly asks to include them.
