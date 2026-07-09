"""Temporal activities and workflow wrapping LangGraph execution."""

import asyncio
import contextlib
import json

from connectrpc.errors import ConnectError
from google.protobuf.json_format import MessageToDict, ParseDict
from harpia.artifacts.v1.artifacts_pb2 import (
    CarouselDraft,
    LinkedInPostDraft,
    NewsList,
    TextDraft,
)
from temporalio import activity
from temporalio.exceptions import ApplicationError

from harpia_agents.agents.newsletter_writer import ElicitationRequest
from harpia_agents.agents.registry import AgentRunResult, run_registered_agent
from harpia_agents.artifacts.client import ArtifactPayloadClient
from harpia_agents.identity import require_temporal_tenant
from harpia_agents.llm import LLMRegistry

_LLM_REGISTRY: LLMRegistry = LLMRegistry.default()

_NON_RETRYABLE_CONNECT_ERROR_MARKERS = (
    "LLM_KEY_DECRYPTION_FAILED",
    "LLM_PROVIDER_NOT_CONFIGURED",
    "LLM_PROVIDER_BLOCKED",
    "HARPIA_INTERNAL_AUTH_TOKEN",
    "static credentials are empty",
)

# Periodic heartbeat cadence for the activity body. The Go workflow sets
# HeartbeatTimeout=30s; ticking at half that interval leaves slack for GC
# pauses or scheduling jitter while still well within the deadline.
_HEARTBEAT_INTERVAL_SECONDS = 10.0


@contextlib.asynccontextmanager
async def _heartbeat_loop(details: dict[str, str], *, every: float = _HEARTBEAT_INTERVAL_SECONDS):
    """Heartbeat to Temporal for the lifetime of the wrapped block.

    Emits one heartbeat immediately (so a short HeartbeatTimeout is satisfied
    before the first tick) and then periodically from a background task until
    the wrapped code returns or raises. ``activity.heartbeat`` is a no-op when
    the activity has already been cancelled, and any ``CancelledError`` raised
    by the periodic task during teardown is swallowed so the original error
    from the wrapped block is the one that propagates.

    Becomes a pass-through when invoked outside an activity context (e.g. in
    unit tests) since there is no Temporal worker to receive the heartbeat.
    """
    if not activity.in_activity():
        yield
        return

    activity.heartbeat(details)

    async def _tick() -> None:
        while True:
            await asyncio.sleep(every)
            activity.heartbeat(details)

    task = asyncio.create_task(_tick(), name="harpia-agent-heartbeat")
    try:
        yield
    finally:
        task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await task


def configure_llm_registry(registry: LLMRegistry) -> None:
    global _LLM_REGISTRY
    _LLM_REGISTRY = registry


def _is_non_retryable_activity_error(exc: Exception) -> bool:
    if isinstance(exc, ValueError):
        return True
    if isinstance(exc, ConnectError):
        message = str(exc)
        return any(marker in message for marker in _NON_RETRYABLE_CONNECT_ERROR_MARKERS)
    return False


def _as_non_retryable_application_error(exc: Exception) -> ApplicationError:
    return ApplicationError(
        str(exc),
        type=exc.__class__.__name__,
        non_retryable=True,
    )


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
        for key in ("tone", "topic", "language", "audience", "topics_to_avoid"):
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
    try:
        return await _run_agent_activity(input_payload)
    except Exception as exc:
        if _is_non_retryable_activity_error(exc):
            raise _as_non_retryable_application_error(exc) from exc
        raise


async def _run_agent_with_heartbeat(
    manifest_id: str,
    *,
    tenant_id: str,
    agent_input: NewsList | TextDraft,
    elicitation_responses: dict[str, str] | None,
) -> AgentRunResult:
    """Run the registered agent inside a periodic Temporal heartbeat loop.

    The LLM call (DeepSeek, etc.) is a single blocking await with no internal
    checkpoints, so a background task heartbeats every
    :data:`_HEARTBEAT_INTERVAL_SECONDS` seconds while it is in flight. The
    first heartbeat is emitted synchronously before the await so a tight
    HeartbeatTimeout is satisfied even if the periodic task has not ticked yet.
    """
    async with _heartbeat_loop({"phase": "llm", "manifest_id": manifest_id}):
        return await run_registered_agent(
            manifest_id,
            tenant_id=tenant_id,
            input_payload=agent_input,
            llm_registry=_LLM_REGISTRY,
            elicitation_responses=elicitation_responses,
        )


async def _run_agent_activity(input_payload: dict) -> dict:
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
    elif manifest_id == "linkedin-carousel-senior":
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

    result = await _run_agent_with_heartbeat(
        manifest_id,
        tenant_id=tenant_id,
        agent_input=agent_input,
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
                    "properties": {field: {"type": "string"} for field in result.required_fields},
                }
            ),
        }

    if isinstance(result, TextDraft):
        output_type = "harpia.artifacts.v1.TextDraft"
    elif isinstance(result, LinkedInPostDraft):
        output_type = "harpia.artifacts.v1.LinkedInPostDraft"
    elif isinstance(result, CarouselDraft):
        output_type = "harpia.artifacts.v1.CarouselDraft"
    else:
        raise ValueError("unsupported agent result type")

    payload = MessageToDict(result, preserving_proto_field_name=True)
    artifact_id = await ArtifactPayloadClient().create_payload(
        tenant_id=tenant_id,
        artifact_type_key=str(input_payload.get("output_artifact_type_key") or output_type),
        payload=payload,
        step_execution_id=str(input_payload.get("step_execution_id", "")),
        plan_execution_id=str(input_payload.get("plan_execution_id", "")),
    )
    return {
        "status": "completed",
        "output_artifact_id": artifact_id,
    }
