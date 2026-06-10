"""Central LangGraph state graph definition.

The graph implements a supervisor pattern:
    Planner (supervisor) -> Worker Agents (specialized subgraphs)

Flow:
    Task input -> Planner decomposes -> Routes to workers -> Aggregates -> Output
"""

from typing import Literal

from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph
from langgraph.graph.state import CompiledStateGraph
from pydantic import BaseModel, ConfigDict, Field

type TaskStatus = Literal[
    "pending", "planning", "running", "waiting_human", "completed", "failed"
]
type StateUpdate = dict[str, object]


class Subtask(BaseModel):
    """A single subtask in the task decomposition plan."""

    model_config = ConfigDict(extra="forbid")

    title: str
    description: str
    suggested_agent_type: str
    needs_human_input: bool = False


class SubtaskResult(BaseModel):
    """Typed result produced by a worker for a completed subtask."""

    model_config = ConfigDict(extra="forbid")

    subtask_title: str
    description: str
    agent_type: str
    output: str


class TaskPlan(BaseModel):
    """Structured output from the planner LLM."""

    model_config = ConfigDict(extra="forbid")

    subtasks: list[Subtask]


class TaskState(BaseModel):
    """Shared state across all nodes in the agent graph."""

    model_config = ConfigDict(extra="forbid")

    task_id: str = ""
    tenant_id: str = ""
    description: str = ""
    subtasks: list[Subtask] = Field(default_factory=list)
    current_subtask: int = 0
    results: dict[str, SubtaskResult] = Field(default_factory=dict)
    status: TaskStatus = "pending"
    human_feedback: str | None = None
    error: str | None = None


def planner_node(state: TaskState) -> StateUpdate:
    """Decompose a high-level task into subtasks using an LLM."""
    try:
        llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
        structured_llm = llm.with_structured_output(TaskPlan)
        plan = TaskPlan.model_validate(
            structured_llm.invoke(
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
        )
        return {"status": "running", "subtasks": plan.subtasks, "current_subtask": 0}
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
    if current.needs_human_input and state.human_feedback is None:
        return "human_feedback"
    return "worker"


def worker_node(state: TaskState) -> StateUpdate:
    """Execute the current subtask with a simulated agent invocation."""
    idx = state.current_subtask
    subtask = state.subtasks[idx]
    title = subtask.title or f"subtask_{idx}"

    result = SubtaskResult(
        subtask_title=title,
        description=subtask.description,
        agent_type=subtask.suggested_agent_type,
        output=(
            f"Successfully executed subtask: {title}. "
            f"Agent type used: {subtask.suggested_agent_type}."
        ),
    )

    results = {**state.results, title: result}
    next_idx = idx + 1
    is_last = next_idx >= len(state.subtasks)

    return {
        "results": results,
        "current_subtask": next_idx,
        "status": "completed" if is_last else "running",
    }


def human_feedback_node(state: TaskState) -> StateUpdate:
    """Handle human-in-the-loop interaction by simulating acceptance."""
    feedback = state.human_feedback or "accepted"
    return {"human_feedback": feedback, "status": "running"}


def build_graph() -> CompiledStateGraph[TaskState, None, TaskState, TaskState]:
    """Build and return the compiled LangGraph state graph."""
    graph: StateGraph[TaskState, None, TaskState, TaskState] = StateGraph(TaskState)

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


__all__ = ["Subtask", "SubtaskResult", "TaskPlan", "TaskState", "build_graph"]
