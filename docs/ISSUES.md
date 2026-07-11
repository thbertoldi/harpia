# Issue Backlog

Run these after pushing to GitHub and setting up a remote.

```bash
# Create the repo on GitHub first:
gh repo create harpia-org/harpia --public --source=. --push

# Then create issues:
```

---

## Infrastructure

```bash
gh issue create --title "infra: generate ConnectRPC code from proto schemas" --body "Run \`buf generate\` in \`proto/\` to produce Go, Python, and TypeScript clients. Wire into \`mise.toml\` as \`mise run buf-generate\`." --label "infra,feature"

gh issue create --title "infra: add Temporalite to dev compose" --body "Replace full Temporal deployment with Temporalite single binary for local development. Update \`deploy/dev/compose.yaml\` and \`Tiltfile\`." --label "infra,chore"

gh issue create --title "infra: set up GitHub Actions CI pipeline" --body "Create CI workflow: lint, test, buf breaking, build containers on PR. Push to ghcr.io on merge to trunk." --label "infra,feature"

gh issue create --title "infra: add Zitadel to dev compose" --body "Configure Zitadel as OIDC provider in the development environment. Update compose.yaml and Tiltfile." --label "infra,feature"

gh issue create --title "infra: add Valkey cache client to control-plane" --body "Wire up Valkey Go client. Implement agent capability cache, rate limiting, and active workflow state cache." --label "infra,feature"
```

## Control Plane (Go)

```bash
gh issue create --title "feat(api): implement ConnectRPC server scaffolding" --body "Replace Gin handlers with ConnectRPC service implementations. Wire up the generated code from \`proto/\`. Implement TaskService, AgentService, and IdentityService." --label "feature" --assignee @me

gh issue create --title "feat(api): implement database repository layer" --body "Create repository pattern with pgx for PostgreSQL. Implement RLS-aware queries for tasks, agents, tenants. Use the schema from \`database/migrations/\`." --label "feature" --assignee @me

gh issue create --title "feat(api): implement Temporal workflow client" --body "Wire up Temporal Go SDK. Create workflow starter for task decomposition. Implement human-in-the-loop signal handling." --label "feature" --assignee @me

gh issue create --title "feat(api): implement Valkey cache integration" --body "Add agent capability cache, rate limiter, and workflow state cache using Valkey." --label "feature" --assignee @me
```

## Agent Runtime (Python)

```bash
gh issue create --title "feat(agent): implement ConnectRPC server" --body "Replace stub server with real ConnectRPC implementation. Wire up generated Python code from \`proto/\`." --label "feature" --assignee @me

gh issue create --title "feat(agent): implement supervisor graph logic" --body "Replace stub nodes in \`graph.py\` with real planner, worker, and human_feedback implementations. Integrate with LLM provider." --label "feature" --assignee @me

gh issue create --title "feat(agent): implement agent capability matching" --body "Embed agent capability descriptions. Store embeddings in pgvector. Implement semantic similarity matching via cosine distance." --label "feature" --assignee @me

gh issue create --title "feat(agent): integrate Temporal worker" --body "Create Temporal worker that executes LangGraph workflows as activities. Handle signals for human-in-the-loop." --label "feature" --assignee @me
```

## Frontend (Svelte)

```bash
gh issue create --title "feat(ui): implement ConnectRPC client and real-time updates" --body "Generate TypeScript client from proto. Implement server-streaming for real-time task progress in the UI." --label "feature" --assignee @me

gh issue create --title "feat(ui): implement task dashboard" --body "Create task list view, task detail with subtask tree, agent status display. Use Connect server-streaming for live updates." --label "feature" --assignee @me

gh issue create --title "feat(ui): implement auth flow with Zitadel" --body "Add login/logout flow. Tenant selector. Session management via Zitadel OIDC." --label "feature" --assignee @me

gh issue create --title "feat(ui): implement human-in-the-loop feedback UI" --body "Approval/rejection interface for tasks requiring human input. Integrate with Connect streaming feedback mechanism." --label "feature" --assignee @me
```

## Slack Integration

```bash
gh issue create --title "feat(slack): implement Slack bot service" --body "Create ConnectRPC service that bridges Temporal human-in-the-loop signals to Slack. Listen for Slack interactions and translate to Temporal signals." --label "feature" --assignee @me
```

## Observability

```bash
gh issue create --title "feat(obs): instrument services with OpenTelemetry" --body "Add OTEL tracing to control-plane (Go), agent-runtime (Python), and frontend (TypeScript). Export to Tempo." --label "feature" --assignee @me

gh issue create --title "feat(obs): add Prometheus metrics to all services" --body "Instrument control-plane and agent-runtime with Prometheus metrics. Create Grafana dashboards." --label "feature" --assignee @me
```
