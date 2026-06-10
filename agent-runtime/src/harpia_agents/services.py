"""ConnectRPC service implementations for the agent runtime."""

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import TYPE_CHECKING

from harpia.agents.v1.agent_type_pb2 import AgentType
from harpia.agents.v1.agents_connect import AgentService
from harpia.agents.v1.agents_pb2 import (
    AgentInstanceStatus,
    ContinueExecutionRequest,
    ContinueExecutionResponse,
    ExecuteTaskRequest,
    ExecuteTaskResponse,
    ListAgentTypesRequest,
    ListAgentTypesResponse,
    MatchAgentRequest,
    MatchAgentResponse,
    RegisterAgentTypeRequest,
    RegisterAgentTypeResponse,
)
from langchain_openai import OpenAIEmbeddings

from harpia_agents.graph import TaskState, build_graph
from harpia_agents.identity import require_selected_tenant, require_tenant

if TYPE_CHECKING:
    from connectrpc.request import RequestContext


class AgentServiceImpl(AgentService):
    """ConnectRPC AgentService implementation with LangGraph integration."""

    async def register_agent_type(
        self,
        request: RegisterAgentTypeRequest,
        ctx: RequestContext,
    ) -> RegisterAgentTypeResponse:
        require_selected_tenant(ctx)
        agent_type = AgentType(
            display_name=request.name,
            description=request.description,
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
