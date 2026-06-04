"""Central LangGraph state graph definition.

The graph implements a supervisor pattern:
    Planner (supervisor) -> Worker Agents (specialized subgraphs)

Flow:
    Task input -> Planner decomposes -> Routes to workers -> Aggregates -> Output
"""

from typing import Any, Literal

from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph
from pydantic import BaseModel


class Subtask(BaseModel):
    """A single subtask in the task decomposition plan."""

    title: str
    description: str
    suggested_agent_type: str
    needs_human_input: bool = False


class TaskPlan(BaseModel):
    """Structured output from the planner LLM."""

    subtasks: list[Subtask]


class TaskState(BaseModel):
    """Shared state across all nodes in the agent graph."""

    task_id: str = ""
    tenant_id: str = ""
    description: str = ""
    subtasks: list[dict[str, Any]] = []
    current_subtask: int = 0
    results: dict[str, Any] = {}
    status: Literal[
        "pending", "planning", "running", "waiting_human", "completed", "failed"
    ] = "pending"
    human_feedback: str | None = None
    error: str | None = None


def planner_node(state: TaskState) -> dict[str, Any]:
    """Decompose a high-level task into subtasks using an LLM."""
    try:
        llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
        structured_llm = llm.with_structured_output(TaskPlan)
        plan: TaskPlan = structured_llm.invoke(
            [
                (
                    "system",
                    "You are a task planner. Decompose the given task into concrete, "
                    "executable subtasks. Each subtask must have a clear title, "
                    "description, and a suggested agent type (e.g., 'research', "
                    "'code', 'data', 'human'). Set needs_human_input=True for "
                    "subtasks that require human approval or input before proceeding.",
                ),
                ("human", state.description),
            ]
        )
        subtasks = [subtask.model_dump() for subtask in plan.subtasks]
        return {"status": "running", "subtasks": subtasks, "current_subtask": 0}
    except Exception as e:
        return {"status": "failed", "error": f"Planner error: {e}"}


def router_node(state: TaskState) -> str:
    """Route to the appropriate node based on current state and subtask progress."""
    if state.status == "failed":
        return END
    if state.status == "completed":
        return END
    if state.current_subtask >= len(state.subtasks):
        return END
    current = state.subtasks[state.current_subtask]
    if current.get("needs_human_input", False) and state.human_feedback is None:
        return "human_feedback"
    return "worker"


def worker_node(state: TaskState) -> dict[str, Any]:
    """Execute the current subtask with a simulated agent invocation."""
    idx = state.current_subtask
    subtask = state.subtasks[idx]
    title = subtask.get("title", f"subtask_{idx}")

    result = {
        "subtask": title,
        "description": subtask.get("description", ""),
        "output": (
            f"Successfully executed subtask: {title}. "
            f"Agent type used: {subtask.get('suggested_agent_type', 'unknown')}."
        ),
        "agent_type": subtask.get("suggested_agent_type", "unknown"),
    }

    results = {**state.results, title: result}
    next_idx = idx + 1
    is_last = next_idx >= len(state.subtasks)

    return {
        "results": results,
        "current_subtask": next_idx,
        "status": "completed" if is_last else "running",
    }


def human_feedback_node(state: TaskState) -> dict[str, Any]:
    """Handle human-in-the-loop interaction by simulating acceptance."""
    feedback = state.human_feedback or "accepted"
    return {"human_feedback": feedback, "status": "running"}


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

    graph.add_conditional_edges("human_feedback", router_node, {
        "worker": "worker",
        "human_feedback": "human_feedback",
        END: END,
    })

    return graph.compile()


__all__ = ["build_graph", "TaskState"]
