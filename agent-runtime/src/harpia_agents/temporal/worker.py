"""Temporal activities and workflow wrapping LangGraph execution."""

import json
from datetime import timedelta

from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, NewsList, TextDraft
from temporalio import activity, workflow

from harpia_agents.agents.newsletter_writer import ElicitationRequest
from harpia_agents.agents.registry import run_registered_agent
from harpia_agents.artifacts.client import ArtifactPayloadClient
from harpia_agents.graph import TaskState, build_graph
from harpia_agents.identity import require_temporal_tenant
from harpia_agents.llm import LLMRegistry

_LLM_REGISTRY: LLMRegistry = LLMRegistry.default()


def configure_llm_registry(registry: LLMRegistry) -> None:
    global _LLM_REGISTRY
    _LLM_REGISTRY = registry


@activity.defn
async def decompose_task_activity(input: dict) -> dict:
    """Decompose a high-level task into subtasks using LangGraph."""
    tenant_id = require_temporal_tenant(input)
    graph = build_graph()
    state = TaskState(
        task_id=input["task_id"],
        tenant_id=tenant_id,
        description=input["description"],
    )
    result = await graph.ainvoke(state.model_dump())
    return result


@activity.defn
async def execute_subtask_activity(input: dict) -> dict:
    """Execute a single subtask using LangGraph worker node."""
    tenant_id = require_temporal_tenant(input)
    graph = build_graph()
    state = TaskState(
        task_id=input["task_id"],
        tenant_id=tenant_id,
        description=input["description"],
        subtasks=input.get("subtasks", []),
        current_subtask=input.get("current_subtask", 0),
    )
    result = await graph.ainvoke(state.model_dump())
    return result


def _parse_literal_json(raw: object) -> dict[str, object] | None:
    if not isinstance(raw, str) or not raw.strip():
        return None
    decoded = json.loads(raw)
    if not isinstance(decoded, dict):
        raise ValueError("literal_json must decode to object")
    return decoded


async def _resolve_input_payload(
    input_payload: dict,
    *,
    tenant_id: str,
    artifact_type_key: str,
) -> dict[str, object]:
    input_artifacts = input_payload.get("input_artifacts", [])
    if not isinstance(input_artifacts, list):
        raise ValueError("input_artifacts must be a list")

    client = ArtifactPayloadClient()
    for artifact in input_artifacts:
        if not isinstance(artifact, dict):
            continue
        artifact_type = str(artifact.get("artifact_type_key", "")).strip()
        if artifact_type != artifact_type_key:
            continue

        artifact_id = str(artifact.get("artifact_id", "")).strip()
        if artifact_id:
            return await client.get_payload(tenant_id=tenant_id, artifact_id=artifact_id)

        literal = _parse_literal_json(artifact.get("literal_json"))
        if literal is not None:
            return literal

    raise ValueError(f"missing input artifact {artifact_type_key}")


def _extract_content_preferences(input_payload: dict) -> dict[str, str]:
    responses: dict[str, str] = {}
    input_artifacts = input_payload.get("input_artifacts", [])
    if not isinstance(input_artifacts, list):
        return responses

    for artifact in input_artifacts:
        if not isinstance(artifact, dict):
            continue
        if str(artifact.get("input_name", "")).strip() != "harpia.internal.ContentPreferences":
            continue
        literal = _parse_literal_json(artifact.get("literal_json")) or {}
        for key in ("tone", "topic", "topics_to_avoid"):
            value = literal.get(key)
            if isinstance(value, str) and value.strip():
                responses[key] = value.strip()
    return responses


def _extract_elicitation_responses(input_payload: dict) -> dict[str, str] | None:
    signal = input_payload.get("elicitation_response")
    if not isinstance(signal, dict):
        return None

    raw_response = signal.get("response_text")
    if not isinstance(raw_response, str) or not raw_response.strip():
        return None

    try:
        decoded = json.loads(raw_response)
    except json.JSONDecodeError:
        return {"tone": raw_response.strip()}

    if not isinstance(decoded, dict):
        return {"tone": raw_response.strip()}
    parsed = {str(key): str(value) for key, value in decoded.items()}
    return parsed or None


