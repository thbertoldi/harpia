## Why

PlanTemplate catalog content is currently embedded in SQL migrations, which makes a new template a schema change and leaves DAG, artifact, executor, and runtime input wiring errors to surface late. ADR-015 accepts YAML-authored templates plus a startup seeder so the catalog can grow as validated content while preserving ADR-012's execution snapshot invariant.

## What Changes

- Add declarative YAML PlanTemplate files under `control-plane/internal/plans/templates/`, embedded into the API binary.
- Add an idempotent startup seeder that validates and reconciles `plan_templates`, `plan_template_steps`, `plan_template_step_dependencies`, and template `input_parameters`.
- **BREAKING** Remove PlanTemplate data inserts/updates from existing migrations `000004`, `000011`, and `000013`, leaving DDL intact.
- Migrate `weekly-newsletter-linkedin` into YAML and add a feasible `news-digest-draft` template using only existing SKUs.
- Add localized catalog copy for the new template and any new input labels in `en` and `pt-BR`.
- Seed curated RSS feed presets as tenant ExecutorInstallation configs for the existing `rss-news-feed` SKU.

## Capabilities

### New Capabilities

- `plan-template-authoring`: YAML-authored PlanTemplates are loaded, validated, and reconciled into the global plan catalog at startup.
- `rss-feed-presets`: Curated RSS source groups are available as named `rss-news-feed` ExecutorInstallations with `{feeds: [...]}` config.

### Modified Capabilities

- None.

## Impact

- Backend: `control-plane/internal/plans`, API startup seeding, template tests, and migration seed cleanup.
- Content: embedded YAML template catalog and RSS preset definitions.
- Frontend: i18n content keys for new template/step/input copy.
- Database: existing template rows become startup-seeded content rather than migration-seeded content; no new migration is expected for pre-v1 cleanup.
