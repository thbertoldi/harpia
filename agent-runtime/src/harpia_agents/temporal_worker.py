"""Temporal worker runner — starts a worker that polls the harpia task queue."""

import asyncio
import logging
import os

from temporalio.client import Client
from temporalio.worker import Worker

from harpia_agents.llm import LLMRegistry
from harpia_agents.temporal.worker import (
    HarpiaTaskWorkflow,
    configure_llm_registry,
    decompose_task_activity,
    execute_subtask_activity,
    run_agent_activity,
)

logger = logging.getLogger("harpia_agents.temporal_worker")


async def run_worker() -> None:
    """Connect to Temporal server and run a worker for the harpia task queue."""
    temporal_host = os.environ.get("TEMPORAL_HOST", "localhost:7233")
    task_queue = os.environ.get("TEMPORAL_TASK_QUEUE", "harpia-task-queue")
    configure_llm_registry(LLMRegistry.default())

    client = await Client.connect(temporal_host)
    worker = Worker(
        client,
        task_queue=task_queue,
        workflows=[HarpiaTaskWorkflow],
        activities=[decompose_task_activity, execute_subtask_activity, run_agent_activity],
    )
    logger.info("Temporal worker started", extra={"host": temporal_host, "queue": task_queue})
    await worker.run()


if __name__ == "__main__":
    asyncio.run(run_worker())
