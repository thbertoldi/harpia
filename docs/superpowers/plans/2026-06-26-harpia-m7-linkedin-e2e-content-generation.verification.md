# M7 LinkedIn E2E Content Generation — Verification Log

**Date:** 2026-06-27
**Branch:** `feat/ux-realignment-m6-lapidacao` (M7 commits `a95bebf`…`8ec3b49`)
**Plan:** `docs/superpowers/plans/2026-06-26-harpia-m7-linkedin-e2e-content-generation.md`
**Spec:** `docs/superpowers/specs/` (M7 LinkedIn e2e content generation spec, `92b0780`)

## Status: M7 implementation complete; full-suite verification green; awaiting Monday rehearsal + human review.

Demo target: **Monday 2026-06-29** — Platform Engineer configures DeepSeek +
grouped sports RSS, Ana selects the LinkedIn plan, uses Suggest, runs it, and a
publish approval lands in `/inbox` (approval-only dry-run, no public publish).

---

## Task completion

| Task | Scope | Status | Commit |
|---|---|---|---|
| 1 | DeepSeek agent-runtime provider (OpenAI-compatible) | ✓ | `a95bebf` (+`c773841`, `5c3e92a`) |
| 2 | DeepSeek BYOK admin settings | ✓ | `952130e` |
| 3 | Grouped RSS feed installations UI | ✓ | `c2b05fa` |
| 4 | LinkedIn approval-only dry-run | ✓ | `975136c` |
| 5 | Suggest LinkedIn sports plan configuration | ✓ | `b52d7bf` |
| 6 | Agent-runtime artifact bridge (load/persist) | ✓ | `496aea7` |
| 7 | Python worker owns `RunAgentActivity` | ✓ | `df17b72` |
| 8 | Inbox publish-approval visibility | ✓ | `3bcfe4c` |
| 9 | E2E demo journey | ✓ | `7b7e6e0` |

## Test + lint status

- **control-plane:** `go test ./...` → all packages **ok, 0 failures**.
- **Frontend Vitest:** `bun run test` → **289/289 passed** (48 files; incl.
  llm-config-client, executor-installations, plans/assistant, inbox/aggregator).
- **agent-runtime:** `uv run --project agent-runtime pytest -q` → **66/66 passed**
  (incl. deepseek provider, tenant-credential routing, temporal agent activity).
- **Frontend types:** `bun run check` → **12 errors, all PRE-EXISTING** (same set
  documented in the M6 verification log; predate the branch). None in M7 feature
  files; `BindingMatrixCard.svelte` carries only state-locality warnings.
  Out of M7 scope.

## Bug found + fixed during verification (root-caused, not patched)

1. **Dev environment failed to bootstrap** (`0614bd5`) — `zitadel-register-client`
   Job hit `BackoffLimitExceeded`. Root cause: the branded email message-texts
   step PUT locale `pt-BR` to Zitadel's `/management/v1/text/message/{type}/{language}`,
   which Zitadel rejects (`Language is not supported`); under `curl -fS` the
   non-2xx aborted the bootstrap Job, so the demo's dev stack never came up.
   Fixed by normalising `pt-BR`→`pt` and switching source data to the `pt` code
   in both the dev kind manifests and the prod Helm chart; email HTML/TXT
   templates match either `pt` or browser-sent `pt-BR`. Re-run → Job `Complete`,
   `harpia-oidc-config` published, `helm lint` clean. **On the M7 critical path**
   (rehearsal needs the dev stack) though not a planned task.
2. **`pytest -q` from repo root flaked one test** (`8ec3b49`) — invocation-only,
   not a code regression. `test_tenant_resolver_invokes_internal_rpc_with_tenant_headers`
   relied on `asyncio_mode=auto`, which is only loaded when pytest's rootdir
   resolves to `agent-runtime/pyproject.toml`. Run unscoped from repo root, the
   config isn't picked up and the unmarked async test errors. Added an explicit
   `@pytest.mark.asyncio` (matching the rest of the suite) so the plan's exact
   command is cwd-independent.

## Manual verification still recommended (Monday rehearsal)

Plan Task 9 Step 5 — verify live against the (now-bootstrapping) dev stack:
1. Platform Engineer saves DeepSeek (`deepseek-v4-flash`) in `/admin/settings`.
2. Creates a Sports RSS group (+ optional HN group) in `/admin/integrations`.
3. Sets LinkedIn Publish to **Approval only**.
4. Ana opens `/new`, picks the LinkedIn newsletter plan, clicks **Suggest**,
   enters `sports`, applies → 4/4 steps bound.
5. Saves and runs; `fetch-news` → `write-draft` → `adapt-for-linkedin` produce
   real artifact IDs; `publish-linkedin` pauses for approval.
6. `/inbox` shows the pending approval with a LinkedIn-draft preview pointer.

(The Playwright `engineer-journey.spec.ts` exercises this path with mocks; the
live run confirms the real Temporal + agent-runtime + artifact wiring.)

## Known follow-ups (out of M7 scope, flagged not silently dropped)

1. **Pre-existing svelte-check errors (12)** — predate the branch; separate cleanup.
2. **Plan verification command** — the plan lists `uv run --project agent-runtime pytest -q`
   from repo root, which collects 8 extra non-agent-runtime tests and depends on
   rootdir config. Scope to `agent-runtime` (or cd in) for a clean 58. The
   `@pytest.mark.asyncio` fix removes the failure regardless.
3. **Zitadel real assets** — final Bodoni Moda wordmark + WOFF2 fonts (see
   `deploy/dev/kind/assets/NOTES.md`); current logo is a serif-fallback placeholder.
4. **DeepSeek pricing** — intentionally zero in the registry for Monday; real
   prices are M8 scope.
5. **Real LinkedIn OAuth publish** — deferred; Monday ends at approval (dry-run).
