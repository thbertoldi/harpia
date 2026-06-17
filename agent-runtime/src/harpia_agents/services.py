"""ConnectRPC service implementations for the agent runtime."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator
from typing import TYPE_CHECKING

from google.protobuf.json_format import MessageToDict
from harpia.agents.v1.agents_connect import AgentService
from harpia.agents.v1.agents_pb2 import (
    AgentInstanceStatus,
    AgentType,
    ContinueExecutionRequest,
    ContinueExecutionResponse,
    ExecuteTaskRequest,
    ExecuteTaskResponse,
    FeedbackRequest,
    ListAgentTypesRequest,
    ListAgentTypesResponse,
    MatchAgentRequest,
    MatchAgentResponse,
    RegisterAgentTypeRequest,
    RegisterAgentTypeResponse,
)
from langchain_openai import OpenAIEmbeddings

from harpia_agents.agents.newsletter_writer import ElicitationRequest
from harpia_agents.agents.registry import has_runner, run_registered_agent
from harpia_agents.graph import TaskState, build_graph
from harpia_agents.identity import require_selected_tenant, require_tenant
from harpia_agents.llm import LLMRegistry

if TYPE_CHECKING:
    from connectrpc.request import RequestContext


class AgentServiceImpl(AgentService):
    """ConnectRPC AgentService implementation with LangGraph integration."""

    def __init__(self, *, llm_registry: LLMRegistry | None = None) -> None:
        self._llm_registry = llm_registry or LLMRegistry.default()

    async def register_agent_type(
        self,
        request: RegisterAgentTypeRequest,
        ctx: RequestContext,
    ) -> RegisterAgentTypeResponse:
        require_selected_tenant(ctx)
        agent_type = AgentType(
            name=request.name,
            display_name=request.name,
            description=request.description,
            capabilities_text=request.capabilities_text,
            capabilities=[request.capabilities_text],
        )
        return RegisterAgentTypeResponse(agent_type=agent_type)

    def list_agent_types(
        self,
        request: ListAgentTypesRequest,
        ctx: RequestContext,
    ) -> AsyncIterator[ListAgentTypesResponse]:
        require_selected_tenant(ctx)

        async def _stream() -> AsyncIterator[ListAgentTypesResponse]:
            # TODO: query from registry
            yield ListAgentTypesResponse(agent_types=[])

        return _stream()

    async def match_agent(
        self,
        request: MatchAgentRequest,
        ctx: RequestContext,
    ) -> MatchAgentResponse:
        require_tenant(ctx, request.tenant_id)
        # TODO(#35): embeddings use a separate abstraction from chat completion providers.
        embedder = OpenAIEmbeddings()
        await embedder.aembed_query(request.task_description)
        return MatchAgentResponse(matches=[])

    def execute_task(
        self,
        request: ExecuteTaskRequest,
        ctx: RequestContext,
    ) -> AsyncIterator[ExecuteTaskResponse]:
        tenant_id = require_tenant(ctx, request.tenant_id)

        async def _stream() -> AsyncIterator[ExecuteTaskResponse]:
            if has_runner(request.agent_type_id):
                try:
                    payload = (
                        json.loads(request.task_description) if request.task_description else {}
                    )
                    if not isinstance(payload, dict):
                        raise ValueError("task_description must be a JSON object")
                    result = await run_registered_agent(
                        request.agent_type_id,
                        input_news_list=payload,
                        llm_registry=self._llm_registry,
                    )
                    if isinstance(result, ElicitationRequest):
                        yield ExecuteTaskResponse(
                            status=AgentInstanceStatus.AGENT_INSTANCE_STATUS_AWAITING_FEEDBACK,
                            message="elicitation requested",
                            feedback_request=FeedbackRequest(
                                subtask_id=request.subtask_id,
                                question=result.question,
                                options=list(result.required_fields),
                            ),
                        )
                        return

                    yield ExecuteTaskResponse(
                        status=AgentInstanceStatus.AGENT_INSTANCE_STATUS_COMPLETED,
                        output=json.dumps(
                            MessageToDict(result, preserving_proto_field_name=True),
                            sort_keys=True,
                        ),
                    )
                    return
                except Exception as exc:
                    yield ExecuteTaskResponse(
                        status=AgentInstanceStatus.AGENT_INSTANCE_STATUS_FAILED,
                        error=str(exc),
                    )
                    return

            graph = build_graph()
            state = TaskState(
                task_id=request.task_id,
                tenant_id=tenant_id,
                description=request.task_description,
            )
            async for event in graph.astream(state.model_dump()):
                yield ExecuteTaskResponse(
                    status=AgentInstanceStatus.AGENT_INSTANCE_STATUS_EXECUTING,
                    message=str(event),
                )
            yield ExecuteTaskResponse(
                status=AgentInstanceStatus.AGENT_INSTANCE_STATUS_COMPLETED,
                output="task completed",
            )

        return _stream()

    async def continue_execution(
        self,
        request: ContinueExecutionRequest,
        ctx: RequestContext,
    ) -> ContinueExecutionResponse:
        require_tenant(ctx, request.tenant_id)
        return ContinueExecutionResponse(
            agent_instance_id=request.agent_instance_id,
            status=AgentInstanceStatus.AGENT_INSTANCE_STATUS_COMPLETED,
            message="execution continued",
        )
