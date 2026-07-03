"""Temporal worker runner for agent-runtime activities."""

import asyncio
import logging
import os

from temporalio.client import Client
from temporalio.worker import Worker
from temporalio.worker.workflow_sandbox import (
    SandboxedWorkflowRunner,
    SandboxRestrictions,
)

from harpia_agents.llm import LLMRegistry
from harpia_agents.temporal.worker import (
    configure_llm_registry,
    run_agent_activity,
)

logger = logging.getLogger("harpia_agents.temporal_worker")


async def run_worker() -> None:
    """Connect to Temporal server and run a worker for agent activities."""
    temporal_host = os.environ.get("TEMPORAL_HOST", "localhost:7233")
    task_queue = os.environ.get("TEMPORAL_TASK_QUEUE", "harpia-agent-task-queue")
    configure_llm_registry(LLMRegistry.default())

    client = await Client.connect(temporal_host)
    # The generated protobuf package (`harpia`) and the agent runtime
    # (`harpia_agents`) run their heavy, activity-only logic at import time
    # (e.g. harpia/__init__.py resolves a filesystem path, agents import LLM
    # SDKs). Those imports are deterministic for workflow purposes, so pass
    # them through the workflow sandbox instead of re-validating them — the
    # workflow body still executes sandboxed.
    runner = SandboxedWorkflowRunner(
        restrictions=SandboxRestrictions.default.with_passthrough_modules(
            "harpia",
            "harpia_agents",
        )
    )
    worker = Worker(
        client,
        task_queue=task_queue,
        activities=[run_agent_activity],
        workflow_runner=runner,
    )
    logger.info("Temporal worker started", extra={"host": temporal_host, "queue": task_queue})
    await worker.run()


if __name__ == "__main__":
    asyncio.run(run_worker())
