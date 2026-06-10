import pytest
from pydantic import ValidationError

from harpia_agents.graph import (
    Subtask,
    SubtaskResult,
    TaskPlan,
    TaskState,
    build_graph,
    human_feedback_node,
    planner_node,
    router_node,
    worker_node,
)


def test_task_state_defaults():
    state = TaskState()
    assert state.task_id == ""
    assert state.status == "pending"
    assert state.subtasks == []
    assert state.results == {}


def test_graph_builds():
    graph = build_graph()
    assert graph is not None


def test_planner_node_keeps_typed_subtasks(monkeypatch):
    subtask = Subtask(
        title="Draft brief",
        description="Draft the initial project brief",
        suggested_agent_type="writing",
    )

    class FakeStructuredLLM:
        def invoke(self, messages):
            assert messages[-1] == ("human", "Create a launch plan")
            return TaskPlan(subtasks=[subtask])

    class FakeChatOpenAI:
        def __init__(self, *, model, temperature):
            assert model == "gpt-4o-mini"
            assert temperature == 0

        def with_structured_output(self, schema):
            assert schema is TaskPlan
            return FakeStructuredLLM()

    monkeypatch.setattr("harpia_agents.graph.ChatOpenAI", FakeChatOpenAI)

    update = planner_node(TaskState(description="Create a launch plan"))

    assert update["status"] == "running"
    assert update["current_subtask"] == 0
    assert update["subtasks"] == [subtask]
    assert isinstance(update["subtasks"][0], Subtask)
    assert update["subtasks"][0].title == "Draft brief"


def test_compiled_graph_preserves_typed_state_between_nodes(monkeypatch):
    subtask = Subtask(
        title="Draft brief",
        description="Draft the initial project brief",
        suggested_agent_type="writing",
    )

    class FakeStructuredLLM:
        def invoke(self, messages):
            assert messages[-1] == ("human", "Create a launch plan")
            return TaskPlan(subtasks=[subtask])

    class FakeChatOpenAI:
        def __init__(self, *, model, temperature):
            assert model == "gpt-4o-mini"
            assert temperature == 0

        def with_structured_output(self, schema):
            assert schema is TaskPlan
            return FakeStructuredLLM()

    monkeypatch.setattr("harpia_agents.graph.ChatOpenAI", FakeChatOpenAI)

    graph = build_graph()
    result = graph.invoke(TaskState(description="Create a launch plan").model_dump())
    final_state = TaskState.model_validate(result)

    assert final_state.status == "completed"
    assert isinstance(final_state.subtasks[0], Subtask)
    assert final_state.subtasks[0].title == "Draft brief"
    assert isinstance(final_state.results["Draft brief"], SubtaskResult)
    assert final_state.results["Draft brief"].subtask_title == "Draft brief"


def test_router_node_uses_typed_subtask_attributes():
    state = TaskState(
        subtasks=[
            Subtask(
                title="Approve scope",
                description="Confirm scope with the overseer",
                suggested_agent_type="human",
                needs_human_input=True,
            )
        ]
    )

    assert router_node(state) == "human_feedback"
    assert router_node(state.model_copy(update={"human_feedback": "approved"})) == "worker"


def test_worker_node_stores_typed_subtask_result():
    state = TaskState(
        subtasks=[
            Subtask(
                title="Draft brief",
                description="Draft the initial project brief",
                suggested_agent_type="writing",
            )
        ]
    )

    update = worker_node(state)

    assert update["status"] == "completed"
    assert update["current_subtask"] == 1
    result = update["results"]["Draft brief"]
    assert isinstance(result, SubtaskResult)
    assert result.subtask_title == "Draft brief"
    assert result.description == "Draft the initial project brief"
    assert result.agent_type == "writing"


def test_task_state_round_trips_subtasks_through_pydantic():
    state = TaskState(
        subtasks=[
            Subtask(
                title="Draft brief",
                description="Draft the initial project brief",
                suggested_agent_type="writing",
            )
        ],
        results={
            "Draft brief": SubtaskResult(
                subtask_title="Draft brief",
                description="Draft the initial project brief",
                agent_type="writing",
                output="done",
            )
        },
    )

    round_tripped = TaskState.model_validate(state.model_dump())

    assert isinstance(round_tripped.subtasks[0], Subtask)
    assert round_tripped.subtasks[0].title == "Draft brief"
    assert isinstance(round_tripped.results["Draft brief"], SubtaskResult)
    assert round_tripped.results["Draft brief"].output == "done"


def test_subtask_rejects_unknown_fields():
    with pytest.raises(ValidationError):
        Subtask.model_validate(
            {
                "title": "Draft brief",
                "description": "Draft the initial project brief",
                "suggested_agent_type": "writing",
                "transport_only": "not part of the domain schema",
            }
        )


def test_human_feedback_node_returns_update_without_mutating_state():
    state = TaskState(status="waiting_human")

    update = human_feedback_node(state)

    assert update == {"human_feedback": "accepted", "status": "running"}
    assert state.human_feedback is None
    assert state.status == "waiting_human"
