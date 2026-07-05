"""Harpia Agent Runtime entry point.

Starts a ConnectRPC ASGI server or a Temporal worker depending on HARPIA_ROLE.
"""

import asyncio
import logging
import os
import sys

import uvicorn

from harpia_agents.identity import TenantResolverMiddleware
from harpia_agents.services import AgentServiceImpl

logger = logging.getLogger("harpia_agents")

# Single stdout handler so container runtimes (`kubectl logs`) capture every
# record. Without this the root logger has no handler and Python's lastResort
# handler silently drops INFO records to stderr.
_LOG_FORMAT = "%(asctime)s %(levelname)s %(name)s: %(message)s"


def _configure_logging() -> None:
    """Install a stdout handler on the root logger and pick up the level.

    Level is overridable via ``HARPIA_LOG_LEVEL`` (e.g. ``DEBUG``). Defaults to
    INFO so the worker startup banner and activity logs are visible. Must run
    before :func:`start_worker` / :func:`start_server` so the
    "Temporal worker started" line is captured.
    """
    level_name = os.environ.get("HARPIA_LOG_LEVEL", "INFO").upper()
    level = getattr(logging, level_name, logging.INFO)

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(logging.Formatter(_LOG_FORMAT))

    root = logging.getLogger()
    # Replace any pre-existing handlers so re-entry (e.g. tests, reloaders)
    # does not double-emit records.
    root.handlers[:] = [handler]
    root.setLevel(level)
    logging.getLogger("harpia_agents").setLevel(level)


def env_bool(name: str, default: bool = False) -> bool:
    value = os.environ.get(name)
    if value is None:
        return default
    return value.lower() in {"1", "t", "true", "y", "yes", "on"}


def create_app(*, allow_dev_auth: bool = False):
    """Create the ConnectRPC ASGI application with the AgentService implementation."""
    service = AgentServiceImpl()
    from harpia.agents.v1.agents_connect import (
        AgentServiceASGIApplication,
    )

    return TenantResolverMiddleware(
        AgentServiceASGIApplication(service),
        allow_dev_auth=allow_dev_auth,
    )


def start_server() -> None:
    """Start the ConnectRPC ASGI server."""
    host = os.environ.get("HARPIA_AGENT_HOST", "0.0.0.0")
    port = int(os.environ.get("HARPIA_AGENT_PORT", "8000"))

    logger.info("starting harpia agent runtime", extra={"host": host, "port": port})
    app = create_app(allow_dev_auth=env_bool("HARPIA_ALLOW_DEV_AUTH"))
    logger.info("agent runtime ready (ConnectRPC ASGI server)")
    uvicorn.run(app, host=host, port=port)


def start_worker() -> None:
    """Start the Temporal worker."""
    from harpia_agents.temporal_worker import run_worker

    asyncio.run(run_worker())


def main() -> None:
    """Entry point: dispatch based on HARPIA_ROLE or --worker flag."""
    _configure_logging()
    role = os.environ.get("HARPIA_ROLE", "server")

    if role == "worker":
        logger.info("starting in worker mode (Temporal)")
        start_worker()
    else:
        start_server()


if __name__ == "__main__":
    main()
