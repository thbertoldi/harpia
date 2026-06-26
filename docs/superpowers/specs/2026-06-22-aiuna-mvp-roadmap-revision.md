# Aiuna MVP — Roadmap Revision (post-M6)

**Date:** 2026-06-22
**Status:** Locked partition; specs for each milestone written one at a time when that milestone's turn comes up.
**Supersedes:** Master design spec §10 from milestone M7 onward (M1-M6 unchanged).
**Originating conversation:** Brainstorming session 2026-06-22 between Thiago and Claude. The Aiuna PRD (`/home/thbertoldi/Downloads/Aiuna_PRD_Final_MVP_Growth.docx`) was introduced mid-session and triggered this revision.

The master spec ends at M6 (polish/IA cleanup). The Aiuna PRD describes a much larger product surface — an operational backbone with shared data layers (CRM, opportunity pipeline, knowledge base, leader dashboard, indicators, governance) sitting on top of the Plan execution engine M1-M6 builds. This doc partitions that surface into a roadmap that fits the **2-month Aiuna Growth Front MVP** target (~2026-08-22 ship), under the architectural locks established during the brainstorm.

This is the **map**, not the specs. Each milestone gets its own brainstorm + spec when its turn comes.

---

## 1. Architectural locks

These decisions shape every milestone below. See companion memory files for the long-form rationale; this section is the at-a-glance map.

### 1.1 Plans generate Artifacts; UI browses Artifacts; nothing is created out-of-band

The PRD lists camadas fixas N01-N12 (Perfil empresa, Cadastro clientes, Histórico, Catálogo, Funil, Tarefas, Aprovações, Limites autonomia, Painel líder, Indicadores, Base conhecimento, Registro ações). Naively each is its own bounded context with its own CRUD UI — five-plus milestones of plumbing.

**Decision:** collapse them into one architectural shift. Plans emit **typed Artifacts** (`type + schema + payload + area`). Persistent artifact stores back read-model browsers per type:

```
Plan execution ──produces──► typed Artifact stream
                                     │
                                     ├──► Customer Artifacts        ──► /customers
                                     ├──► Opportunity Artifacts     ──► /opportunities (funil)
                                     ├──► Knowledge Artifacts       ──► /knowledge
                                     ├──► Catalog Artifacts         ──► /catalog
                                     ├──► Calendar/Meeting Artifacts ──► /agenda
                                     ├──► Content Artifacts          ──► /content
                                     └──► Proposal Artifacts         ──► /proposals
```

No standalone CRUD UI. Ana creates a customer by running an N02-style Plan. The /customers screen is a read-model. Adding a new domain (e.g., projects) means declaring a new Artifact type + a typed browser view + (optionally) a new Plan template — all of it data, not engineering.

See [[project-artifact-architecture]].

### 1.2 Áreas are RBAC entities, not sidebar groupings

The PRD's 12 business areas (Marketing, Vendas, Direção, Administrativo, Atendimento, Financeiro, Operações, RH, Compras/Logística, TI/Dados, Jurídico/Compliance, CS) become first-class RBAC entities. Users carry Área memberships. A Marketing user sees Marketing Plans + Agents + Integrations + Artifacts; a Vendas user sees a disjoint set; multi-membership users see the union.

Persona (Ana operator / Platform Engineer) is orthogonal: persona picks the sidebar surface, Área filters the content within.

Single-seat tenants (v1 GTM) carry one user with membership in every Área — RBAC stays dormant but the substrate is in place. Multi-seat tenants benefit immediately.

Áreas also become the grain for **Limites de autonomia (PRD N08)**: per-Área autonomy policies layered over per-Task overseer.

See [[project-areas-rbac]].

### 1.3 Generic substrate, not a vertical CRM/ERP

The platform's value is the Plan / Task / Agent / Artifact / Decisão humana substrate plus the *content* (Plan templates, Agent prompts) authored on it. Vertical features (a custom CRM screen, a bespoke opportunity board) are anti-goals — they consume engineering for one tenant's flavor of work and lock the platform into someone else's mental model.

