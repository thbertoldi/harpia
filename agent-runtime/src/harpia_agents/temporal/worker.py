"""Temporal activities and workflow wrapping LangGraph execution."""

import json
import uuid
from datetime import timedelta

from google.protobuf.json_format import MessageToDict
from harpia.artifacts.v1.artifacts_pb2 import TextDraft
from temporalio import activity, workflow

from harpia_agents.agents.newsletter_writer import ElicitationRequest
from harpia_agents.agents.registry import run_registered_agent
from harpia_agents.graph import TaskState, build_graph
from harpia_agents.identity import require_temporal_tenant


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


def _extract_news_list_payload(input_payload: dict) -> dict[str, object]:
    input_artifacts = input_payload.get("input_artifacts", [])
    if not isinstance(input_artifacts, list):
        raise ValueError("input_artifacts must be a list")

    for artifact in input_artifacts:
        if not isinstance(artifact, dict):
            continue
        artifact_type = str(artifact.get("artifact_type_key", "")).strip()
        literal_json = artifact.get("literal_json")
        if artifact_type != "harpia.artifacts.v1.NewsList" or not isinstance(literal_json, str):
            continue
        payload = json.loads(literal_json)
        if not isinstance(payload, dict):
            raise ValueError("NewsList literal_json must decode to object")
        return payload

    raise ValueError("missing NewsList input artifact with literal_json payload")


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
    require_temporal_tenant(input_payload)
    installation = input_payload.get("executor_installation_snapshot")
    if not isinstance(installation, dict):
        raise ValueError("executor_installation_snapshot is required")
    manifest_id = str(installation.get("manifest_id", "")).strip()
    if not manifest_id:
        raise ValueError("executor_installation_snapshot.manifest_id is required")

    result = await run_registered_agent(
        manifest_id,
        input_news_list=_extract_news_list_payload(input_payload),
        elicitation_responses=_extract_elicitation_responses(input_payload),
    )
    if isinstance(result, ElicitationRequest):
        return {
            "status": "elicitation_requested",
            "elicitation_thread_id": result.thread_id,
        }

    if not isinstance(result, TextDraft):
        raise ValueError("unsupported agent result type")

    payload = MessageToDict(result, preserving_proto_field_name=True)
    return {
        "status": "completed",
        "output_artifact_id": f"inline:text-draft:{uuid.uuid4()}",
        "output_payload_json": payload,
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
