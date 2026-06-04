"""Central LangGraph state graph definition.

The graph implements a supervisor pattern:
    Planner (supervisor) -> Worker Agents (specialized subgraphs)

Flow:
    Task input -> Planner decomposes -> Routes to workers -> Aggregates -> Output
"""

from typing import Annotated, Any, Literal

from langgraph.graph import StateGraph, END
from pydantic import BaseModel


class TaskState(BaseModel):
    """Shared state across all nodes in the agent graph."""
    task_id: str = ""
    tenant_id: str = ""
    description: str = ""          # Human-provided description
    subtasks: list[dict[str, Any]] = []  # Planner output
    current_subtask: int = 0
    results: dict[str, Any] = {}
    status: Literal["pending", "planning", "running", "waiting_human", "completed", "failed"] = "pending"
    human_feedback: str | None = None
    error: str | None = None


def planner_node(state: TaskState) -> dict[str, Any]:
    """Decompose a high-level task into subtasks."""
    # TODO: Implement LLM-based task decomposition
    return {"status": "planning", "subtasks": []}


def router_node(state: TaskState) -> str:
    """Route to the appropriate worker node based on subtask type."""
    if state.status == "completed":
        return END
    if state.status == "waiting_human":
        return "human_feedback"
    return "worker"


def worker_node(state: TaskState) -> dict[str, Any]:
    """Execute the current subtask with a specialized agent."""
    # TODO: Execute via Temporal workflow / agent invocation
    return {"status": "running"}


def human_feedback_node(state: TaskState) -> dict[str, Any]:
    """Handle human-in-the-loop interaction."""
    return {"status": "pending"}


def build_graph() -> StateGraph:
    """Build and return the compiled LangGraph state graph."""
    graph = StateGraph(TaskState)

    graph.add_node("planner", planner_node)
    graph.add_node("worker", worker_node)
    graph.add_node("human_feedback", human_feedback_node)

    graph.set_entry_point("planner")
    graph.add_conditional_edges("planner", router_node, {
        "worker": "worker",
        "human_feedback": "human_feedback",
        END: END,
    })
    graph.add_conditional_edges("worker", router_node, {
        "worker": "worker",
        "human_feedback": "human_feedback",
        END: END,
    })
    graph.add_edge("human_feedback", "planner")

    return graph.compile()


__all__ = ["build_graph", "TaskState"]
