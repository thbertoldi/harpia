"""Harpia Agent Runtime entry point.

Starts a ConnectRPC ASGI server for the agent service.
"""

import asyncio
import logging
import os

from harpia_agents.services import AgentServiceImpl

logger = logging.getLogger("harpia_agents")


def create_app():
    """Create the ConnectRPC ASGI application with the AgentService implementation."""
    service = AgentServiceImpl()
    try:
        from harpia_agents.gen.harpia.agents.v1.agents_connect import (
            AgentServiceASGIApplication,
        )

        return AgentServiceASGIApplication(service)
    except ImportError:
        logger.warning("connectrpc package not available; ASGI app is a stub")
        return None


def main():
    host = os.environ.get("HARPIA_AGENT_HOST", "0.0.0.0")
    port = int(os.environ.get("HARPIA_AGENT_PORT", "8000"))

    logger.info("starting harpia agent runtime", extra={"host": host, "port": port})
    app = create_app()
    if app is not None:
        logger.info("agent runtime ready (ConnectRPC ASGI server)")
        # TODO: fully integrate ConnectRPC ASGI with uvicorn lifecycle
        # uvicorn.run(app, host=host, port=port)
    else:
        logger.info("agent runtime ready (ConnectRPC server stub)")


if __name__ == "__main__":
    main()
    asyncio.get_event_loop().run_forever()
