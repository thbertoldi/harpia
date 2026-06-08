"""Temporal activities and workflow wrapping LangGraph execution."""

from datetime import timedelta

from temporalio import activity, workflow

from harpia_agents.graph import TaskState, build_graph
from harpia_agents.identity import validate_temporal_input


@activity.defn
async def decompose_task_activity(input: dict) -> dict:
    """Decompose a high-level task into subtasks using LangGraph."""
    tenant_id = validate_temporal_input(input)
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
    validate_temporal_input(input)
    graph = build_graph()
    state = TaskState(**input)
    result = await graph.ainvoke(state.model_dump())
    return result


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