When the PRD says "Must Have" for something that smells like CRUD, check whether it expresses as a Plan emitting a typed Artifact. Usually yes.

See [[project-positioning]].

### 1.4 Scope is adaptive against a hard deadline

2-month target. Implement one milestone at a time. At month-end (~2026-07-22) take stock; if behind, simplify further — do not extend. The PRD's MoSCoW prioritization is partner input, not engineering source of truth.

See [[project-aiuna-mvp-deadline]].

### 1.5 No backwards compatibility yet

Pre-v1. Break schemas, rename routes, delete protocol fields freely. No migrations, deprecation shims, or transitional redirects until v1 cuts.

See [[feedback-no-pre-v1-compat]].

---

## 2. Milestone partition (post-M6)

```
M6 ── Lapidação                                          (≈1.5 wk)  ◀ NOW
       see docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md

M7 ── LLM providers (BYOK + dynamic models)              (≈1.5 wk)
       Platform Engineer adds a provider (Anthropic / OpenAI / Google /
       local) + API key. Tenant's available model catalog populates
       dynamically. Plans + Agents pick from it.
       Prerequisite for M9 (specialized agent authoring).

M8 ── Áreas + RBAC + typed Artifact foundation           (≈2 wk)
       Áreas as domain entities; user-area memberships; server-side
       query filtering. Artifact gets type + schema + payload + area.
       Artifact persistence (independent of chat thread).
       Per-Área autonomy policy (PRD N08) ships with the substrate.

M9 ── Artifact browsers + Plan template authoring        (≈2 wk)
       Generic typed-artifact browser + 4-5 styled views:
         /customers, /opportunities (funil), /knowledge, /catalog,
         /agenda.
       Plan template editor: Platform Engineer authors templates
       (task chain, agent bindings, artifact outputs) as content,
       not engineering. Specialized agents (Estrategista, Branding,
       Funil, Redator, Designer, etc.) are authored here too —
       agent = prompt + tools + KB access bound to a Plan slot.

M10── Authored MVP content stream                        (≈1 wk eng +
       Plans authored as templates against the M9 editor:           rolling content)
         D01 Definir objetivos
         N02-N05 (cadastro/histórico/catálogo/funil — as Plans)
         N11 Base de conhecimento (as Plan + Knowledge artifacts)
         M01, M03, M04, M05, M07, M08
         V02, V03, V04, V05, V06, V08
       Specialized agents authored as needed per template.
       Engineering work is constant-time per template after M9 ships;
       content work continues past the 2-month deadline if needed.

M11── Painel do líder + Indicadores básicos              (≈1 wk)
       PRD N09 + N10 — read-models over the Artifact stream and the
       Inbox aggregator. Today: tarefas + aprovações + alertas +
       oportunidades + indicadores básicos + próximos passos.
       Minimal first; flesh out in V1.1 if time.
```

**Total engineering wall-clock: ~9 weeks.** Tight against 8. Content (M10) parallelizes with the engineering of M11 — the second month overlaps authoring and dashboard work.

---

## 3. Confirmed cuts (decided 2026-06-22)

| Cut | Original position | Now | Rationale |
|---|---|---|---|
| Integration previews (RSS pulls live, etc.) | M7 in earlier sketch | **V1.1** | Useful for plan-authoring confidence but not required for any Must-Have plan to function. |
| Specialized-agent authoring as standalone milestone | M10 in earlier sketch | **folded into M9** | Agents become Plan-template-content; if templates are authorable, agents are too. |
| Camadas fixas as separate bounded contexts | M11-M13 in earlier sketch | **collapsed into M8 + M9** | Plans-as-source-of-truth + Artifact browsers; no separate CRUD per layer. |
| Governance hardening as standalone milestone (N08 + N12) | M14 in earlier sketch | **N08 in M8 (per-Área autonomy); N12 stays as basic /admin/audit** | Strengthens audit log in V1.1 if needed. |
| Frente Growth packaging (subscription gating) | M15 in earlier sketch | **V1.1** | Not needed to demo MVP; single-tenant pilots don't need subscription gating. |

