# /settings — Tenant Settings Page

Route implementing issue #54. Provides a single-page admin surface for tenant
configuration, LLM provider keys, quotas, and billing.

## Access

Server load (`+page.server.ts`) enforces `manageTenantSettings` permission
(defined in `$lib/auth-roles`). This grants access to **Engineer** and
**Leader** roles; **Overseer** is redirected with a 403.

## Sections

| Section       | Anchor           | Status                                         |
| ------------- | ---------------- | ---------------------------------------------- |
| Tenant        | `#tenant`        | Read-only (write APIs pending — see issue #36) |
| LLM Providers | `#llm-providers` | Interactive via mocked client                  |
| Quotas        | `#quotas`        | Disabled placeholder (pending issue #34)       |
| Billing       | `#billing`       | Placeholder (post-MVP)                         |

## LLM Provider Client

The LLM Providers section calls a typed client interface defined in
`$lib/clients/llm-config-client.ts`. The interface mirrors the RPC contract
from the #33 design note (`docs/notes/2026-06-16-tenant-llm-config-design.md`):

```ts
interface LLMConfigClient {
  getLLMProviderConfigs(req): Promise<GetLLMProviderConfigsResponse>;
  setLLMProviderConfig(req): Promise<SetLLMProviderConfigResponse>;
  deleteLLLMProviderConfig(req): Promise<DeleteLLMProviderConfigResponse>;
  rotateLLMProviderConfigKey(req): Promise<RotateLLMProviderConfigKeyResponse>;
}
```

**Today:** `getLLMConfigClient()` returns a ConnectRPC client backed by the
public `LLMConfigService` RPCs from #33.

**When #33 merges:**

1. Run `bun run buf:generate` to generate TypeScript from
   `proto/harpia/llm_config/v1/llm_config.proto`.
2. Create a real ConnectRPC client in `llm-config-client.ts`.
3. Update the factory to return it.
4. The page itself does **not** need to change.

See `TODO(#54-followup)` comments in `llm-config-client.ts`.

## Key Design Decisions

- **Write-only key input:** The form never echoes a stored API key. The
  `GetLLMProviderConfigs` response only returns `has_key` (bool) and
  `last_rotated_at` (timestamp). Displaying a stored key is explicitly
  prohibited by the #33 design note.
- **Error taxonomy:** The three stable error codes from the design note
  (`LLM_NO_PROVIDER_CONFIGURED`, `LLM_PROVIDER_BLOCKED_BY_PLATFORM`,
  `LLM_KEY_DECRYPTION_FAILED`) are mapped to user-facing i18n strings in
  `mapErrorCode()`. Raw codes and stack traces are never shown.
- **Quotas disabled:** The `#quotas` section is rendered as a non-interactive
  placeholder until issue #34 (usage tracking) lands. The gauge shows `—` not
  `0%` to avoid displaying a misleading value.

## i18n Keys Added

All copy goes through `translate()`. New keys added to `en.json` and `pt-BR.json`:

- `nav.settings`
- `settings.heading` / `settings.subheading`
- `settings.nav.*` (tenant, llmProviders, quotas, billing)
- `settings.tenant.*` (heading, subheading, name, namePlaceholder, slug, slugHint, managedByPlatform, managedByPlatformHint, todoRef)
- `settings.llmProviders.*` (heading, subheading, empty._, badge._, keyInput._, actions._, defaultModel.label, allowedModels._, transparency, removeConfirm._, toast._, error._)
- `settings.quotas.*` (heading, subheading, comingSoon, comingSoonDescription, monthlyBudget, hardCap, usageGauge)
- `settings.billing.*` (heading, placeholder)

## Tests

- `settings.test.ts` — AuthZ (Leader/Engineer can access, Overseer denied),
  i18n catalog completeness, copy guard (no hard-coded literals), and nav
  section registration.
- `$lib/clients/llm-config-client.test.ts` — Mock client unit tests:
  get/set/delete/rotate lifecycle, write-only guarantees (key never echoed),
  factory singleton, and client injection.
