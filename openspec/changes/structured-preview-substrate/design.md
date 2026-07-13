## Context

Artifacts already provide the reusable substrate required by Platform Constitution §8: typed payloads with identity, immutable versions, hashes, lineage, object storage, and tenant-scoped lookup. The preview boundary does not share that maturity. `BuildPreview` is a closed type switch supplemented by HTML/image payload-shape heuristics; `PreviewArtifactResponse` has a fixed `oneof`; and the frontend formats that `oneof` through another switch. Existing composite LinkedIn carousel support is deliberately narrow and cannot establish a general renderer contract.

The change spans the protobuf contract, artifact registry metadata, database schema, control-plane adapters, generated clients, and Svelte preview UI. It must preserve the existing Artifact/Resource ownership model, keep infrastructure and provider SDKs out of domain packages, retain tenant boundaries, and avoid treating Aiuna as an Odoo mirror. Flint is an MIT JavaScript library that compiles a compact semantic spec and inline data to backend-native chart options; it is appropriate only as a statically imported frontend rendering dependency, never as an artifact-supplied executable.

## Goals / Non-Goals

**Goals:**

- Make a server-authoritative, versioned structured preview available for any registered ArtifactType through platform-owned allowlists.
- Introduce reusable `TabularData` and `ChartSpec` Artifact payload contracts that retain typed tabular semantics, pinned provenance, and an immutable chart-to-data relationship.
- Render tables, Flint charts, carousels, documents, markdown, and images through an allowlisted Svelte component map; unknown renderer keys must remain safe and inspectable.
- Make HTML documents safe against active content and data egress with sanitization plus a restrictive `srcdoc` CSP/network policy, not iframe sandboxing alone.
- Permit an eventual Odoo Q&A PlanTemplate to produce tenant-owned result snapshots without prescribing or implementing that plan.

**Non-Goals:**

- Implementing the Odoo Q&A PlanTemplate, an Odoo executor, ExecutorInstallation credentials, or an Odoo data mirror.
- Replacing every legacy fixed preview in a big-bang migration; only carousel, markdown, and image paths move when their current data maps cleanly.
- Building a generic editor, a remote renderer/plugin system, user-authored renderer code, or artifact-supplied component URLs/modules.
- Changing the execution-driven-conversation work or plan execution semantics.

## Decisions

### 1. Extend the existing preview response during the pre-v1 contract cut

`artifacts.proto` adds:

```proto
message StructuredPreview {
  string renderer_key = 1;
  int32 renderer_version = 2;
  bytes spec_json = 3;
  repeated ArtifactRef assets = 4;
}
```

`PreviewArtifactResponse.oneof preview` receives `StructuredPreview structured_preview = 8`; fields 1–7 remain generated and readable for the incremental cut. `ArtifactType` and `RegisterArtifactTypeRequest` gain `preview_renderer_key` and `preview_renderer_version`. The global `artifact_types` row and the Go seed model persist the same metadata so the declaration is queryable rather than inferred from a payload.

This is a deliberate pre-v1 breaking schema/API change: generated Go, TypeScript, and Python consumers update together with the database migration and seeds. The alternative—encoding renderer selection in each artifact payload—would let untrusted producer data select behavior and would not make type capabilities discoverable. Retaining the legacy variants temporarily avoids a high-risk all-preview rewrite while the generic envelope becomes canonical for new structured types.

### 2. A platform-owned projector registry resolves declared renderer keys

The artifacts adapter owns a `PreviewProjector` interface and registry. A registered projector has a stable renderer key/version, accepts the validated payload for its ArtifactType, produces a JSON-only renderer spec plus `ArtifactRef` assets, and validates that output before it enters `StructuredPreview`. Application composition registers the finite built-in projectors; neither an Artifact payload nor a tenant can register code or change the registry.

`PreviewArtifact` loads the Artifact and requested immutable version under the caller tenant before loading its ArtifactType. It resolves the type declaration to a registered projector, validates the resulting key/version/spec/assets, and returns `structured_preview`. A type that has no declared or registered projector follows the legacy fixed type handling, then a JSON preview rather than an error. Legacy HTML shape recognition is normalized to the `document.html` path before UI rendering, so the same document security controls protect it.

The alternative of sending raw payloads and an ArtifactType key to the frontend would duplicate validation, leak storage/payload-shape coupling into every client, and contradict the existing server-preview authority. A registry keyed only by payload fields was rejected because it is neither allowlisted nor auditable.

### 3. `TabularData` and `ChartSpec` are portable typed Artifact payloads

The protobuf contract adds `TabularData`, `TabularColumn`, `TabularRow`, a closed semantic-type enum, and `ChartSpec`. `TabularData` carries ordered columns (`key`, display label, semantic type), ordered scalar row values aligned to those columns, `truncated`/pagination metadata, a source fingerprint, and an RFC 3339 `as_of` timestamp. Values cannot be nested objects or executable expressions; validation rejects duplicate/blank keys, row-width/type mismatches, invalid metadata combinations, and unbounded preview payloads.

`ChartSpec` carries one required, pinned `tabular_data` `ArtifactRef` and a compact Flint semantic chart object. Its validator accepts only the documented Flint chart/encoding/layout fields, requires fields to exist in the referenced tabular schema, and rejects `data.url`, scripts, functions, renderer options, and arbitrary backend configuration. Creation/version validation resolves the ref within the caller tenant, requires `harpia.artifacts.v1.TabularData`, and verifies artifact ID, immutable version ID, type key, and content hash before persisting the chart. The chart projector loads only that pinned tabular version under the same tenant, then emits an inline-data `chart.flint.v1` spec; it does not follow a current-version alias.

