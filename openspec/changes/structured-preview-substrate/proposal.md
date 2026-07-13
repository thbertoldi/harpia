## Why

Harpia Artifact identity, versioning, lineage, object storage, and tenant-scoped lookup are reusable, but previewing is not: the control plane and frontend each select a fixed set of payload-shaped variants. That prevents any PlanTemplate—including a future governed Odoo question-and-answer plan—from safely presenting first-class tables, charts, or composite content without forcing it through arbitrary HTML.

This change establishes the platform-owned structured-preview substrate now, before rich data-producing plans multiply incompatible one-off preview implementations.

## What Changes

- **BREAKING** extend the artifacts protobuf contract with a generic `StructuredPreview` envelope and `ArtifactType` renderer declaration; preserve the existing fixed preview variants only during this pre-v1 cut.
- Add an allowlisted control-plane `PreviewProjector` registry that resolves an `ArtifactType` to a platform-registered projector, validates its structured preview, and falls back to the current fixed preview handling and then JSON for unknown types.
- Add validated, reusable `TabularData` and `ChartSpec` typed artifact payloads, including typed columns, rows, tabular provenance/freshness metadata, and a pinned tabular ArtifactRef for charts.
- Add an allowlisted frontend renderer registry keyed by `renderer_key`, with native accessible table, Flint-backed chart, carousel, sanitized document, markdown, image, and JSON-fallback renderers. Artifacts can select no code, URL, or module.
- Migrate existing carousel, markdown, and image preview paths onto the registry where their existing contracts map cleanly; retain non-migrated fixed variants until a later incremental migration.
- Harden `document.html` preview rendering with sanitization and a restrictive CSP/network policy in addition to iframe sandboxing.
- Document the forward integration boundary: a later Odoo Q&A PlanTemplate may snapshot scoped downstream query results into tenant-owned `TabularData` and optional `ChartSpec` Artifacts, without storing credentials or mirroring Odoo.

## Capabilities

### New Capabilities
- `structured-preview-projection`: Contract, typed payload validation, and tenant-safe server projection of platform-registered structured Artifact previews.
- `structured-preview-renderers`: Allowlisted client renderer registry for structured previews, including accessible tables, Flint charts, safe documents, and graceful fallback.

### Modified Capabilities
- `artifact-preview-panel`: Replace the fixed preview-kind registry contract with the structured renderer-key contract and strengthen HTML preview isolation requirements.

## Impact

- **Contracts:** `proto/harpia/artifacts/v1/artifacts.proto`, generated protobuf/Connect consumers, and artifact type registration APIs.
- **Control plane:** `control-plane/internal/artifacts/preview.go`, validation, artifact type repository/handler logic, and focused artifact preview tests; projector implementations stay in the artifacts adapter layer rather than domain packages.
- **Frontend:** `frontend/src/lib/artifacts/preview.ts`, artifact preview components/tests, i18n locale files, and a new pinned `flint-chart` npm dependency plus its rendering adapter.
- **Security:** server-side/renderer document sanitization, CSP policy, asset reference authorization, and regression coverage for untrusted markup and network egress.
- **Architecture:** follows the Artifact/Resource and downstream-system ownership boundaries in Platform Constitution §§8 and 15; it does not introduce an Odoo executor, credentials, a downstream mirror, or a new system of record.
