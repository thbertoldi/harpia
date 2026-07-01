# Milestone D Verification — Chat-Driven Plan Proposal

**Date:** 2026-07-01  
**Commit range:** `9edbb38` (Task 1) … `c5433b9` (Task 10); verification doc in Task 11 commit.

## Step 1: Backend checks

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat ./internal/copilot ./internal/plans ./internal/threads ./cmd/api -count=1
```

```
ok  	github.com/harpia/control-plane/internal/chat	0.005s
ok  	github.com/harpia/control-plane/internal/copilot	0.004s
ok  	github.com/harpia/control-plane/internal/plans	0.046s
ok  	github.com/harpia/control-plane/internal/threads	0.004s
ok  	github.com/harpia/control-plane/cmd/api	0.007s
```

## Step 2: Proto lint

```bash
cd /home/thbertoldi/harpia/proto && buf lint
```

Exit code: 0 (no output).

## Step 3: Frontend focused tests

```bash
cd /home/thbertoldi/harpia/frontend && bunx vitest run src/lib/plans/template-inputs.test.ts src/lib/chat/client.test.ts
```

```
 ✓ src/lib/plans/template-inputs.test.ts (2 tests) 3ms
 ✓ src/lib/chat/client.test.ts (2 tests) 79ms

 Test Files  2 passed (2)
      Tests  4 passed (4)
```

## Step 4: Frontend type check (baseline)

```bash
cd /home/thbertoldi/harpia/frontend && bun run check 2>&1 | tail -3
```

```
====================================
svelte-check found 12 errors and 5 warnings in 9 files
error: script "check" exited with code 1
```

All 12 errors are in pre-existing baseline files (`auth-roles.test.ts`, `plans/artifact-flow.ts`, `canvas/ScheduleDialog.svelte`, `+layout.svelte`, `plans/[templateId]/+page.svelte`). No new errors in files created or modified by this milestone.

## Step 5: Manual smoke

**Status:** Not executed in this session — no dev stack with tenant LLM configuration was running.

Recommended manual checks against a running dev stack:

1. Go to `/new`, type "Create a LinkedIn post about retail in Portuguese", press Enter.
2. Confirm landing on `/chat/[threadId]` with a `PLAN_PROPOSED` card (LinkedIn template, inferred theme/language).
3. Adjust an input, click "Create this plan".
4. Confirm `PLAN_ATTACHED` and binding-matrix prompt; composer remains available.
5. With no LLM configured, repeat steps 1–2 and confirm "couldn't match — browse templates" graceful degradation.

## Commits (Tasks 1–10)

| Commit    | Message |
|-----------|---------|
| `9edbb38` | feat: add ProposePlan rpc and stub handler |
| `8ed06e9` | feat: add plan proposal chat payload builders |
| `ed4edff` | feat: add plan classifier port and llm adapter |
| `98b3cfb` | feat: add copilot template catalog adapter |
| `1c20211` | feat: implement ProposePlan handler and wire classifier |
| `9d7c8bd` | feat: emit PLAN_ATTACHED when a plan is created from a thread |
| `3381ce7` | feat: map new plan/artifact/error message kinds |
| `a5c3792` | feat: add generic template inputs form and helpers |
| `95fb327` | feat: add plan proposal card |
| `c5433b9` | feat: chat composer auto-proposes plans; /new prompt-first |
