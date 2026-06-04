from harpia_agents.graph import TaskState, build_graph


def test_task_state_defaults():
    state = TaskState()
    assert state.task_id == ""
    assert state.status == "pending"


def test_graph_builds():
    graph = build_graph()
    assert graph is not None
