# Conversational Plan Workspace Verification

**Date:** 2026-06-30

| Check | Result | Notes |
|---|---|---|
| `proto: buf lint` | PASS | Exit 0. |
| `control-plane: go test ./...` | PASS | Exit 0. |
| `agent-runtime focused pytest` | PASS | `11 passed in 0.84s` after language prompt fixes. |
| `frontend focused vitest` | PASS | `32 passed` across template inputs, artifacts, activity, assistant, preview, and matrix tests. |
| `frontend bun run check` | BASELINE FAIL | Still reports the known 12 errors and 5 warnings in auth role tests, artifact-flow, ScheduleDialog, layout route typing, template route params, and existing Svelte state warnings. No new modified-file errors remain. |
| dev DB migration apply | PASS | `000013_template_inputs_artifact_versions.sql` applied successfully to local dev Postgres. |
| manual app smoke | PASS WITH CAVEAT | Created configuration `3a0ea69a-0a1e-4d9f-842d-f8a016a6449c`, applied `theme=retail growth` and `language=en-US`, started execution `bd44e7a2-23d6-4d50-8c13-e5084ed063f0`, generated English LinkedIn artifact `4d0e6378-2adf-42ca-9c9b-712eddde86cd`, saved version 2 with a manual text line, and verified preview/history. Caveat: the already-open Playwright page did not receive final stream updates before timeout, but a fresh load showed `Ready for approval` and `4 of 4 bound`. |

## Smoke Notes

- AIUNA branding and Ana dev session rendered on `/new`.
- Generated LinkedIn post text was in English and referenced retail-growth content.
- Artifact preview no longer sends an empty `artifact_version_id`; the invalid UUID preview error is fixed.
- Binding matrix reload now hydrates saved bindings from the current configuration instead of showing `0 of 4 bound`.
- Remaining follow-up: investigate watch-stream resiliency because one long-lived browser session stayed at `Write Draft is running` even though the workflow reached approval in Temporal/DB and a reload showed the final state.
