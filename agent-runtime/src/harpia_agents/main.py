"""Harpia Agent Runtime entry point.

Starts a ConnectRPC ASGI server or a Temporal worker depending on HARPIA_ROLE.
"""

import asyncio
import logging
import os

import uvicorn

from harpia_agents.services import AgentServiceImpl

logger = logging.getLogger("harpia_agents")


def create_app():
    """Create the ConnectRPC ASGI application with the AgentService implementation."""
    service = AgentServiceImpl()
    from harpia_agents.gen.harpia.agents.v1.agents_connect import (
        AgentServiceASGIApplication,
    )

    return AgentServiceASGIApplication(service)


def start_server() -> None:
    """Start the ConnectRPC ASGI server."""
    host = os.environ.get("HARPIA_AGENT_HOST", "0.0.0.0")
    port = int(os.environ.get("HARPIA_AGENT_PORT", "8000"))

    logger.info("starting harpia agent runtime", extra={"host": host, "port": port})
    app = create_app()
    logger.info("agent runtime ready (ConnectRPC ASGI server)")
    uvicorn.run(app, host=host, port=port)


def start_worker() -> None:
    """Start the Temporal worker."""
    from harpia_agents.temporal_worker import run_worker

    asyncio.run(run_worker())


def main() -> None:
    """Entry point: dispatch based on HARPIA_ROLE or --worker flag."""
    role = os.environ.get("HARPIA_ROLE", "server")

    if role == "worker":
        logger.info("starting in worker mode (Temporal)")
        start_worker()
    else:
        start_server()


if __name__ == "__main__":
    main()
