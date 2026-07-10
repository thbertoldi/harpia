# Architecture Decision Records — historical index

> **These ADRs are dated historical record, not the current source of truth.**
> The canonical description of the platform is the
> **[Platform Constitution](../architecture/harpia-platform.md)**, which supersedes
> ADR-001…017 as the reading order. Where an ADR and the constitution disagree, **the
> constitution wins.** Read the constitution first; consult an ADR only for the *rationale*
> behind a decision.

New architectural decisions still start as an ADR here; when accepted, their outcome is
folded into the constitution and the ADR is stamped historical (see constitution §17).

## Supersession map

Arrow = "later ADR changed the direction of the earlier one." This is the least-whiplash
reading order.

- **ADR-002 → ADR-007 → (ADR-011 §7, ADR-014)** — supervisor graph gained scorer/router
  (007); MCP discipline refined (011); memory boundary sharpened (014).
- **ADR-002 → ADR-009** — state-merge convention formalized.
- **ADR-001 → ADR-013** — Python ConnectRPC appendix; ADR-001's three-mode table stays authoritative.
- **ADR-006 → ADR-010** — Budget is a *capability* inside Agent Orchestration, not a new context.
- **ADR-006 / ADR-002 / ADR-007 → ADR-012** *(major)* — Task Management → Plan Management;
  new Artifact Types context; Task→PlanExecution, Subtask→StepExecution; planner demoted to
  adaptive mode; `harpia.tasks.v1` deprecated.
- **ADR-012 → ADR-015** — template *data* moved from SQL migrations to declarative YAML.
- **ADR-012 → ADR-017** — 1:1 thread↔configuration superseded by 1:N; adds `kind` + `origin_thread_id`.
- **AGENTS.md "tokens LOCKED" → ADR-016** — color/depth unlocked (governed); fonts still locked.

## Status of each ADR vs the constitution

| ADR | Subject | Standing |
|---|---|---|
| 001 | API & communication | Live (constitution §6). |
| 002 | Workflow & agents | Amended — graph & modes (§7.5). |
| 003 | Data architecture | Live (§6); task/subtask hierarchy renamed (§4). |
| 004 | Identity & access | Amended — OpenFGA/ReBAC authz layer retired; RLS + Zitadel roles + thin app checks (§6, §11). Zitadel AuthN + RLS still live. |
| 005 | Dev & delivery | Live (§6). |
| 006 | Domain-driven design | Amended — six contexts, Plan Management (§5). |
| 007 | Agentic patterns | Amended — refined by 011/014; gates scoped to adaptive mode (§7.5). |
| 008 | Tenant-safe boundaries | Live (§6). |
| 009 | LangGraph state semantics | Live — adaptive mode only (§6). |
| 010 | Budget policy service | Live — budget = capability (§5); its "five contexts" claim corrected to six. |
| 011 | MCP capability gating | Live (§5). |
| 012 | Plan-centric task model | Core, amended by 015/017 (§7, §9). |
| 013 | ConnectRPC Python | Live (§6). |
| 014 | Agent memory boundary | Live, generalized — MemoryResource → **Resource** (§8). |
| 015 | PlanTemplate authoring | Live (§13); Áreas RBAC now membership-based, not OpenFGA (§11). |
| 016 | Design token refresh | Live (§10). |
| 018 | Capability-based agent teams (roles × tiers) | **Accepted** — folded into §4/§7.2/§13. Supersedes the one-SKU-per-step model built under `rich-linkedin-content`. |
| 017 | Navigation & lifecycle | Live (§9). |

## Known contradictions resolved by the constitution

- **Five vs six bounded contexts** (ADR-010 vs ADR-012) → **six** (constitution §5).
- **Chat assistant singular vs plural** (ADR-012 §10 vs ADR-017) → **plural / 1:N** (§9).
- **Overseer gate universal vs template-mode** (ADR-007 vs ADR-012) → **mode split** (§7.5).
- **`subtask_*` nouns in new protos** (ADR-010/011) → rename to `step_*`
  ([cleanup backlog](../architecture/cleanup-backlog.md), C4).
- **Áreas RBAC vs OpenFGA** (ADR-015 vs ADR-004) → **Áreas are membership-based RBAC** (Zitadel roles + RLS + thin app checks); OpenFGA/ReBAC retired (§11).
