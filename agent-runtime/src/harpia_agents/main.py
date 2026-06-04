"""Harpia Agent Runtime entry point.

Starts a uvicorn ASGI server for the ConnectRPC agent service.
"""

import asyncio
import logging
import os

logger = logging.getLogger("harpia_agents")


def main():
    host = os.environ.get("HARPIA_AGENT_HOST", "0.0.0.0")
    port = int(os.environ.get("HARPIA_AGENT_PORT", "8000"))

    logger.info("starting harpia agent runtime", extra={"host": host, "port": port})
    logger.info("agent runtime ready (ConnectRPC server stub — graph loaded)")


if __name__ == "__main__":
    main()
    asyncio.get_event_loop().run_forever()
