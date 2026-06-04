# ADR-003: Data Architecture

**Status:** Accepted
**Date:** 2026-06-03
**Deciders:** thbertoldi

## Context

Harpia is a multi-tenant platform. Each tenant has its own users, tasks, agents, and data. We need:

1. **Strong tenant isolation** — no cross-tenant data leaks
2. **Agent capability matching** — semantic similarity search for task → agent dispatch
3. **Fast responses** — sub-millisecond reads for hot paths (active tasks, agent status)
4. **File storage** — agent artifacts, task attachments
5. **All open-source, all self-hosted**

## Decision

### Primary Database: PostgreSQL 16

- **Row-Level Security (RLS)** for tenant isolation. Each tenant's queries are filtered at the database level via `current_tenant_id()` — no application-level filtering needed.
- **pgvector** for embedding storage and similarity search (agent capability matching).
- **Recursive CTEs** for tree-shaped queries (task → subtask hierarchies). No graph database needed.

### Cache: Valkey (BSD-3)

Redis changed to non-open-source licenses (RSALv2/SSPL). **Valkey** is the community fork, BSD-3 licensed, fully open-source. Same API, same speed.

| Use case | Pattern |
|---|---|
| Agent capability cache | `task_domain → best_agent` mapping, warmed on agent registration |
| Active workflow state | "What is agent X doing right now?" — answered from cache, not a Temporal query |
| Rate limiting | Per-tenant, per-user token buckets |
| Session state | If needed by the control plane |

### Object Storage: Garage (AGPL-3.0)

S3-compatible, self-hosted, open-source. Stores agent artifacts, task attachments, and file uploads.

### No Graph Database

The data model is tree-shaped: `tenant → workspace → task → subtask → agent_instance`. This is naturally represented in a relational database with foreign keys and recursive CTEs. Adding Neo4j or similar would add operational complexity without benefit for this data shape.

## Rationale

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Neo4j / graph DB | Data is tree-shaped, not graph-shaped. Recursive CTEs handle traversal. Adds a new database to manage. |
| MongoDB | Lacks RLS, transactions, and the query power we need for multi-tenant isolation. |
| MinIO | AGPL-3.0 but with a commercial license model we don't accept. Garage is fully open-source. |
| Redis | License changed to RSALv2/SSPL. Not open-source under our philosophy. |
| NATS JetStream (as KV store) | No longer in stack. Valkey is better suited for caching. |

## Consequences

- **Easier:** One database to operate. RLS guarantees isolation at the lowest level. Valkey adds speed without complexity.
- **Harder:** RLS policies must be maintained as the schema evolves. Valkey introduces cache invalidation challenges — solved by warming on registration and TTL-based expiry.
- **Next:** Implement repository pattern in the control plane with RLS-aware queries. Set up Valkey client. Configure Garage for dev via compose.yaml.
