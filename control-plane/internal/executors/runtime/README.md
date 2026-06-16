# Executor runtime

The executor runtime is the control-plane boundary for **deterministic integration SKUs**
(rss-news-feed today; linkedin-publish and others follow the same pattern).

## Responsibilities

- **Integration registry** — route `IntegrationExecutionRequest` by `Installation.ExecutorSKUKey`.
- **Config validation** — reject invalid installation `config_json` at create time and during plan readiness checks.
- **Artifact port** — `ExecutorArtifactStore` loads inputs and creates outputs using stable artifact type **keys** (`harpia.artifacts.v1.NewsList`), not database UUIDs.
- **Failure classification** — handlers return `RetryableError` for transient failures; validation problems are terminal failed results or `TerminalValidationError`.

## Adding a new integration SKU

1. Add the SKU key to `executors/catalog.go` and seed compatibility metadata.
2. Implement `runtime.IntegrationHandler` under `executors/integrations/<sku>/`.
3. Implement `runtime.ConfigValidator` for installation `config_json`.
4. Register both in `executors/bootstrap.NewRuntime` (worker) and `executors/bootstrap.DefaultConfigValidators` (API/plans).
5. Add contract tests via `executors/contracttest` (see `integrations/rss/contract_test.go`).

Handlers must not import workflow or Temporal packages. The workflow adapter maps
`RetryableError` to Temporal application errors.