---

## 4. PRD plans deferred from MVP to V1.1 (against PRD's Must Have list)

These are pushed to V1.1 to fit the 2-month window. Each can return to MVP if the month-end take-stock shows headroom.

| PRD code | Plan | Why deferred |
|---|---|---|
| N01 | Perfil da empresa | Ship as a single hardcoded Platform Engineer form, not as a Plan — cuts authoring overhead without losing the data. |
| N06 | Tarefas e projetos | Existing PlanConfiguration/Execution surfaces double as the task tracker; don't build a separate one. |
| N07 | Fila de aprovações | Already shipped as `/inbox` (M2). Done. |
| N12 | Registro de ações dos agentes | Minimal `/admin/audit` shipped; richer filtering and search defer to V1.1. |
| M02 | Marca e posicionamento | Folded into M01 diagnostic. |
| M06 | Plano de lançamento | PRD-classified Should Have, not Must — already deferred per PRD. Confirmed not in MVP. |

---

## 5. Out of scope for the MVP

These come from the PRD and stay out for the 2-month window:

- **Experience Front** (Atendimento + Agenda + Relacionamento) — PRD V1.1 release.
- **Operations Front** (Administrativo + Financeiro + Operações completos) — PRD Futuro.
- **Enterprise Deployment** (advanced governance, integrations, auditoria, dedicated/own-hosted) — PRD Sob projeto.
- **Financeiro** (F01-F04) — explicitly out of MVP per PRD §6.
- **RH** (RH01-RH03), **Compras/Estoque/Logística** (L01-L04), **CS** (CS01-CS03) — PRD Futuro.
- **Jurídico/Compliance avançado**, **LGPD política de IA** — PRD Enterprise/Futuro.
- **Marketplace de agentes** — PRD out-of-MVP and out of master spec §9.
- **Mobile-specific layouts** beyond responsive reduction.
- **Telemetry / analytics events** beyond what each milestone needs to function.

---

## 6. Open questions to resolve at each milestone's brainstorm

These don't block the partition but will surface when each milestone is specced.

**M7 (LLM providers):**
- Where do provider keys live — Zitadel custom claims, our own secrets store, or a new BC? (Probably our own; Zitadel is for identity, not secrets.)
- Dynamic model discovery: each vendor has a different `list models` API shape — abstract behind a port, adapter per vendor.
- Pricing surface: vendors expose per-model prices differently (or not at all). For the cost pill to stay accurate, we need a price catalog the Platform Engineer can override per provider.

**M8 (Áreas + Artifacts):**
- Áreas in Zitadel custom claims or our own DB? (Likely our own DB with Zitadel only providing identity; gives us flexibility to add/rename Áreas without re-syncing identity.)
- Artifact schema validation — JSON Schema vs protobuf-of-protobufs vs custom DSL?
- Per-Área autonomy policy shape (free-form vs structured).

**M9 (Browsers + Plan template editor):**
- Plan template editor surface — chat-driven (consistent with the rest of the platform) or form-driven (Platform Engineer is a different persona than Ana, may want different ergonomics)?
- Browser column customization per Artifact type — declarative in the type schema?

**M10 (Content):**
- Order of plan authoring — Vendas first (concrete output) or Marketing first (more visible to a demo)?
- Agent reuse — how many distinct agent prompts are actually needed, and how many templates share an agent?

**M11 (Painel + Indicadores):**
- Indicadores derivation — server-side aggregations (Postgres views) or client-side over streamed artifacts?
- Painel layout — single dashboard or per-Área dashboards?

Each becomes the opening question of its milestone's brainstorm.

---

## 7. References

- Master design spec — `docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md`
- M6 spec — `docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md`
- Aiuna PRD source — `/home/thbertoldi/Downloads/Aiuna_PRD_Final_MVP_Growth.docx`
- Memory files: [[project-positioning]], [[project-artifact-architecture]], [[project-areas-rbac]], [[project-aiuna-mvp-deadline]], [[feedback-no-pre-v1-compat]].
