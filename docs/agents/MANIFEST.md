# Agent Type Manifest

The agent type manifest is the single source of truth for what an agent is in
Harpia. Platform Engineers author one YAML file per immutable agent version,
for example `agents/email-drafter/v1.yaml`. Runtime and control-plane code use
the protobuf contract in `proto/harpia/agents/v1/agent_type.proto`.

## Schema

| Field | Type | Required | Convention |
|---|---|---:|---|
| `id` | string | yes | Stable kebab-case identifier, for example `email-drafter`. |
| `version` | string | yes | Semver. Treat each row/file as immutable after registration. |
| `display_name` | string | yes | Human-readable name shown to Platform Engineers and Overseers. |
| `description` | string | yes | Short explanation of when to use this agent. |
| `capabilities` | string array | yes | Free-form matching tags for pgvector routing. |
| `model_id` | string | yes | Must resolve through the LLM registry. |
| `system_prompt` | string | yes | Jinja template. Each `{{variable}}` must be declared in `input_schema.properties`. |
| `allowed_tool_ids` | string array | yes | Tool registry IDs this agent may invoke. No other tools are allowed. |
| `input_schema` | object | yes | JSON Schema for supervisor-provided input. |
| `output_schema` | object | yes | JSON Schema for the agent result returned to the supervisor. |
| `cost_estimate` | number | yes | Rough USD-per-invocation estimate for routing/scoring. |
| `metadata` | object | yes | Free-form Platform Engineer notes. Use `{}` when none are needed. |

## Example

```yaml
id: email-drafter
version: 1.0.0
display_name: Email Drafter
description: Drafts concise, context-aware email responses for overseer review.
capabilities:
  - email
  - writing
  - drafting
model_id: openai-gpt-4o-mini
system_prompt: |
  Draft an email to {{recipient_name}} about {{purpose}}.
  Keep the tone {{tone}} and return only the proposed message body.
allowed_tool_ids:
  - tenant-knowledge-search
input_schema:
  type: object
  required:
    - recipient_name
    - purpose
    - tone
  properties:
    recipient_name:
      type: string
    purpose:
      type: string
    tone:
      type: string
output_schema:
  type: object
  required:
    - body
  properties:
    body:
      type: string
cost_estimate: 0.01
metadata:
  owner: platform-engineering
```

## Python Boundary

Use `harpia_agents.agents.AgentType` at the serialization boundary:

```python
from harpia_agents.agents import AgentType, ManifestReferenceRegistry

registry = ManifestReferenceRegistry.from_iterables(
    model_ids=["openai-gpt-4o-mini"],
    tool_ids=["tenant-knowledge-search"],
)

manifest = AgentType.from_yaml("agents/email-drafter/v1.yaml", registry=registry)
proto = manifest.to_proto()
same_manifest = AgentType.from_proto(proto, registry=registry)
```

Validation rejects missing required fields, unknown top-level fields, invalid
`id`/`version` formats, undeclared prompt variables, unknown `model_id` values,
and undefined `allowed_tool_ids` when registry IDs are provided.

## Conventions

- Store manifests as `agents/<agent-id>/v<major>.yaml` or
  `agents/<agent-id>/<version>.yaml`; keep the manifest `version` semver exact.
- Keep `capabilities` short and domain-facing. They are matching hints, not an
  authorization system.
- Keep tool access least-privilege. A tool ID must be listed in
  `allowed_tool_ids` before an agent can invoke it.
- Put operational notes, owner, maturity, and canary status in `metadata`.
  Do not put routing or authorization fields there.
