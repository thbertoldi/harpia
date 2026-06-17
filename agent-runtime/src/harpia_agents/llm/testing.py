"""Test doubles for LLM provider and registry integration tests."""

from __future__ import annotations

from collections import deque
from collections.abc import AsyncIterator, Callable, Mapping, Sequence

from harpia_agents.llm.pricing import ModelPricing
from harpia_agents.llm.provider import (
    ChatMessage,
    CompletionChunk,
    CompletionResult,
    LLMProvider,
    TokenUsage,
)


class FakeLLMProvider(LLMProvider):
    name = "fake"

    def __init__(
        self,
        *,
        model_ids: Sequence[str],
        responses: Sequence[str] | None = None,
        transformer: Callable[[Sequence[ChatMessage]], str] | None = None,
    ) -> None:
        self.supported_models = frozenset(model_ids)
        self.pricing: Mapping[str, ModelPricing] = {
            model_id: ModelPricing(0.0, 0.0) for model_id in self.supported_models
        }
        self._responses = deque(responses or [])
        self._transformer = transformer

    def _next_content(self, messages: Sequence[ChatMessage]) -> str:
        if self._responses:
            return self._responses.popleft()
        if self._transformer is not None:
            return self._transformer(messages)
        return ""

    async def complete(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> CompletionResult:
        del max_tokens, temperature
        content = self._next_content(messages)
        usage = TokenUsage(
            input_tokens=self.count_tokens(model_id, messages),
            output_tokens=max(1, len(content) // 4),
            cost_usd=0.0,
        )
        return CompletionResult(
            content=content,
            model_id=model_id,
            usage=usage,
            provider=self.name,
            raw={"fake": True},
        )

    async def stream(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> AsyncIterator[CompletionChunk]:
        result = await self.complete(
            model_id=model_id,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )
        yield CompletionChunk(content_delta=result.content, usage=result.usage)

    def count_tokens(self, model_id: str, messages: Sequence[ChatMessage]) -> int:
        del model_id
        return sum(max(1, len(message.content) // 4) for message in messages)

    def estimate_cost(self, model_id: str, input_tokens: int, output_tokens: int) -> float:
        del model_id, input_tokens, output_tokens
        return 0.0
