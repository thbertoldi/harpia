"""Canonical LLM provider protocol and shared value objects."""

from __future__ import annotations

import time
from collections.abc import AsyncIterator, Callable, Mapping, Sequence
from dataclasses import dataclass
from typing import Any, Literal, Protocol, runtime_checkable

from harpia_agents.llm.pricing import ModelPricing


@dataclass(frozen=True)
class ChatMessage:
    role: Literal["system", "user", "assistant"]
    content: str


@dataclass(frozen=True)
class TokenUsage:
    input_tokens: int
    output_tokens: int
    cost_usd: float


@dataclass(frozen=True)
class CompletionChunk:
    content_delta: str
    usage: TokenUsage | None = None


@dataclass(frozen=True)
class CompletionResult:
    content: str
    model_id: str
    usage: TokenUsage
    provider: str
    raw: Any


def estimate_cost(
    pricing: Mapping[str, ModelPricing], model_id: str, *, input_tokens: int, output_tokens: int
) -> float:
    """Estimate USD cost for a request given model pricing."""
    model_pricing = pricing[model_id]
    return (input_tokens / 1_000_000) * model_pricing.input_per_million_usd + (
        output_tokens / 1_000_000
    ) * model_pricing.output_per_million_usd


@runtime_checkable
class LLMProvider(Protocol):
    name: str
    supported_models: frozenset[str]
    pricing: Mapping[str, ModelPricing]

    async def complete(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> CompletionResult: ...

    def stream(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> AsyncIterator[CompletionChunk]: ...

    def count_tokens(self, model_id: str, messages: Sequence[ChatMessage]) -> int: ...

    def estimate_cost(self, model_id: str, input_tokens: int, output_tokens: int) -> float: ...


async def with_heartbeat(
    stream: AsyncIterator[CompletionChunk],
    *,
    heartbeat: Callable[[], None],
    every: float = 5.0,
) -> AsyncIterator[CompletionChunk]:
    """Wrap a stream and emit periodic heartbeats between yielded chunks.

    Providers should yield chunks frequently enough so this wrapper can run
    heartbeat callbacks without violating Temporal activity heartbeat windows.
    """
    last_heartbeat = time.monotonic()
    async for chunk in stream:
        now = time.monotonic()
        if now - last_heartbeat >= every:
            heartbeat()
            last_heartbeat = now
        yield chunk
