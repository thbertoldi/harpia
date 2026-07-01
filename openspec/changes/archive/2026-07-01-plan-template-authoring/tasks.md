## 1. Template Loader And Validation

- [x] 1.1 Add tests for loading valid YAML templates and rejecting duplicate template keys.
- [x] 1.2 Add tests for validation failures: bad DAG, duplicate step key, unknown ArtifactType, unknown ExecutorSKU, bad runtime mapping, and adjacent artifact mismatch.
- [x] 1.3 Implement embedded YAML template structs, loader, and validation helpers.

## 2. Template Seeder

- [x] 2.1 Add tests for idempotent template upsert and child-row reconciliation.
- [x] 2.2 Implement `EnsurePlanTemplates` with database-backed ArtifactType/SKU validation and authoritative template removal.
- [x] 2.3 Run `EnsurePlanTemplates` in API startup after executor catalog seeding.

## 3. Catalog Content Migration

- [x] 3.1 Add `weekly-newsletter-linkedin` YAML with existing steps, edges, executor metadata, and input parameters.
- [x] 3.2 Add feasible `news-digest-draft` YAML using `rss-news-feed` then `newsletter-writer-senior`.
- [x] 3.3 Remove PlanTemplate data inserts/updates from migrations `000004`, `000011`, and `000013` while retaining DDL.

## 4. RSS Presets

- [x] 4.1 Add tests for dev tenant RSS preset installation create/update behavior.
- [x] 4.2 Implement curated RSS preset seeding as named `rss-news-feed` ExecutorInstallations.
- [x] 4.3 Run RSS preset seeding in API startup after dev entitlements exist.

## 5. Localization

- [x] 5.1 Add `en` and `pt-BR` i18n content keys for `news-digest-draft` and any new input labels.

## 6. Verification And Archive

- [x] 6.1 Run `openspec validate plan-template-authoring`.
- [x] 6.2 Run `cd control-plane && go test ./...`.
- [x] 6.3 Run `cd proto && buf lint`.
- [x] 6.4 Run `cd frontend && bun run check` and confirm no new errors beyond baseline.
- [x] 6.5 Run `cd frontend && bunx vitest run <files touched>`.
- [x] 6.6 Archive the OpenSpec change after all gates pass.
