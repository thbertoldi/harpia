# Tenant LLM Provider Config + BYO Keys (Issue #33) - Stage 1 Design Note

Date: 2026-06-16  
Scope: control-plane design with a thin agent-runtime resolver consumer (no implementation in this stage)

## 1) Goals and non-goals

### Goals

- Introduce tenant-scoped LLM provider configuration so each tenant can use its own provider credentials.
- Store provider API keys encrypted at rest using an envelope model (per-row DEK wrapped by KEK).
- Support KEK rotation without re-encrypting tenant API keys or rewriting all rows.
- Enforce provider resolution fallback chain: tenant key -> platform key -> terminal error.
- Prevent raw API keys from being emitted in logs, traces, panic strings, or user-visible error messages.
- Enforce admin-only mutation APIs with app-level role checks (`admin`) plus tenant scoping.
- Preserve forward compatibility for new providers and managed modes without schema rewrites.
- Keep runtime contract explicit so control-plane resolves per-tenant provider settings safely.

### Non-goals

- Building an LLM proxy/gateway in this issue (LiteLLM proxy remains parked decision; see section 10).
- Implementing per-call provider/model cost attribution or billing logic (covered by #34).
- Shipping tenant settings UI or self-service frontend workflow (covered by #54).
- Designing cross-region key replication or HSM integration in this milestone.
- Solving full cluster compromise; envelope encryption is blast-radius reduction, not absolute secrecy.
- Supporting provider-specific advanced options beyond key + default/allowed model configuration.
- Reworking existing plan-time model selection logic outside required resolver integration points.
- Adding background key rotation jobs or secret orchestration operators in stage 1.

## 2) Threat model

| Actor | Asset at risk | Mitigation |
|---|---|---|
| Tenant admin (legitimate, scoped actor) | Raw provider key for their own tenant | Handler enforces admin role check; mutation/read-metadata operations are scoped to tenant; APIs never return raw keys, only `has_key` and metadata. |
| Platform operator | Raw provider keys across tenants | Access mediated by control-plane service and k8s secret permissions; no plaintext keys in DB; logging redaction/lint guard reduces accidental exposure. |
| Malicious tenant member (non-admin) | Raw provider key for their tenant | Mutating RPCs require `tenant.admin`; RPC middleware enforces tenant context; DB RLS blocks cross-tenant row access. |
| Attacker with read-only DB access | Encrypted tenant keys and wrapped DEKs | Envelope encryption: DB only stores ciphertext + wrapped DEK + KEK version; attacker cannot decrypt without KEK material. |
| Attacker with full cluster access | Raw provider keys | Not fully mitigated: attacker can likely read KEKs and process memory; envelope encryption does not protect against full cluster compromise. It still improves rotation and limits DB-only breach impact. |

## 3) Encryption envelope

This design uses a two-layer envelope:

1. Generate a random per-row DEK (32 bytes).
2. Encrypt provider API key with DEK using AES-256-GCM and random 12-byte nonce.
3. Encrypt (wrap) DEK with active KEK version using AES-256-GCM and independent random 12-byte nonce.
4. Persist ciphertext blobs plus `kek_version`.

Inner payload (API key encryption) byte layout:

- `nonce || ciphertext || tag`

Outer payload (DEK wrapping) stored with explicit version:

- `kek_version || encrypted_dek_nonce || encrypted_dek || encrypted_dek_tag`

KEK management:

- KEKs live in Kubernetes Secret `aiuna-llm-kek` with versioned entries (`v1`, `v2`, ...).
- Control-plane loads all KEK versions at startup; one version is marked active for new writes.
- Encryption always uses active KEK version.
- Decryption chooses KEK by `kek_version` referenced in each row.

Rotation procedure:

1. Add `v(N+1)` to secret.
2. Restart control-plane pods so keyring refreshes.
3. Mark `v(N+1)` active for encryption.
4. Existing rows remain decryptable under historical versions.

No row-level re-encryption is required during KEK rotation, satisfying issue acceptance criteria.

Cryptography implementation choice:

- Use Go standard library (`crypto/aes` + `crypto/cipher` with AES-256-GCM).
- Rationale: no additional dependency surface, audited primitives, predictable maintenance profile, and sufficient performance for control-plane key operations.

## 4) Database schema

Migration approach:

- Use Atlas migration flow already adopted in `database/`.
- Planned migration location in stage 2: `database/migrations/` with standard timestamped naming consistent with repo conventions.

Proposed SQL sketch:

```sql
create table tenant_llm_configs (
  id uuid primary key default gen_random_uuid(),
  tenant_id uuid not null references tenants(id) on delete cascade,
  provider text not null,
  kek_version text not null,
  encrypted_dek bytea not null,
  encrypted_api_key bytea not null,
  default_model text null,
  allowed_models text[] not null default '{}',
  managed_by text not null check (managed_by in ('platform', 'tenant_self')),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index tenant_llm_configs_tenant_provider_uk
  on tenant_llm_configs (tenant_id, provider);

-- RLS (mirrors tenant-scoped pattern in existing migrations)
alter table tenant_llm_configs enable row level security;

-- Example policy shape, bound to app tenant context setting
create policy tenant_llm_configs_tenant_isolation on tenant_llm_configs
  using (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
  with check (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- Grants aligned with app role model
grant select, insert, update, delete on tenant_llm_configs to app_user;
```

Notes:

- `provider` remains string-based (enum-like values such as `anthropic`, `openai`, `ollama`) for forward provider compatibility.
- `allowed_models = '{}'` means "no extra tenant restriction" beyond provider support and platform policy.
- `managed_by` reserves semantic extension space without future schema rewrite.

## 5) Proto / RPC contract (signatures only)

Recommendation: add a new API surface at `proto/harpia/llm_config/v1/llm_config.proto`.

Justification:

- Keeps identity tenant/member semantics separate from provider secret/config lifecycle.
- Aligns with bounded-context separation (Identity/Tenants vs Agent Orchestration policy/config).
- Reduces coupling and avoids further growth of `identity.proto`.

Proposed signatures (public):

```proto
rpc SetLLMProviderConfig(SetLLMProviderConfigRequest) returns (SetLLMProviderConfigResponse);
rpc GetLLMProviderConfigs(GetLLMProviderConfigsRequest) returns (GetLLMProviderConfigsResponse);
rpc DeleteLLMProviderConfig(DeleteLLMProviderConfigRequest) returns (DeleteLLMProviderConfigResponse);
rpc RotateLLMProviderConfigKey(RotateLLMProviderConfigKeyRequest) returns (RotateLLMProviderConfigKeyResponse);
```

Contract rules:

- `SetLLMProviderConfigRequest` includes write-only `api_key` field; responses never echo it.
- `GetLLMProviderConfigsResponse` includes only metadata: `provider`, `default_model`, `allowed_models`, `has_key`, `managed_by`, `last_rotated_at`.
- No encrypted blob or plaintext key is ever exposed via public RPC.

Internal resolver (not public RPC):

- Recommended MVP path: option (a), ConnectRPC internal endpoint `ResolveLLMProviderForTenant`.
- Returns a short-lived opaque credential bundle for one workload execution context; control-plane decrypts server-side and transmits over authenticated internal channel (mTLS).
- Rationale: smallest change to runtime while preserving explicit control-plane ownership of key material and fallback policy.
- Option (b), full control-plane proxying actual LLM calls, is deferred as future LiteLLM-proxy direction.

## 6) Fallback chain

Resolution order for `(tenant_id, provider, model)`:

1. Tenant-scoped key/config if present and permitted.
2. Platform-scoped provider key/config if tenant key absent.
3. Terminal error if neither is available.

Stable error envelope codes:

- `LLM_NO_PROVIDER_CONFIGURED`
- `LLM_PROVIDER_BLOCKED_BY_PLATFORM`
- `LLM_KEY_DECRYPTION_FAILED`

Behavioral notes:

- If platform policy blocks provider (for tenant or globally), return `LLM_PROVIDER_BLOCKED_BY_PLATFORM` even if tenant has no key.
- Platform keys should live in Kubernetes Secrets (mounted file or env projection), with refresh on pod restart; avoid plaintext in static config maps.

## 7) Log scrubbing - lint enforcement

Planned safeguards:

- Add CI check script under `scripts/ci/` (stage 2) scanning Go/Python non-test sources for probable key patterns (`sk-ant-...`, `sk-...`, and provider-specific prefixes) and fail build on matches.
- Define Go wrapper type `secret.Redacted` whose `String()`, `%v` formatting, and `MarshalJSON` return masked values (e.g., `***REDACTED***`).
- Route decrypted key flows through `secret.Redacted` in internal service boundaries, including resolver and rotate-key paths.
- Add Python `RedactedSecret` dataclass in `agent-runtime/src/harpia_agents/llm/secrets.py` with masked `__repr__`/`__str__`.

## 8) AuthZ

Authorization model:

- Mutating RPCs (`Set`, `Delete`, `Rotate`) require `tenant.admin` permission
  (app-level role check via `users.role`, not OpenFGA — see constitution §11).
- Read metadata RPC (`Get`) follows tenant membership policy but never reveals secret value.

Defense in depth:

- RPC middleware enforces tenant context and actor membership.
- DB RLS enforces tenant row isolation even if middleware is misconfigured.

## 9) Test plan (planned only; no stage 1 implementation)

- Unit tests for envelope encryption/decryption: correct round-trip, nonce uniqueness, tamper detection, wrong-KEK failure.
- Integration test for KEK rotation: old row decrypts after active KEK advances; new rows use latest version.
- RLS isolation test: cross-tenant query/update attempts denied.
- Handler tests for ConnectRPC methods: authz failures, metadata-only responses, error code stability.
- Lint regression tests for log scrubbing script across Go and Python fixture files.
- Python resolver client test validating internal resolver contract and redacted logging behavior.

## 10) Forward compatibility - LiteLLM Proxy

LiteLLM Proxy adoption is intentionally out of scope for #33 and not pre-architected here. The only explicit compatibility hook is `managed_by`: if proxy mode is adopted later, rows can use `managed_by='proxy_virtual'`, `encrypted_api_key` can hold proxy-issued virtual credentials, and resolver routing can switch to proxy-backed execution without rewriting historical schema or tenant config APIs.

## 11) Open questions

- What KEK rotation cadence should operations enforce (for example, fixed 30/60/90-day schedule vs incident-driven), and who owns runbook execution?
- Should `allowed_models` be strict deny-by-default when empty for high-security tenants, or remain permissive as designed (`{}` means no extra restriction)?
- Do we want to support tenant-specific `default_model` overrides only at this config layer, or should plan-time `PlanConfiguration` always be authoritative when provided?
- Should `ollama` entries permit non-secret local endpoint-only configuration without encryption, or must all providers stay on the same encrypted path for consistency?

## 12) Rough implementation slicing (stage 2 plan)

- Add/update proto contracts and run `buf generate`.
- Add Atlas migration for `tenant_llm_configs` with RLS and unique index.
- Implement envelope encryption module + focused unit tests.
- Implement repository layer + ConnectRPC handlers + handler/integration tests.
- Implement agent-runtime internal resolver client and redacted secret wrapper.
- Add CI secret-pattern scrubber lint script and test fixtures.
- Update docs for operations runbook (KEK rotation) and tenant admin usage.
