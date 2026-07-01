## Context

ADR-012 defines PlanTemplate as a global catalog DAG of PlanSteps and requires PlanExecution snapshots to isolate runs from later template edits. ADR-015 accepts an MVP authoring model where engineer-authored YAML files are embedded into the control-plane binary and reconciled into the existing plan template tables at API startup.

The current catalog has one template, `weekly-newsletter-linkedin`, spread across migrations `000004`, `000011`, and `000013`. The executor catalog is already startup-seeded by `executors.EnsureCatalog`; artifact types are still migration-seeded. RSS feed URLs must live in `ExecutorInstallation.config_json` as `{feeds: [...]}`, not in templates or artifact payloads.

## Goals / Non-Goals

**Goals:**

- Move PlanTemplate row content out of SQL migrations and into embedded YAML files.
- Validate template structure before database reconciliation: DAG, duplicates, known ArtifactTypes, known ExecutorSKUs, runtime mappings, and linear step contract continuity.
- Reconcile the database catalog idempotently on startup, including child rows and deletion of templates removed from YAML.
- Preserve `weekly-newsletter-linkedin` and add `news-digest-draft`, a feasible two-step template using `rss-news-feed` and `newsletter-writer-senior`.
- Seed curated RSS presets as named `rss-news-feed` ExecutorInstallations for the dev tenant.
- Add required localized content keys for new catalog copy.

**Non-Goals:**

- No runtime PlanTemplate CRUD service, authoring UI, template marketplace, or tenant-specific template forks.
- No compatibility shim for SQL-seeded template content; pre-v1 allows a single authoring path.
- No new ExecutorSKU. Blog, email, X/Twitter, Instagram, and other channel templates remain blocked-on-new-executors.
- No migration solely to delete template rows from already-applied dev databases; the startup seeder is authoritative.

## Decisions

1. **YAML schema mirrors the persisted and proto-facing shapes.**
   Files declare `key`, `name`, `description`, `vertical`, integer `version`, ordered `steps`, `edges`, and `input_parameters`. `executor_requirement`, `default_executor_sku_key`, and input parameter runtime mappings use the same JSON field names currently persisted in `input_parameters` so `templateToProto` continues to decode without a proto change.

2. **Validation is load-time and database-backed for external references.**
   The loader validates intrinsic file structure first, then `EnsurePlanTemplates` queries the database for all referenced ArtifactType keys and ExecutorSKU keys. Missing references fail startup. The seeder must run after `executors.EnsureCatalog` so SKU validation sees the authoritative catalog.

3. **Reconciliation replaces child rows for each YAML template.**
   Upsert `plan_templates` by `key`, delete that template's existing steps and dependencies, then insert the YAML child rows with deterministic positions. This avoids drift in deleted/renamed child rows. After all files are applied, delete `plan_templates` whose keys are absent from the embedded set. Cascades clean up removed child rows.

4. **Runtime mapping validation uses the current MVP target vocabulary.**
   `SEED_ARTIFACT` mappings must target a known step and include `inputName`; `SLOT_BINDING` mappings must target a known step; `BEHAVIOR_POLICY` mappings must target an allowlisted policy key (`publish_approval_mode`, `elicitation_timeout_behavior`). Unknown targets or missing required fields fail validation.

5. **RSS presets surface as seeded named installations.**
   Add an `EnsureTenantRSSPresetInstallations` startup step for the dev tenant. It finds the `rss-news-feed` SKU and upserts installations by display name with `kind=integration`, `enabled=true`, `connection_status=connected`, and config JSON `{feeds:[...]}`. Existing chat/configuration UI can offer these through the existing integration selector/slot-binding path, preserving the ADR-012 rule that templates do not carry feed URLs.

6. **Feed verification results.**
   Verified 200 + RSS/Atom-looking XML and kept:
   - Tech/startup: `https://techcrunch.com/feed/`, `https://www.theverge.com/rss/index.xml`, `https://feeds.arstechnica.com/arstechnica/index`, `https://hnrss.org/frontpage`
   - Business: `https://sloanreview.mit.edu/feed/`
   - Marketing/creator: `https://www.socialmediatoday.com/feeds/news/`
   - Brazil/pt-BR: `https://www.infomoney.com.br/feed/`, `https://exame.com/feed/`, `https://tecnoblog.net/feed/`, `https://canaltech.com.br/rss/`, `https://startupi.com.br/feed/`

   Dropped after verification:
   - HBR: `https://hbr.org/feed`, `https://hbr.org/topic/feed`, `https://hbr.org/the-latest/feed`, `https://hbr.org/rss`, `https://hbr.org/web/feed` returned 404/non-feed.
   - Reuters Business: `https://www.reutersagency.com/feed/?best-topics=business-finance&post_type=best`, `https://www.reuters.com/business/rss`, `https://www.reuters.com/markets/rss`, `https://feeds.reuters.com/reuters/businessNews`, `https://www.reuters.com/arc/outboundfeeds/rss/category/business/?outputType=xml` returned 404/401/network failure/non-feed.
   - Content Marketing Institute: `https://contentmarketinginstitute.com/feed/`, `https://contentmarketinginstitute.com/feed/rss/`, `https://contentmarketinginstitute.com/articles/feed/`, `https://contentmarketinginstitute.com/feed/rss` returned non-feed or 404.

## Risks / Trade-offs

- **Deleting templates absent from YAML could remove local experiments** -> This is intentional for pre-v1 authoritative content; local experiments must be committed as YAML.
- **Replacing child rows changes child UUIDs** -> Slot bindings use step keys and executions snapshot configurations; current runtime does not depend on stable template step UUIDs.
- **Startup now depends on artifact type seed order** -> Existing migrations seed ArtifactTypes before API startup; tests should cover unknown ArtifactType failures.
- **RSS preset installation upsert by display name is a weak identity** -> There is no installation key column today. Prefix names consistently (`RSS Preset: ...`) and only reconcile those names.
- **Business and Marketing preset groups are sparse after validation** -> Keep only live feeds and document dropped sources rather than seeding dead URLs.