This separates a reusable governed data snapshot from the presentation intent. Embedding rows in `ChartSpec` would duplicate business data and lose an independently inspectable/pinned input; storing a raw Vega-Lite/ECharts object would expose renderer-specific and potentially unsafe configuration rather than the portable semantic contract.

### 4. The client registry is a statically linked component map

`frontend/src/lib/artifacts/preview.ts` becomes normalization and lookup code for a finite `renderer_key` registry. It maps `table.v1`, `chart.flint.v1`, `carousel.v1`, `document.html`, `markdown`, and `image` to local Svelte renderer components; a missing key, unknown version, malformed JSON, or failed renderer validation displays the raw structured spec in the JSON fallback. The renderer key is data, not a dynamic import target. Existing fixed response variants normalize into their corresponding registered renderer when possible, preserving the existing LinkedIn-specific behavior until the carousel migration completes.

`chart.flint.v1` statically imports the lockfile-pinned `flint-chart` package (current release `0.1.1`) and `echarts`, compiles only the already validated inline semantic input to an ECharts option, and renders it locally. It never enables Flint's URL-backed data option or injects returned markup. The chart has a concise localized accessible name and exposes its tabular data through the native table renderer as the non-visual equivalent. `table.v1` uses semantic `<table>`, `<caption>`, `<thead>`, and scoped header cells, preserves declared column order, and visibly states when data is truncated or paginated.

Choosing ECharts gives a direct browser renderer for Flint's compiled output. Supporting multiple backend renderers or accepting arbitrary backend options is deferred: it expands the supply-chain and validation surface without improving this substrate.

### 5. HTML documents are sanitized and CSP-confined before `srcdoc`

The document projector/legacy normalizer sanitizes untrusted HTML with an explicit allowlist: it removes scripts, event attributes, forms, base/meta refresh, embedded objects, and unsafe URL schemes. Asset URLs are limited to server-authorized, tenant-scoped preview assets. The document component places the sanitized source in an iframe with no `allow-scripts`, a restrictive `sandbox`, and a `srcdoc` CSP that defaults to `default-src 'none'`; `connect-src`, `form-action`, `frame-src`, and `base-uri` remain `'none'`, while only the minimal sanitized inline style/image policy is allowed. The CSP and sanitizer are both required because a sandbox alone is not a data-exfiltration boundary.

Sanitizing only in the browser was rejected because a compromised or alternate client could receive active markup. Relying only on CSP was rejected because safe display semantics and malicious URL/event removal still require structural sanitization.

### 6. Odoo remains a forward consumer of the substrate, not a special case

A later `query-odoo` PlanTemplate can invoke Odoo only through a scoped Odoo `ExecutorInstallation`, snapshot an answer and selected result set into tenant-owned `TabularData` Artifacts, and optionally emit a `ChartSpec` that pins that snapshot. Credentials, endpoints, and live downstream records never appear in either payload, preview spec, or ArtifactType. This is the Constitution §15 model: Aiuna governs an operation and previews selected Artifact snapshots while Odoo remains the system of record.

Making this substrate Odoo-specific, or fetching live Odoo data in a renderer, was rejected because it would breach generic-platform boundaries, introduce credential exposure, and create a second source of truth.

## Risks / Trade-offs

- **A new generic envelope could become an unchecked JSON escape hatch** → validate renderer key/version and each renderer-specific JSON schema on the server and defensively re-validate/JSON-fallback on the client; impose explicit preview size, row, and asset limits.
- **A chart could display data different from its reviewed input** → require and verify the exact version/hash of the `TabularData` `ArtifactRef`; project only that stored version and test mutation of the current tabular Artifact after chart creation.
- **HTML can exfiltrate data despite an iframe sandbox** → sanitize at the server boundary and use a deny-by-default `srcdoc` CSP/network policy with tests for scripts, handlers, forms, external URLs, and `data:`/`javascript:` schemes.
- **Flint/ECharts expands bundle and supply-chain surface** → use lockfile-pinned static dependencies, a narrow adapter, local data only, and component tests that never perform network fetches; assess the bundle before accepting the dependency update.
- **Legacy and structured variants diverge during the cut** → normalize migrated variants through one frontend registry and add parity tests; retain unmigrated variants only until a named follow-up migrates them.
- **Global ArtifactType metadata is mistaken for tenant configuration** → keep renderer key/version in the global catalog row; tenant ownership/authorization remains on Artifacts, ArtifactVersions, referenced assets, and payload access.

## Migration Plan

1. Add the proto messages/fields, artifact type columns, and generated code in one deployment. Extend all default type seeds with declared renderer metadata and seed `TabularData`/`ChartSpec`; pre-v1 does not require a compatibility shim or data backfill.
2. Land the control-plane registry, validators, projector implementations, reference verification, and JSON fallback before any plan produces a structured type. Test the requested-version and cross-tenant paths with the in-memory and database-backed artifact suites.
3. Add the frontend registry/components, pinned `flint-chart@0.1.1` and `echarts` dependencies, localized strings, and renderer/security tests. Deploy it with the generated TypeScript contract before enabling structured type declarations.
4. Migrate carousel, markdown, and image previews only where their existing payload can be projected without behavior loss; leave other fixed variants operational and tracked for later incremental work.
5. Roll back only before a structured ArtifactType is used by redeploying the prior services and reverting the schema/dependency release. Once structured Artifacts exist, fix forward: immutable Artifacts and versions are retained for inspection, and no lossy payload rewrite is permitted.

## Open Questions

None. The renderer key set, Flint backend, typed payload ownership, and security boundary are settled by this change; future PlanTemplates choose only from this substrate.
