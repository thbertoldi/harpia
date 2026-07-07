# ADR-009: LangGraph State Merge Semantics

> **⚠ Historical record — not the current source of truth.** The canonical description of the
> platform is the **[Platform Constitution](../architecture/harpia-platform.md)**, which
> supersedes ADR-001…017 as the reading order. See the
> **[ADR supersession map](README.md)** for how this ADR stands today. Where this ADR and the
> constitution disagree, the constitution wins.

**Status:** Accepted
**Date:** 2026-06-10
**Deciders:** thbertoldi

## Context

The agent runtime supervisor graph uses LangGraph with a Pydantic
`TaskState` model in `agent-runtime/src/harpia_agents/graph.py`. Issue #64
made the state domain-typed: graph nodes now carry `Subtask` instances and
`SubtaskResult` instances instead of converting them to `dict[str, Any]`.

That leaves one explicit design decision: LangGraph updates returned from a
node replace the returned field. Replacement is correct for fields that move the
workflow forward, such as `status` and `current_subtask`, but it can lose data
for accumulating fields such as `results`, future reflection history, cost
ledgers, and trust-scoring observations.

The project quality bar favors domain types and runtime validation at the
boundary of agent orchestration. It also favors keeping concrete framework
mechanics from contaminating the domain model.

## Decision

Keep the supervisor state as a Pydantic `BaseModel` and standardize on an
explicit-merge convention for accumulating fields.

Node authors must treat every returned state field as a full replacement. When a
node extends an aggregate, it must copy the prior value and return the merged
replacement:

```python
results = {**state.results, subtask.title: result}
return {"results": results}
```

For future list-like accumulators, nodes must use the same explicit replacement
shape:

```python
return {"reflection_history": [*state.reflection_history, new_record]}
```

No LangGraph reducer functions are used for the current supervisor graph.
Reducer files such as `agent-runtime/src/harpia_agents/state/reducers.py` should
not be introduced until the graph has multiple independent writers updating the
same field in one graph step.

## Rationale

Pydantic state keeps the graph boundary honest. The graph can validate inbound
state, round-trip serialized state back into domain models, and preserve IDE and
type-checker feedback for fields such as `subtask.suggested_agent_type`.

Explicit replacement is also easy to audit in the current graph because the
supervisor is linear: planner writes `subtasks`, worker appends one result at a
time, and router only chooses the next edge. There is no concurrent fan-in where
framework-level reducers would remove real complexity.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| TypedDict state with `Annotated[..., reducer]` fields | LangGraph reducers would enforce append/merge semantics, but the top-level state would lose Pydantic validation and serialization ergonomics. The domain model would become easier to contaminate with framework-specific annotations. |
| Hybrid TypedDict graph state plus Pydantic domain records | This keeps typed records inside the state, but introduces two type systems for a graph that is currently linear. The extra mental overhead is not justified until multiple graph branches write to the same aggregate. |
| Implicit convention without an ADR | This is what the code would otherwise drift toward. Future scorer, reflection, cost, and trust-scoring work would each invent merge behavior independently. |

## Serialization

Temporal and future checkpointing adapters may serialize `TaskState` with
Pydantic APIs such as `model_dump()` or `model_dump_json()`. On restore, adapters
must call `TaskState.model_validate(...)` before handing state back to graph
nodes. Transport adapters may work with dictionaries at process boundaries, but
graph nodes must not convert domain records to dictionaries for intra-graph
handoff.

If the project later adopts a LangGraph checkpointer such as `PostgresSaver`,
the checkpointer stores serialized state snapshots. The adapter layer owns
compatibility with LangGraph and checkpoint storage versions; the domain graph
continues to consume `TaskState`, `Subtask`, and typed accumulator records.

## Migration Path

The current code after issue #64 already follows this ADR:

1. `TaskState` remains a Pydantic `BaseModel`.
2. Domain state inside the graph uses Pydantic records.
3. `worker_node` extends `results` with `{**state.results, title: result}`.
4. Tests round-trip state through Pydantic and assert typed records survive.

Future migrations must follow the same rule:

1. Add new accumulator records as Pydantic domain models.
2. Add accumulator fields to `TaskState` with `Field(default_factory=...)`.
3. In each node, return full merged replacements for accumulator fields.
4. Add tests that prove prior accumulator entries survive the node update.
5. Revisit reducers only when graph topology introduces parallel writers.

## Consequences

What becomes easier:

- Graph nodes retain typed domain access and runtime validation.
- Serialization remains straightforward for Temporal and future checkpointing.
- Reviews can spot accumulator updates by looking for explicit copy-and-merge.
- Downstream issues have a consistent rule: #24 scorer, #44 planner enrichment,
  #46 reflection loop, #34 cost tracking, and #25 trust scoring all add typed
  state and explicit merged replacements.

What becomes harder:

- The merge convention is enforced by code review and tests, not by LangGraph's
  reducer mechanism.
- A node can still accidentally overwrite an accumulator if tests do not cover
  prior entries.
- If the graph evolves to parallel branches that update the same field, this ADR
  must be revisited.

Follow-up guardrails:

- Accumulator-node tests must seed an existing entry and assert it survives.
- Type-check sentinels should reference fields that future graph nodes depend on.
- A future move to reducers requires a new ADR because it changes the graph
  state contract and serialization story.
