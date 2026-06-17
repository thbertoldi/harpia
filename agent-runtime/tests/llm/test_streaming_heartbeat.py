import asyncio
from collections.abc import AsyncIterator

import pytest

from harpia_agents.llm.provider import CompletionChunk, TokenUsage, with_heartbeat


async def _chunk_stream() -> AsyncIterator[CompletionChunk]:
    yield CompletionChunk(content_delta="a")
    await asyncio.sleep(0.02)
    yield CompletionChunk(content_delta="b")
    await asyncio.sleep(0.02)
    yield CompletionChunk(content_delta="c", usage=TokenUsage(1, 1, 0.0))


@pytest.mark.asyncio
async def test_with_heartbeat_forwards_chunks_and_calls_heartbeat() -> None:
    beats: list[str] = []

    def heartbeat() -> None:
        beats.append("beat")

    chunks = [
        chunk async for chunk in with_heartbeat(_chunk_stream(), heartbeat=heartbeat, every=0.01)
    ]

    assert [chunk.content_delta for chunk in chunks] == ["a", "b", "c"]
    assert beats
