"""Contract verification for the elicitation executor result shape (E5.1, #130).

The control-plane RunAgentActivity maps an awaiting-feedback ExecuteTaskResponse
to ExecutorResultStatusElicitationRequested. These tests pin the agent-runtime
side of that contract: when an agent run yields an ElicitationRequest, the
streamed response carries the awaiting-feedback status plus the structured prompt
(question) and schema (required fields).
"""

import pytest
from harpia.agents.v1.agents_pb2 import AgentInstanceStatus, ExecuteTaskRequest

from harpia_agents import services
from harpia_agents.agents.newsletter_writer import ElicitationRequest
from harpia_agents.identity import RequestContext


class FakeRequestContext:
    """Minimal ConnectRPC context exposing a resolved request context."""

    def __init__(self, tenant_id: str = "dev") -> None:
        self.scope = {
            "harpia.request_context": RequestContext(
                tenant_id=tenant_id,
                user_id="dev-user",
                roles=("admin",),
            )
        }


@pytest.mark.asyncio
async def test_execute_task_surfaces_structured_elicitation(monkeypatch) -> None:  # noqa: ANN001
    elicitation = ElicitationRequest(
        thread_id="elicitation-abc123",
        question="Before writing the newsletter, what tone should I use?",
        required_fields=("tone", "topics_to_avoid"),
    )
    monkeypatch.setattr(services, "has_runner", lambda _agent_id: True)

    async def fake_run(_agent_id, *, tenant_id=None, input_news_list, llm_registry):  # noqa: ANN001, ANN202
        assert tenant_id == "dev"
        del input_news_list, llm_registry
        return elicitation

    monkeypatch.setattr(services, "run_registered_agent", fake_run)

    impl = services.AgentServiceImpl()
    request = ExecuteTaskRequest(
        tenant_id="dev",
        agent_type_id="newsletter-writer-senior",
        subtask_id="sub-1",
        task_description="{}",
    )

    responses = [response async for response in impl.execute_task(request, FakeRequestContext())]

    assert len(responses) == 1
    response = responses[0]
    assert response.status == AgentInstanceStatus.AGENT_INSTANCE_STATUS_AWAITING_FEEDBACK
    # Structured prompt is preserved.
    assert response.feedback_request.question == elicitation.question
    # Schema fields are carried as the response options.
    assert list(response.feedback_request.options) == ["tone", "topics_to_avoid"]


def test_elicitation_request_is_structured() -> None:
    elicitation = ElicitationRequest(
        thread_id="elicitation-xyz",
        question="What tone should I use?",
        required_fields=("tone",),
    )
    assert elicitation.thread_id.startswith("elicitation-")
    assert elicitation.question
    assert elicitation.required_fields == ("tone",)