@activity.defn(name="RunAgentActivity")
async def run_agent_activity(input_payload: dict) -> dict:
    """Run a manifest-backed agent and return executor activity contract fields."""
    tenant_id = require_temporal_tenant(input_payload)
    installation = input_payload.get("executor_installation_snapshot")
    if not isinstance(installation, dict):
        raise ValueError("executor_installation_snapshot is required")
    manifest_id = str(installation.get("manifest_id", "")).strip()
    if not manifest_id:
        raise ValueError("executor_installation_snapshot.manifest_id is required")

    if manifest_id == "newsletter-writer-senior":
        payload = await _resolve_input_payload(
            input_payload,
            tenant_id=tenant_id,
            artifact_type_key="harpia.artifacts.v1.NewsList",
        )
        agent_input = NewsList()
        ParseDict(payload, agent_input)
        elicitation_responses = {
            **_extract_content_preferences(input_payload),
            **(_extract_elicitation_responses(input_payload) or {}),
        }
    elif manifest_id == "linkedin-voice-senior":
        payload = await _resolve_input_payload(
            input_payload,
            tenant_id=tenant_id,
            artifact_type_key="harpia.artifacts.v1.TextDraft",
        )
        agent_input = TextDraft()
        ParseDict(payload, agent_input)
        elicitation_responses = _extract_elicitation_responses(input_payload)
    else:
        raise ValueError(f"unsupported manifest id: {manifest_id}")

    result = await run_registered_agent(
        manifest_id,
        tenant_id=tenant_id,
        input_payload=agent_input,
        llm_registry=_LLM_REGISTRY,
        elicitation_responses=elicitation_responses,
    )
    if isinstance(result, ElicitationRequest):
        return {
            "status": "elicitation_requested",
            "elicitation_thread_id": result.thread_id,
            "elicitation_prompt": result.question,
            "elicitation_schema_json": json.dumps(
                {
                    "type": "object",
                    "required": list(result.required_fields),
                    "properties": {
                        field: {"type": "string"} for field in result.required_fields
                    },
                }
            ),
        }

    if isinstance(result, TextDraft):
        output_type = "harpia.artifacts.v1.TextDraft"
    elif isinstance(result, LinkedInPostDraft):
        output_type = "harpia.artifacts.v1.LinkedInPostDraft"
    else:
        raise ValueError("unsupported agent result type")

    payload = MessageToDict(result, preserving_proto_field_name=True)
    artifact_id = await ArtifactPayloadClient().create_payload(
        tenant_id=tenant_id,
        artifact_type_key=str(input_payload.get("output_artifact_type_key") or output_type),
        payload=payload,
        step_execution_id=str(input_payload.get("step_execution_id", "")),
    )
    return {
        "status": "completed",
        "output_artifact_id": artifact_id,
    }


@workflow.defn
class HarpiaTaskWorkflow:
    """Temporal workflow that orchestrates LangGraph task execution."""

    @workflow.run
    async def run(self, task_input: dict) -> dict:
        decompose_result = await workflow.execute_activity(
            decompose_task_activity,
            task_input,
            start_to_close_timeout=timedelta(minutes=10),
        )

        results = []
        for subtask in decompose_result.get("subtasks", []):
            execute_input: dict = {
                "task_id": task_input["task_id"],
                "tenant_id": task_input["tenant_id"],
                "description": subtask.get("description", subtask.get("title", "")),
                "subtasks": [subtask],
                "current_subtask": 0,
            }
            result = await workflow.execute_activity(
                execute_subtask_activity,
                execute_input,
                start_to_close_timeout=timedelta(minutes=5),
            )
            results.append(result)

        return {"status": "completed", "results": results}
